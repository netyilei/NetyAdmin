# 日志模块详解

本文档详细介绍 NetyAdmin 日志模块的架构设计、使用方式和二次开发指南。

---

## 一、模块概述

日志模块提供操作日志和错误日志的完整管理能力，支持自动记录、查询、清理，以及敏感信息脱敏。

### 1.1 核心特性

- **操作日志**：自动记录所有管理操作
- **错误日志**：捕获并记录系统异常
- **敏感脱敏**：自动脱敏密码、Token等敏感字段
- **批量清理**：支持按保留策略自动清理
- **状态追踪**：错误日志支持标记解决状态

---

## 二、目录结构

```
server/internal/domain/entity/log/
├── operation.go        # 操作日志实体
└── error.go            # 错误日志实体

server/internal/repository/log/
├── operation.go        # 操作日志仓储
└── error.go            # 错误日志仓储

server/internal/service/log/
├── operation.go        # 操作日志服务
└── error.go            # 错误日志服务

server/internal/middleware/
├── operation_log.go    # 操作日志中间件
└── recovery.go         # 异常恢复中间件

server/internal/interface/admin/http/handler/v1/
├── operation_log/      # 操作日志Handler
└── error_log/          # 错误日志Handler
```

---

## 三、数据模型

### 3.1 操作日志（operation_logs）

```go
type OperationLog struct {
    ID           uint           `gorm:"primarykey"`
    AdminID      uint           `gorm:"index"`                         // 操作者ID
    AdminName    string         `gorm:"size:64"`                       // 操作者名称
    Module       string         `gorm:"size:64"`                       // 功能模块
    Action       string         `gorm:"size:64"`                       // 操作动作
    Method       string         `gorm:"size:16"`                       // 请求方法
    Path         string         `gorm:"size:512"`                      // 请求路径
    IP           string         `gorm:"size:64"`                       // 客户端IP
    UserAgent    string         `gorm:"size:512"`                      // 用户代理
    RequestBody  string         `gorm:"type:text"`                     // 请求体（脱敏后）
    ResponseBody string         `gorm:"type:text"`                     // 响应体
    Status       int            `gorm:"index"`                         // HTTP状态码
    Duration     int            `gorm:"comment:'请求耗时(ms)'"`          // 耗时
    CreatedAt    int64          `gorm:"autoCreateTime;index"`
}
```

### 3.2 错误日志（error_logs）

```go
type ErrorLog struct {
    ID          uint           `gorm:"primarykey"`
    Module      string         `gorm:"size:64;index"`                 // 错误模块
    Level       string         `gorm:"size:16;index"`                 // 错误级别：error/warn
    Message     string         `gorm:"type:text"`                     // 错误消息
    Detail      string         `gorm:"type:text"`                     // 详细堆栈
    RequestID   string         `gorm:"size:64;index"`                 // 请求ID
    URL         string         `gorm:"size:512"`                      // 请求URL
    Method      string         `gorm:"size:16"`                       // 请求方法
    IP          string         `gorm:"size:64"`                       // 客户端IP
    UserAgent   string         `gorm:"size:512"`                      // 用户代理
    IsResolved  bool           `gorm:"default:false;index"`           // 是否已解决
    ResolvedAt  *int64         `gorm:"index"`                         // 解决时间
    ResolvedBy  uint           `gorm:"index"`                         // 解决人
    CreatedAt   int64          `gorm:"autoCreateTime;index"`
}
```

---

## 四、自动记录机制

### 4.1 操作日志中间件

```go
// internal/middleware/operation_log.go

func OperationLogMiddleware(logService log.OperationLogService) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        // 读取请求体
        var requestBody []byte
        if c.Request.Body != nil {
            requestBody, _ = io.ReadAll(c.Request.Body)
            c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
        }
        
        // 包装响应Writer
        writer := &responseWriter{ResponseWriter: c.Writer}
        c.Writer = writer
        
        c.Next()
        
        // 记录日志
        duration := time.Since(start).Milliseconds()
        
        log := &entity.OperationLog{
            AdminID:      getAdminID(c),
            AdminName:    getAdminName(c),
            Module:       parseModule(c.Request.URL.Path),
            Action:       parseAction(c.Request.Method),
            Method:       c.Request.Method,
            Path:         c.Request.URL.Path,
            IP:           c.ClientIP(),
            UserAgent:    c.Request.UserAgent(),
            RequestBody:  maskSensitive(string(requestBody)),
            ResponseBody: writer.body.String(),
            Status:       c.Writer.Status(),
            Duration:     int(duration),
        }
        
        logService.Create(c.Request.Context(), log)
    }
}
```

### 4.2 敏感字段脱敏

```go
// 敏感字段列表
var sensitiveFields = []string{
    "password",
    "old_password",
    "new_password",
    "token",
    "access_token",
    "refresh_token",
    "secret_key",
}

// 脱敏处理
func maskSensitive(jsonStr string) string {
    var data map[string]interface{}
    if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
        return jsonStr
    }
    
    for _, field := range sensitiveFields {
        if _, exists := data[field]; exists {
            data[field] = "***"
        }
    }
    
    result, _ := json.Marshal(data)
    return string(result)
}
```

### 4.3 错误捕获

```go
// internal/middleware/recovery.go

func RecoveryMiddleware(errorLogService log.ErrorLogService) gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if err := recover(); err != nil {
                // 记录错误日志
                errorLog := &entity.ErrorLog{
                    Module:    "system",
                    Level:     "error",
                    Message:   fmt.Sprintf("%v", err),
                    Detail:    string(debug.Stack()),
                    RequestID: c.GetString("request_id"),
                    URL:       c.Request.URL.String(),
                    Method:    c.Request.Method,
                    IP:        c.ClientIP(),
                    UserAgent: c.Request.UserAgent(),
                }
                errorLogService.Create(c.Request.Context(), errorLog)
                
                // 返回错误响应
                response.Error(c, errorx.CodeInternalError)
                c.Abort()
            }
        }()
        
        c.Next()
    }
}
```

---

## 五、API接口

### 5.1 操作日志

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/operation-logs | 操作日志列表 |
| DELETE | /admin/v1/operation-logs/:id | 删除日志 |
| POST | /admin/v1/operation-logs/batch-delete | 批量删除 |

### 5.2 错误日志

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/error-logs | 错误日志列表 |
| PUT | /admin/v1/error-logs/:id/resolve | 标记已解决 |
| DELETE | /admin/v1/error-logs/:id | 删除日志 |
| POST | /admin/v1/error-logs/batch-delete | 批量删除 |

---

## 六、日志清理

### 6.1 清理任务

```go
// internal/job/system_log_cleanup.go

func (t *SystemLogCleanupTask) Run(ctx context.Context) error {
    // 读取保留天数
    opsRetention := t.getRetentionDays("ops_config")
    errorRetention := t.getRetentionDays("error_config")
    
    cutoffOps := time.Now().AddDate(0, 0, -opsRetention).Unix()
    cutoffError := time.Now().AddDate(0, 0, -errorRetention).Unix()
    
    // 清理操作日志
    t.operationLogRepo.DeleteBefore(ctx, cutoffOps)
    
    // 清理错误日志
    t.errorLogRepo.DeleteBefore(ctx, cutoffError)
    
    return nil
}
```

### 6.2 配置保留天数

```sql
-- 在sys_configs中配置
INSERT INTO sys_configs (`group`, key, value) VALUES
('ops_config', 'retention_days', '30'),      -- 操作日志保留30天
('error_config', 'retention_days', '90');    -- 错误日志保留90天
```

---

## 七、二次开发示例

### 7.1 自定义日志字段

```go
// 1. 扩展实体
// internal/domain/entity/log/operation.go

type OperationLog struct {
    // ... 现有字段
    
    OrganizationID uint   `gorm:"index"`    // 组织ID（多租户场景）
    ExtraData      string `gorm:"type:text"` // 扩展数据（JSON）
}

// 2. 修改中间件
func OperationLogMiddleware(...) gin.HandlerFunc {
    return func(c *gin.Context) {
        // ...
        
        log := &entity.OperationLog{
            // ... 现有字段
            OrganizationID: getOrgID(c),
            ExtraData:      getExtraData(c),
        }
        
        logService.Create(c.Request.Context(), log)
    }
}
```

### 7.2 添加审计字段

```go
// 记录数据变更前后的值
type OperationLog struct {
    // ... 现有字段
    
    BeforeData string `gorm:"type:text"` // 变更前数据
    AfterData  string `gorm:"type:text"` // 变更后数据
}

// 在Service层记录
func (s *articleService) Update(ctx context.Context, id uint, req *dto.UpdateArticleReq) error {
    // 1. 获取原数据
    oldArticle, _ := s.repo.GetByID(ctx, id)
    
    // 2. 更新数据
    // ...
    
    // 3. 记录审计日志
    s.logService.RecordChange(ctx, log.ChangeRecord{
        Module:     "content",
        Action:     "update_article",
        ObjectID:   id,
        ObjectType: "article",
        Before:     oldArticle,
        After:      newArticle,
    })
    
    return nil
}
```

### 7.3 错误告警通知

```go
// internal/service/log/error.go

func (s *errorService) Create(ctx context.Context, log *entity.ErrorLog) error {
    // 1. 保存日志
    if err := s.repo.Create(ctx, log); err != nil {
        return err
    }
    
    // 2. 严重错误发送告警
    if log.Level == "error" && strings.Contains(log.Message, "critical") {
        s.alertService.SendAlert(ctx, alert.AlertMessage{
            Title:   "系统严重错误",
            Content: log.Message,
            Level:   alert.LevelCritical,
        })
    }
    
    return nil
}
```

---

## 八、查询优化

### 8.1 索引建议

```sql
-- 操作日志索引
CREATE INDEX idx_operation_logs_created_at ON operation_logs(created_at);
CREATE INDEX idx_operation_logs_admin_id ON operation_logs(admin_id);
CREATE INDEX idx_operation_logs_module ON operation_logs(module);

-- 错误日志索引
CREATE INDEX idx_error_logs_created_at ON error_logs(created_at);
CREATE INDEX idx_error_logs_is_resolved ON error_logs(is_resolved);
CREATE INDEX idx_error_logs_level ON error_logs(level);
```

### 8.2 分区表（大数据量）

```sql
-- 按月份分区
CREATE TABLE operation_logs (
    -- ... 字段定义
) PARTITION BY RANGE (created_at);

-- 创建分区
CREATE TABLE operation_logs_2024_01 PARTITION OF operation_logs
    FOR VALUES FROM (1704067200) TO (1706745600);
```

---

## 九、最佳实践

1. **异步记录**：日志写入使用异步方式，避免阻塞请求
2. **采样记录**：高频接口可配置采样率，减少日志量
3. **分级存储**：热数据存SSD，冷数据归档到对象存储
4. **定期归档**：历史日志定期导出到对象存储后删除
5. **敏感保护**：严格脱敏所有敏感信息

---

## 十、相关文档

- [Server架构设计](./server-architecture.md)
- [任务系统详解](./server-module-task.md)
