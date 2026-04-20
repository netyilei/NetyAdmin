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
├── email_driver.go     # SMTP 驱动实现
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
    Channel    string    `gorm:"size:20"`
    Receiver   string    `gorm:"size:100"` // 手机号/邮箱/Token
    Title      string    `gorm:"size:200"`
    Content    string    `gorm:"type:text"`
    Status     int       `gorm:"default:0"` // 0:等待, 1:成功, 2:失败
    ErrorMsg   string    `gorm:"type:text"`
    Priority   int       `gorm:"default:2"` // 1:高, 2:中, 3:低
    RetryCount int       `gorm:"default:0"`
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

> **说明**：站内信通过 `msg_internal` 表扩展，支持系统公告（全员）和私信（指定用户）两种类型。

---

## 四、核心逻辑

### 4.1 发送流程

1. **业务调用**：`MessageService.SendTemplate(ctx, "VERIFY_CODE", "user@example.com", params)`。
2. **模板加载**：从缓存/数据库获取模板内容。
3. **内容渲染**：将 `{{code}}` 替换为具体数值。
4. **记录入库**：创建 `msg_records`，初始状态为 0。
5. **投递任务**：调用 `Dispatcher.Dispatch` 投递 `msg_send_job` 任务。
6. **异步执行**：`MsgSendJob` 消费者调用对应通道驱动，物理下发消息。

### 4.2 站内信特殊处理

站内信（`channel=internal`）不依赖外部驱动，由系统直接处理：

1. **消息记录**：创建 `msg_records` 记录，状态直接设为成功。
2. **扩展记录**：同时创建 `msg_internal` 扩展记录，关联 `msg_record_id`。
3. **类型区分**：
   - `type=1`：系统公告（`receiver="all"` 时自动设置）
   - `type=2`：私信（指定用户时）

```go
// MsgSendJob.Execute 中的 internal channel 处理
if rec.Channel == "internal" {
    rec.Status = msgEntity.MsgStatusSuccess
    // 更新记录状态
    // 创建 msg_internal 扩展记录
}
```

### 4.3 驱动接口

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

## 五、API 接口 (Admin)

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/message/templates | 模板列表分页查询 |
| POST | /admin/v1/message/templates | 创建消息模板 |
| PUT | /admin/v1/message/templates | 修改消息模板 |
| DELETE | /admin/v1/message/templates/:id | 删除消息模板 |
| GET | /admin/v1/message/records | 消息发送流水查询 |
| POST | /admin/v1/message/send/direct | 管理员手动发送 (调试用) |

---

## 六、二次开发示例

### 6.1 新增短信驱动 (以"腾讯云 SMS"为例)

**1. 实现驱动接口**

```go
// internal/pkg/message/tencent_sms.go
type TencentSmsDriver struct {
    client *sms.Client
    appID  string
}

func (d *TencentSmsDriver) Send(ctx context.Context, receiver string, title string, content string) error {
    // 腾讯云通常需要 TemplateID，如果是 SendDirect 则可能需要特殊处理
    // 此处仅为示意
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

### 6.2 扩展站内信通知

**1. 调用服务发送站内信**

```go
func (s *orderService) OnOrderPaid(ctx context.Context, order *entity.Order) {
    s.msgSvc.SendDirect(ctx, "internal", order.UserID, "支付成功", "您的订单已支付完成")
}
```

**2. 前端展示逻辑**

前端通过轮询或 WebSocket 监听 `msg_records` 表中 `channel='internal'` 且 `userId` 匹配的记录。

---

## 七、最佳实践

1. **模板规范**：所有系统消息必须走 `SendTemplate`，便于非技术人员在后台修改文案。
2. **重试机制**：对于网络超时，驱动层应返回错误，让任务引擎触发重试。
3. **敏感脱敏**：在日志中记录发送内容时，注意对敏感字段（如验证码）进行掩码处理。
4. **频率限制**：发送前建议结合 `cache.RateLimit` 进行防刷控制。

---

## 八、相关文档

- [Server架构设计](./server-architecture.md)
- [任务系统详解](./server-module-task.md)
- [用户模块详解](./server-module-user.md)
