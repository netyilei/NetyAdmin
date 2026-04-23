# 统一消息发送模块详解

本文档详细介绍 NetyAdmin 统一消息发送模块（Message Hub）的架构设计、驱动扩展及二次开发指南。

---

## 一、模块概述

统一消息发送模块旨在为系统提供单一的消息下发入口，支持短信（SMS）、电子邮件（Email）、站内信（Internal）及推送（Push）多种通道。

### 1.1 核心特性

- **驱动化架构**：通道插件化，支持快速接入腾讯云、阿里云等不同服务商。
- **模板渲染**：支持带有变量（如 `{{code}}`）的消息模板，在发送前自动渲染。
- **异步处理**：集成系统任务引擎，支持消息后台异步发送与失败自动重试。
- **优先级队列**：支持多级优先级（如验证码优先于营销消息）。
- **分布式同步**：利用缓存 Tags 机制，实现模板变更全网秒级同步。
- **多认证方式**：Email 驱动支持 PLAIN/LOGIN/CRAM-MD5/AUTO 四种 SMTP 认证模式。
- **STARTTLS 支持**：支持端口 587 的 STARTTLS 加密连接。
- **灵活加密**：同时支持 SSL/TLS（端口 465）和 STARTTLS（端口 587）两种加密方式。

---

## 二、目录结构

```
server/internal/domain/entity/message/
├── message.go          # 模板、记录、站内信实体

server/internal/repository/message/
├── message.go          # 消息仓储实现

server/internal/service/message/
├── message.go          # 消息发送逻辑与模板管理
└── message_job.go      # 异步发送任务处理器

server/internal/pkg/message/
├── driver.go           # 驱动接口定义
├── email_driver.go     # SMTP 驱动实现（基于 go-simple-mail）
└── mock_driver.go      # 单元测试 Mock 驱动
```

---

## 三、数据模型

### 3.1 消息模板 (`msg_templates`)

```go
type MsgTemplate struct {
    ID            uint64         `gorm:"primaryKey"`
    Code          string         `gorm:"size:50;uniqueIndex"` // 业务调用编码
    Name          string         `gorm:"size:100"`
    Channel       string         `gorm:"size:20"`             // sms, email, internal, push
    Title         string         `gorm:"size:200"`            // 邮件/站内信标题
    Content       string         `gorm:"type:text"`           // 模板内容 (支持 {{var}})
    ProviderTplID string         `gorm:"size:100"`            // 第三方侧 ID
    Status        int            `gorm:"default:1"`           // 1:启用, 0:禁用
}
```

### 3.2 消息记录 (`msg_records`)

```go
type MsgRecord struct {
    ID         uint64    `gorm:"primaryKey"`
    UserID     string    `gorm:"size:26;index"`  // 关联用户ID（可选）
    Channel    string    `gorm:"size:20"`          // sms, email, internal, push
    Receiver   string    `gorm:"size:100"`         // 手机号/邮箱/Token
    Title      string    `gorm:"size:200"`         // 消息标题
    Content    string    `gorm:"type:text"`        // 消息正文
    Status     int       `gorm:"default:0"`       // 0:等待, 1:成功, 2:失败
    ErrorMsg   string    `gorm:"type:text"`        // 失败错误信息
    NodeID     string    `gorm:"size:50"`          // 发送节点ID
    Priority   int       `gorm:"default:2"`        // 1:高, 2:中, 3:低
    RetryCount int       `gorm:"default:0"`        // 重试次数
    CreatedAt  time.Time `json:"createdAt"`
    UpdatedAt  time.Time `json:"updatedAt"`
}
```

### 3.3 站内信扩展 (`msg_internal`)

```go
type MsgInternal struct {
    ID          uint64 `gorm:"primaryKey"`
    MsgRecordID uint64 `gorm:"not null;index"` // 关联消息记录
    Type        int    `gorm:"default:1"`      // 1:系统公告, 2:私信
}
```

---

## 四、核心逻辑

### 4.1 发送流程

1. **业务调用**：`MessageService.SendTemplate(ctx, "VERIFY_CODE", "user@example.com", params)`
2. **模板加载**：从缓存/数据库获取模板内容（模板数据走 `LazyCacheManager` 缓存，Tag 失效）
3. **内容渲染**：将 `{{code}}` 替换为具体数值
4. **记录入库**：创建 `msg_records`，初始状态为 `0`（等待发送）
5. **投递任务**：调用 `Dispatcher.Dispatch` 投递 `msg_send_job` 任务
6. **异步执行**：`MsgSendJob` 消费者调用对应通道驱动，物理下发消息
7. **状态更新**：驱动返回后更新 `msg_records.status`（成功=1，失败=2，错误信息写入 `error_msg`）

### 4.2 站内信特殊处理

站内信（`channel=internal`）不依赖外部驱动，由系统直接处理：

1. **消息记录**：创建 `msg_records` 记录，状态直接设为成功
2. **扩展记录**：同时创建 `msg_internal` 扩展记录，关联 `msg_record_id`
3. **类型区分**：
   - `type=1`：系统公告（`receiver="all"` 时自动设置）
   - `type=2`：私信（指定用户时）

### 4.3 Email/SMS 配置读取策略

Email 和 SMS 配置通过 `sys_configs` 表管理，**读取时优先走缓存模块（configWatcher），实现热更**：

```
emailDriver.resolveConfig(ctx)
  → watcherConfigProvider.GetByGroup(ctx, "email_config")
    → configWatcher.GetByGroup("email_config")   // 优先从内存缓存读取
      → 若缓存未命中 → 回源数据库 → 写入缓存 → 返回
```

**设计要点**：

- 使用 `watcherConfigProvider` 包装 `configWatcher`，使其实现 `ConfigProvider` 接口
- `configWatcher` 在启动时加载全量配置到内存，后续通过 PubSubBus 监听配置变更事件
- Admin 端修改配置后，`configWatcher` 自动失效旧缓存并加载新值，无需重启服务
- 发送前检查 `email_config.enabled` / `sms_config.enabled`，未启用的通道直接跳过
- 数据库仍是唯一持久化源，缓存仅加速读取

### 4.4 通道启用检查

`MsgSendJob` 在执行发送前，会先检查目标通道是否启用：

```go
func (j *MsgSendJob) Execute(ctx context.Context, payload string) error {
    // ...
    if !j.isChannelEnabled(ctx, channel) {
        return nil // 通道未启用，静默跳过
    }
    // 调用驱动发送...
}
```

- `email` 通道：读取 `email_config.enabled`
- `sms` 通道：读取 `sms_config.enabled`
- `internal` 通道：始终启用

### 4.5 驱动接口

```go
// Driver 消息发送驱动接口
type Driver interface {
    // Send 发送消息
    // ctx: 上下文，包含超时控制
    // receiver: 接收人标识 (手机号/邮箱/Token)
    // title: 消息标题 (仅部分通道支持)
    // content: 消息正文
    // params: 扩展参数 (供驱动层使用，如模板ID等)
    Send(ctx context.Context, receiver string, title string, content string, params map[string]string) error
}

// SmsDriver 短信专用驱动接口
type SmsDriver interface {
    Driver
    // SendWithTemplate 使用模板发送短信
    SendWithTemplate(ctx context.Context, phone string, templateID string, params map[string]string) error
}
```

---

## 五、Email 驱动详解

### 5.1 依赖

Email 驱动基于开源库 [go-simple-mail](https://github.com/xhit/go-simple-mail) 实现，支持 SSL/TLS、STARTTLS、连接池、超时控制等企业级功能。

```
go-simple-mail/v2 v2.3.0+
```

### 5.2 配置项

Email 配置通过 `sys_configs` 表 `email_config` 分组管理，支持的配置项如下：

| 配置键 | 类型 | 说明 | 示例值 |
|--------|------|------|--------|
| `enabled` | boolean | 是否启用邮件服务 | `true` |
| `host` | string | SMTP 服务器地址 | `smtp.example.com` |
| `port` | number | SMTP 端口 | `465` / `587` |
| `user` | string | 发件人账号 | `noreply@example.com` |
| `password` | string | 发件人密码或授权码 | `xxxx` |
| `from` | string | 发件人地址 | `noreply@example.com` |
| `ssl_enabled` | boolean | 启用 SSL/TLS（端口465） | `true` |
| `starttls_enabled` | boolean | 启用 STARTTLS（端口587） | `false` |
| `auth_type` | string | 认证方式：`plain`/`login`/`crammd5`/`auto` | `plain` |
| `connect_timeout` | number | SMTP 连接超时时间（秒） | `30` |
| `send_timeout` | number | SMTP 发送超时时间（秒） | `30` |

### 5.3 加密与认证矩阵

| 端口 | 加密方式 | 认证方式 | 推荐配置 |
|------|----------|----------|----------|
| 465 | SSL/TLS | PLAIN / LOGIN | `ssl_enabled=true`, `auth_type=plain` |
| 587 | STARTTLS | PLAIN / LOGIN / CRAM-MD5 / AUTO | `starttls_enabled=true`, `auth_type=plain` |
| 25 | 无 | PLAIN / LOGIN | 不推荐（明文传输） |

### 5.4 常见 SMTP 服务配置示例

**QQ邮箱 / 163邮箱（SSL，端口465）**

```toml
ssl_enabled = true
starttls_enabled = false
auth_type = "plain"
port = 465
```

**Gmail / Outlook（STARTTLS，端口587）**

```toml
ssl_enabled = false
starttls_enabled = true
auth_type = "plain"
port = 587
```

**企业邮箱（常见配置）**

```toml
ssl_enabled = true
starttls_enabled = false
auth_type = "login"   # 部分企业邮箱要求 LOGIN 认证
port = 465
```

---

## 六、API 接口 (Admin)

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/message/templates | 模板列表分页查询 |
| POST | /admin/v1/message/templates | 创建消息模板 |
| PUT | /admin/v1/message/templates | 修改消息模板 |
| DELETE | /admin/v1/message/templates/:id | 删除消息模板 |
| GET | /admin/v1/message/records | 消息发送流水查询 |
| POST | /admin/v1/message/send | 管理员手动发送（调试用） |
| POST | /admin/v1/message/records/:id/retry | 失败消息重发 |

### 6.1 消息记录查询参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `current` | int | 当前页码 |
| `size` | int | 每页条数 |
| `channel` | string | 通道筛选：`sms`/`email`/`internal`/`push` |
| `receiver` | string | 接收人（支持模糊搜索） |
| `status` | int | 状态筛选：`0`=等待, `1`=成功, `2`=失败 |

### 6.2 直接发送请求体

```json
POST /admin/v1/message/send
{
  "channel": "email",
  "receiver": "user@example.com",
  "title": "测试邮件",
  "content": "这是一封测试邮件"
}
```

---

## 七、二次开发示例

### 7.1 新增短信驱动（以"腾讯云 SMS"为例）

**1. 实现驱动接口**

```go
// internal/pkg/message/tencent_sms.go
type TencentSmsDriver struct {
    client *sms.Client
    appID  string
}

func (d *TencentSmsDriver) Send(ctx context.Context, receiver string, title string, content string) error {
    req := sms.NewSendSmsRequest()
    req.PhoneNumberSet = []*string{&receiver}
    _, err := d.client.SendSms(req)
    return err
}
```

**2. 在 Wire 中注册**

```go
// internal/app/wire.go
drivers["sms"] = message.NewTencentSmsDriver(cfg.Sms)
```

### 7.2 扩展站内信通知

**1. 调用服务发送站内信**

```go
func (s *orderService) OnOrderPaid(ctx context.Context, order *entity.Order) {
    s.msgSvc.SendDirect(ctx, "internal", order.UserID, "支付成功", "您的订单已支付完成")
}
```

**2. 前端展示逻辑**

前端通过轮询或 WebSocket 监听 `msg_records` 表中 `channel='internal'` 且 `userId` 匹配的记录。

---

## 八、最佳实践

1. **模板规范**：所有系统消息必须走 `SendTemplate`，便于非技术人员在后台修改文案。
2. **重试机制**：对于网络超时，驱动层应返回错误，让任务引擎触发重试。
3. **敏感脱敏**：在日志中记录发送内容时，注意对敏感字段（如验证码）进行掩码处理。
4. **频率限制**：发送前建议结合 `cache.RateLimit` 进行防刷控制。
5. **端口选择**：优先使用 SSL/TLS（端口465）或 STARTTLS（端口587），避免使用明文端口25。
6. **认证方式**：如遇认证失败，可尝试将 `auth_type` 从 `plain` 切换为 `login`。
7. **超时配置**：在低延迟网络环境下可适当降低 `connect_timeout` 和 `send_timeout` 以加快失败感知。

---

## 九、相关文档

- [Server架构设计](./server-architecture.md)
- [任务系统详解](./server-module-task.md)
- [用户模块详解](./server-module-user.md)
- [缓存模块详解](./server-module-cache.md)
- [状态码规范](./status-codes.md)
- [客户端API文档](./client-api/00-authentication.md)
