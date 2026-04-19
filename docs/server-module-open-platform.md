# 开放平台模块详解

本文档详细介绍 NetyAdmin 开放平台（Open Platform）的架构设计、签名验证机制、限流策略及二次开发指南。

---

## 一、模块概述

开放平台为 Web、App、小程序及第三方合作伙伴提供标准的身份识别、安全签名、权限管控和流量治理能力。

### 1.1 核心特性

- **应用隔离**：通过 `AppKey/AppSecret` 实现应用级的物理隔离与配置。
- **签名验证**：基于 HMAC-SHA256 的强加密签名算法，防止请求被篡改。
- **权限管控**：细粒度的 `Scope` (权限范围) 校验，控制应用可调用的 API 集合。
- **分布式限流**：自适应的令牌桶算法，支持 Redis Lua 实现的跨节点精准计数。
- **防重放机制**：集成 `Nonce` 与时钟容差校验，确保请求不可重用。

---

## 二、目录结构

```
server/internal/domain/entity/open_platform/
├── app.go              # 应用实体与 Scope 定义
├── open_log.go         # 开放平台调用日志
└── scope_group.go      # 权限分组实体

server/internal/repository/open_platform/
├── app.go              # 应用仓储实现
└── open_log.go         # 日志仓储实现

server/internal/service/open_platform/
├── app.go              # 应用管理逻辑 (Verify/Limit)
└── open_log.go         # 审计日志记录

server/internal/middleware/
└── open_platform_auth.go # 【核心】签名验证中间件
```

---

## 三、安全机制

### 3.1 签名算法

所有开放接口请求头必须包含：
- `X-App-Key`: 应用唯一标识
- `X-Timestamp`: Unix 时间戳 (秒)
- `X-Nonce`: 随机字符串
- `X-Signature`: 签名结果

**待签名字符串 (StringToSign) 构造规则**：
```text
Method + "\n" +
Path + "\n" +
Timestamp + "\n" +
Nonce + "\n" +
Payload (GET 为排序后的参数, POST 为 Body 的 SHA256)
```
**签名计算**：`Base64(HmacSHA256(AppSecret, StringToSign))`

### 3.2 权限范围 (Scope)

API 开发者在注册路由时可标记所属 Scope。应用必须被授予该 Scope 才能访问：
- `user_base`: 基础注册登录。
- `user_profile`: 用户资料管理。
- `msg_send`: 消息下发。

---

## 四、数据模型

### 4.1 应用表 (`sys_apps`)

```go
type App struct {
    ID          string `gorm:"primaryKey;size:26"` // ULID
    AppKey      string `gorm:"size:26;uniqueIndex"`
    AppSecret   string `gorm:"size:255"`           // AES 加密存储
    Name        string `gorm:"size:100"`
    Status      int    `gorm:"default:1"`          // 1:启用, 0:禁用
    IPStrategy  int    `gorm:"default:1"`          // 1:黑名单, 2:白名单
    QuotaConfig string `gorm:"type:jsonb"`         // 限流配置 {"rate":10, "capacity":20}
}
```

---

## 五、API 接口 (Admin)

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/open-platform/apps | 应用列表查询 |
| POST | /admin/v1/open-platform/apps | 创建新应用 (自动生成密钥) |
| PUT | /admin/v1/open-platform/apps | 修改应用信息/权限范围 |
| POST | /admin/v1/open-platform/apps/reset-secret | 重置 AppSecret |
| GET | /admin/v1/open-platform/logs | API 调用审计日志查询 |

---

## 六、二次开发示例

### 6.1 为 API 增加 Scope 限制

**1. 定义新的 Scope 常量**

```go
// internal/domain/entity/open_platform/app.go
const ScopeOrderAdmin = "order_admin"
```

**2. 在路由注册时标记**

```go
// internal/interface/client/http/router/v1/order.go
func (r *orderRouter) RegisterAuth(group *gin.RouterGroup) {
    // 使用 c.Set 标记所需 Scope，中间件会自动读取
    group.GET("/orders", func(c *gin.Context) {
        c.Set("requiredScope", open_platform.ScopeOrderAdmin)
    }, r.handler.List)
}
```

### 6.2 客户端签名计算 (Node.js 示例)

```javascript
const crypto = require('crypto');

function computeSignature(secret, method, path, timestamp, nonce, payload) {
    const stringToSign = `${method}\n${path}\n${timestamp}\n${nonce}\n${payload}`;
    return crypto.createHmac('sha256', secret)
                 .update(stringToSign)
                 .digest('base64');
}
```

---

## 七、最佳实践

1. **Secret 安全**：`AppSecret` 在数据库中必须 AES 加密存储，在 UI 界面默认不回显。
2. **时钟同步**：客户端与服务器时间偏差不得超过 60s，否则请求失效。
3. **日志清理**：开放平台日志增长较快，建议结合 `task_config` 设置 30 天自动清理。
4. **IPAC 联动**：建议为合作伙伴应用开启白名单模式（`IPStrategy=2`），仅允许其固定服务器 IP 访问。
5. **缓存同步**：应用创建、更新、删除时，系统会自动触发 IPAC 缓存重载，确保 IP 策略实时生效。

---

## 八、相关文档

- [Server架构设计](./server-architecture.md)
- [IP 访问控制](./server-module-ipac.md)
- [缓存模块详解](./server-module-cache.md)
