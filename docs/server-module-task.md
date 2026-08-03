# 任务系统详解

本文档详细介绍 NetyAdmin 任务调度系统的架构设计、配置方式和二次开发指南。

---

## 一、模块概述

任务系统提供统一的任务调度引擎，支持启动期任务、定时任务、后台可控化管理，以及任务日志持久化。

### 1.1 核心特性

- **多触发类型**：once（启动执行）、interval（固定间隔）、cron（Cron表达式）
- **优先级队列**：支持多级优先级（Essential/Normal/Low），确保核心任务优先处理
- **弹性队列**：单机使用多级 Channel，集群使用 Redis 多级 List (BRPop 实现)
- **多机防重**：Redis 启用时，生产者侧（定时触发）通过分布式锁确保同一任务在同一时刻仅由一个实例执行
- **后台管理**：支持启停、重载、立即执行
- **日志持久化**：任务执行记录自动落库

---

## 二、目录结构

```
server/internal/pkg/task/
├── task.go             # 任务接口与优先级常量定义
├── manager.go          # 任务调度管理器
├── queue.go            # 多级优先级队列驱动实现
└── worker.go           # 消费者工作协程
```

---

## 三、架构设计

### 3.1 任务接口与分发体系

任务系统采用“生产者-消费者”模型。生产者负责按计划投递任务到队列，消费者（Worker）负责从队列中取出并执行。

#### 3.1.1 Dispatcher 接口 (生产者)
支持指定任务的优先级权重，权重越高，在队列中越早被消费。

```go
type Dispatcher interface {
    // Dispatch 投递一个子任务到队列
    // payload: 任务参数（自动序列化为 JSON）
    // weight: 优先级权重 (10-100)
    Dispatch(ctx context.Context, taskName string, payload interface{}, weight int) error
}

// 预定义优先级常量 (registry)
	const (
	    WeightEssential = 80  // 核心业务 (验证码发送、即时通知)
	    WeightNormal    = 50  // 普通业务 (文章发布、缓存同步)
	    WeightLow       = 10  // 低优先级 (日志清理、统计报表)
	)
```

#### 3.1.2 Task 接口 (消费者)
```go
type Task interface {
    Name() string                    // 任务标识
    DisplayName() string             // 显示名称
    Run(ctx context.Context) error   // 生产者逻辑 (可选，用于定时自触发)
    Execute(ctx context.Context, payload json.RawMessage) error // 消费者逻辑 (核心)
}
```

### 3.2 优先级队列驱动实现

为了实现核心任务“插队”处理，系统通过 **“物理隔离”** 方案实现了优先级队列：

#### 3.2.1 单机模式 (Local Queue)
- **结构**: 内部维护 `high` (Weight>=80), `normal` (Weight>=50), `low` 三个 Go Channel。
- **算法**: `Pop` 操作采用嵌套 `select` 轮询。优先检查 `high`，若为空则检查 `normal`，以此类推。
- **优点**: 纯内存操作，极低延迟。

#### 3.2.2 集群模式 (Redis Queue)
- **结构**: 对应三个 Redis List Key: `task:queue:high`, `normal`, `low`。
- **算法**: 调用 Redis 原生的 `BRPOP` 指令，参数顺序为 `[high, normal, low]`。Redis 保证按参数顺序检查 List，弹出第一个非空列表的元素。
- **优点**: 分布式环境下完美支持优先级抢占，零额外开销。

### 3.3 多机防重与分布式锁

在多实例部署场景下，定时任务（once/interval/cron）会在每个实例上独立触发，若不加控制会导致同一任务被多个实例重复执行。NetyAdmin 任务引擎通过 **Redis 分布式锁** 解决此问题。

#### 3.3.1 触发条件

分布式锁**仅在 Redis 启用时**自动生效（`redis.enabled = true` 且 Redis 客户端可用）。未启用 Redis 时，任务引擎退化为单机模式，不进行跨实例防重。

#### 3.3.2 锁机制实现

锁逻辑位于 `Manager.execute()` 方法中，覆盖所有由调度引擎触发的任务执行路径（once 同步执行、interval 周期触发、cron 定时触发、ManualRun 手动触发）。

| 维度 | 实现细节 |
|------|----------|
| **锁 Key** | 由 `cache.KeyTaskLock(prefix, taskName)` 生成，格式为 `{prefix}:task:lock:{taskName}` |
| **加锁方式** | `redis.SetArgs` + `NX` 模式（仅当 Key 不存在时设置成功） |
| **TTL 兜底** | 1 小时，防止实例宕机导致死锁 |
| **抢锁失败** | 返回 `redis.Nil`，本实例跳过本次执行并记录日志 |
| **锁释放** | `defer m.redis.Del(ctx, lockKey)`，任务执行完毕（无论成功或失败）后立即释放 |

```go
// 核心逻辑示意 (manager.go)
if m.redisCfg != nil && m.redisCfg.Enabled && m.redis != nil {
    lockKey := cache.KeyTaskLock(m.redisCfg.Prefix, name)
    err := m.redis.SetArgs(ctx, lockKey, "locked", redis.SetArgs{
        Mode: "NX",
        TTL:  1 * time.Hour,
    }).Err()
    if err == redis.Nil {
        // 未抢到锁，其他实例正在执行，本实例跳过
        return
    }
    defer func() { _ = m.redis.Del(ctx, lockKey) }()
}
```

#### 3.3.3 消费者侧为何不需要锁

队列消费者（`executePayload`）**不需要分布式锁**，因为：
- 集群模式下使用 Redis `BRPop` 弹出消息，该操作是**原子**的，同一条消息只会被一个 Worker 取到
- 单机模式下使用本地 Channel，天然不存在跨实例竞争

因此防重负担全部由生产者侧（定时触发入口）的分布式锁承担，消费者侧零额外开销。

#### 3.3.4 注意事项

1. **TTL 选择**：当前固定为 1 小时，适用于绝大多数业务任务。若任务执行时间可能超过 1 小时，需评估是否需要引入锁续期（Watchdog）机制。
2. **手动触发（ManualRun）**：同样会走加锁流程，多实例环境下手动触发也只会由一个实例实际执行。
3. **Redis 故障**：若 Redis 不可用导致加锁失败（非 `redis.Nil` 的其他错误），任务引擎出于安全考虑**不会执行**该次任务，避免重复执行风险。
4. **单机部署**：未启用 Redis 时无分布式锁，但单机本身不存在多实例问题，无需防重。

---

## 四、配置说明

### 4.1 配置文件（config.toml）

```yaml
task:
  # 任务系统总开关
  enabled: true

  jobs:
    article_publish:
      enabled: true
      type: interval
      spec: "1m"
      weight: 50 # 优先级权重

    system_log_cleanup:
      enabled: true
      type: cron
      spec: "0 0 3 * * *"
      weight: 10 # 低优先级
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

## 六、开发示例 (优先级任务)

### 6.1 异步发送短信 (核心任务)

```go
// 业务代码中投递
// 指定 WeightEssential (80)，即使此时有海量日志清理任务在排队，短信也会被优先发送
err := s.dispatcher.Dispatch(ctx, "msg_send_job", smsPayload, task.WeightEssential)
```

### 6.2 任务定义

```go
func (t *MsgSendJob) Execute(ctx context.Context, payload json.RawMessage) error {
    var req SmsRequest
    json.Unmarshal(payload, &req)
    // 调用物理驱动执行发送...
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

## 九、内置任务清单

NetyAdmin 默认注册以下任务（在 `internal/job/init.go` 中通过 `AllJobs` 注册）：

| 任务名 | 类型 | 默认规则 | 权重 | 说明 |
|--------|------|----------|------|------|
| `article_publish` | interval | `1m` | Normal (50) | 文章定时发布扫描 |
| `system_log_cleanup` | cron | `0 0 2 * * *` | Low (10) | 系统日志（任务/操作/错误/消息/开放平台）按保留天数清理 |
| `upload_record_cleanup` | cron | `0 */30 * * * *` | Low (10) | 将超期未通知的 pending 上传记录标记为 expired |
| `token_hash_cleanup` | cron | `0 0 * * * *` | Low (10) | 物理删除 `user_token_hashes` 表中 `expired_at < NOW()` 的过期 token hash 记录 |

### 9.1 Token Hash 清理任务（`token_hash_cleanup`）

#### 背景

用户登录、刷新令牌、登出黑名单等流程会在 `user_token_hashes` 表写入 token hash 行（含 `expired_at` 过期时间）。即便 token 已过期失效，这些行仍会留在表中，长期运行后表会无限堆积，影响查询性能与存储空间。

`0019_user_token_hashes.up.sql` 迁移已为 `expired_at` 列创建 `idx_user_token_expired` 索引，专门用于支撑过期清理的高效范围删除。

#### 实现细节

- **Repository 方法**：`UserRepository.DeleteExpiredTokenHashes(ctx) (int64, error)`
  - SQL 语义：`DELETE FROM user_token_hashes WHERE expired_at < NOW()`
  - `user_token_hashes` 表无 `soft_delete` 字段，`Delete` 即硬删除
  - 通过 `r.getDB(ctx)` 复用 ctx 中携带的事务句柄（与项目其他 repo 方法一致）
- **Job 实现**：`internal/job/token_hash_cleanup.go` 的 `TokenHashCleanupJob`
  - Cron 表达式 `0 0 * * * *`（每小时整点执行一次，6 字段——任务引擎用 `cron.WithSeconds()` 初始化）
  - 调用 repo 方法后，若 `affected > 0` 用 `slog.Info` 记录清理行数
  - 失败时返回 error，由任务引擎的 `onFinish` 回调写入 `sys_task_logs` 表
- **DI 注册**：`wire.go` 中 `job.AllJobs(...)` 调用追加 `repos.user` 参数；`init.go` 的 `AllJobs` 签名已同步增加 `userRepository userRepo.UserRepository` 形参

#### 配置覆盖

可通过 `config.toml` 的 `[task.jobs.token_hash_cleanup]` 覆盖默认配置：

```yaml
task:
  jobs:
    token_hash_cleanup:
      enabled: true
      type: cron
      spec: "0 0 * * * *"   # 每小时整点
      weight: 10
```

#### 注意事项

1. **幂等性**：删除操作天然幂等，重复执行不会产生副作用。
2. **多机部署**：Redis 启用时，任务引擎通过 `cache.KeyTaskLock(prefix, "token_hash_cleanup")` 分布式锁确保多实例环境下同一时刻仅一个实例执行。
3. **不在事务内**：单步删除操作，无需通过 `TransactionManager` 包裹。
4. **与 Logout/RefreshToken 流程的关系**：用户主动 Logout 时会主动删除当前 access/refresh token hash（见 `service/user/user_auth.go`），本任务是兜底清理「用户未主动登出且 token 已过期」的残留行。

---

## 十、最佳实践

1. **任务幂等性**：确保任务可以安全地重复执行
2. **错误处理**：记录详细错误信息，但避免任务无限重试
3. **资源限制**：重型任务使用队列异步处理
4. **监控告警**：对失败率高的任务设置告警
5. **日志保留**：定期清理历史任务日志

---

## 十一、相关文档

- [Server架构设计](./server-architecture.md)
- [配置热同步](./server-module-config.md)
