# 任务系统详解

本文档详细介绍 NetyAdmin 任务调度系统的架构设计、配置方式和二次开发指南。

---

## 一、模块概述

任务系统提供统一的任务调度引擎，支持启动期任务、定时任务、后台可控化管理，以及任务日志持久化。

### 1.1 核心特性

- **多触发类型**：once（启动执行）、interval（固定间隔）、cron（Cron表达式）
- **队列支持**：生产者-消费者模型，支持异步处理
- **弹性队列**：单机使用Channel，集群使用Redis
- **后台管理**：支持启停、重载、立即执行
- **日志持久化**：任务执行记录自动落库

---

## 二、目录结构

```
server/internal/pkg/task/
├── task.go             # 任务接口定义
├── manager.go          # 任务管理器
└── queue.go            # 队列实现

server/internal/job/
├── init.go             # 任务注册入口
├── article_publish.go  # 文章定时发布任务
└── system_log_cleanup.go # 日志清理任务
```

---

## 三、架构设计

### 3.1 任务接口

```go
// Task 任务接口
type Task interface {
    Name() string                    // 任务标识（英文）
    DisplayName() string             // 显示名称（中文）
    Run(ctx context.Context) error   // 定时触发入口
    Execute(ctx context.Context, payload []byte) error // 队列消费入口
}

// TaskWithMetadata 带元数据的任务
type TaskWithMetadata interface {
    Task
    DefaultMetadata() TaskMetadata   // 默认配置
}

// TaskMetadata 任务元数据
type TaskMetadata struct {
    Type   string // once/interval/cron
    Spec   string // 定时规则（interval: 30s, cron: 0 0 * * *）
    Weight int    // 启动执行顺序（越小越先）
}
```

### 3.2 队列驱动

```
单机模式（Redis未启用）:
    Local Queue (Go Channel)
    └── 当前进程内完成生产消费

集群模式（Redis已启用）:
    Redis Queue (List LPUSH/BRPOP)
    └── 支持多机Worker共同分担
```

### 3.3 配置覆盖层级

```
第一层：代码默认值（DefaultMetadata）
    ↓ 被覆盖
第二层：config.toml（task.jobs.{name}）
    ↓ 被覆盖
第三层：sys_configs（task_config分组）
    ↓ 运行时生效
```

---

## 四、配置说明

### 4.1 配置文件（config.toml）

```toml
[task]
# 任务系统总开关
enabled = true

[task.jobs.article_publish]
enabled = true
type = "interval"
spec = "1m"          # 每分钟检查一次
weight = 10

[task.jobs.system_log_cleanup]
enabled = true
type = "cron"
spec = "0 0 3 * * *" # 每天凌晨3点执行
weight = 20
```

### 4.2 动态配置（sys_configs）

| 配置项 | Group | Key | 说明 |
|--------|-------|-----|------|
| 任务总开关 | task_config | enabled | true/false |
| 单任务开关 | task_config | task:{name}:enabled | true/false |
| 单任务规则 | task_config | task:{name}:spec | 定时规则 |
| 日志开关 | task_config | log_enabled | true/false |
| 保留天数 | task_config | retention_days | 日志保留天数 |

---

## 五、API接口

### 5.1 任务管理

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/system/tasks | 任务列表 |
| POST | /admin/v1/system/tasks/:name/run | 立即执行 |
| POST | /admin/v1/system/tasks/:name/start | 启动任务 |
| POST | /admin/v1/system/tasks/:name/stop | 停止任务 |
| POST | /admin/v1/system/tasks/:name/reload | 重载任务 |
| PUT | /admin/v1/system/tasks/:name | 更新配置 |
| GET | /admin/v1/system/tasks/logs | 执行日志 |

---

## 六、内置任务

### 6.1 文章定时发布

```go
// internal/job/article_publish.go

type ArticlePublishTask struct {
    articleRepo content.ArticleRepository
}

func (t *ArticlePublishTask) Name() string {
    return "article_publish"
}

func (t *ArticlePublishTask) DisplayName() string {
    return "文章定时发布"
}

func (t *ArticlePublishTask) DefaultMetadata() task.TaskMetadata {
    return task.TaskMetadata{
        Type:   "interval",
        Spec:   "1m",
        Weight: 10,
    }
}

func (t *ArticlePublishTask) Run(ctx context.Context) error {
    // 查询待发布的文章
    articles, err := t.articleRepo.GetPendingPublish(ctx)
    if err != nil {
        return err
    }
    
    // 发布到期的文章
    now := time.Now()
    for _, article := range articles {
        if article.PublishTime <= now.Unix() {
            if err := t.articleRepo.Publish(ctx, article.ID); err != nil {
                log.Printf("发布文章 %d 失败: %v", article.ID, err)
            }
        }
    }
    
    return nil
}
```

### 6.2 日志清理

```go
// internal/job/system_log_cleanup.go

type SystemLogCleanupTask struct {
    operationLogRepo log.OperationLogRepository
    errorLogRepo     log.ErrorLogRepository
    taskLogRepo      system.TaskLogRepository
    configWatcher    *configsync.Watcher
}

func (t *SystemLogCleanupTask) Run(ctx context.Context) error {
    // 读取保留天数配置
    retentionDays := t.configWatcher.GetConfig("task_config", "retention_days")
    days, _ := strconv.Atoi(retentionDays)
    if days <= 0 {
        days = 30 // 默认30天
    }
    
    cutoffTime := time.Now().AddDate(0, 0, -days)
    
    // 清理各类日志
    t.operationLogRepo.DeleteBefore(ctx, cutoffTime)
    t.errorLogRepo.DeleteBefore(ctx, cutoffTime)
    t.taskLogRepo.DeleteBefore(ctx, cutoffTime)
    
    return nil
}
```

---

## 七、二次开发示例

### 7.1 创建新任务

```go
// internal/job/data_sync.go

package job

import (
    "context"
    "encoding/json"
    "server/internal/pkg/task"
)

// DataSyncTask 数据同步任务
type DataSyncTask struct {
    // 依赖的仓储或服务
}

// SyncPayload 队列消息载荷
type SyncPayload struct {
    TableName string `json:"table_name"`
    RecordID  uint   `json:"record_id"`
}

func NewDataSyncTask() *DataSyncTask {
    return &DataSyncTask{}
}

func (t *DataSyncTask) Name() string {
    return "data_sync"
}

func (t *DataSyncTask) DisplayName() string {
    return "数据同步"
}

func (t *DataSyncTask) DefaultMetadata() task.TaskMetadata {
    return task.TaskMetadata{
        Type:   "interval",
        Spec:   "5m",     // 每5分钟执行一次
        Weight: 30,
    }
}

// Run 定时触发入口
func (t *DataSyncTask) Run(ctx context.Context) error {
    // 查询需要同步的数据
    // ...
    
    // 投递到队列异步处理
    for _, record := range records {
        payload := SyncPayload{
            TableName: "users",
            RecordID:  record.ID,
        }
        data, _ := json.Marshal(payload)
        
        if err := task.GetDispatcher().Dispatch(ctx, t.Name(), data); err != nil {
            return err
        }
    }
    
    return nil
}

// Execute 队列消费入口
func (t *DataSyncTask) Execute(ctx context.Context, payload []byte) error {
    var syncPayload SyncPayload
    if err := json.Unmarshal(payload, &syncPayload); err != nil {
        return err
    }
    
    // 执行具体的同步逻辑
    // ...
    
    return nil
}
```

### 7.2 注册任务

```go
// internal/job/init.go

package job

import "server/internal/pkg/task"

// RegisterTasks 注册所有任务
func RegisterTasks(manager *task.Manager) {
    // 内置任务
    manager.Register(NewArticlePublishTask())
    manager.Register(NewSystemLogCleanupTask())
    
    // 新增任务
    manager.Register(NewDataSyncTask())
}
```

### 7.3 在Wire中注入

```go
// internal/app/wire.go

func ProvideTasks(
    articlePublish *job.ArticlePublishTask,
    logCleanup *job.SystemLogCleanupTask,
    dataSync *job.DataSyncTask,  // 新增
) []task.Task {
    return []task.Task{
        articlePublish,
        logCleanup,
        dataSync,
    }
}
```

---

## 八、任务日志

### 8.1 日志表结构

```sql
CREATE TABLE sys_task_logs (
    id SERIAL PRIMARY KEY,
    task_name VARCHAR(64) NOT NULL,
    task_display_name VARCHAR(128),
    status SMALLINT NOT NULL DEFAULT 0, -- 0:运行中 1:成功 2:失败
    result TEXT,
    error_msg TEXT,
    start_time BIGINT NOT NULL,
    end_time BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 8.2 自定义日志内容

```go
func (t *DataSyncTask) Run(ctx context.Context) error {
    // 设置任务上下文
    ctx = task.WithTaskID(ctx, task.GenerateTaskID())
    
    // 任务开始
    task.Log(ctx, "开始数据同步")
    
    // 执行任务
    count, err := t.doSync(ctx)
    if err != nil {
        task.LogError(ctx, "同步失败: "+err.Error())
        return err
    }
    
    // 任务完成
    task.Log(ctx, fmt.Sprintf("同步完成，共处理 %d 条记录", count))
    
    return nil
}
```

---

## 九、最佳实践

1. **任务幂等性**：确保任务可以安全地重复执行
2. **错误处理**：记录详细错误信息，但避免任务无限重试
3. **资源限制**：重型任务使用队列异步处理
4. **监控告警**：对失败率高的任务设置告警
5. **日志保留**：定期清理历史任务日志

---

## 十、相关文档

- [Server架构设计](./server-architecture.md)
- [配置热同步](./server-module-config.md)
