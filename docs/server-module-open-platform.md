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
- **存储绑定**：每个应用可绑定独立的存储配置，未绑定时自动回退到全局默认存储源。

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
├── app.go              # 应用管理逻辑 (Verify/Limit/GetAppStorageDriver)
└── open_log.go         # 审计日志记录

server/internal/middleware/
└── open_platform_auth.go # 【核心】签名验证中间件

server/internal/interface/client/http/
├── handler/v1/storage_handler.go  # Client端存储上传Handler
├── router/v1/storage_router.go    # Client端存储路由注册
└── dto/v1/storage.go              # Client端存储DTO

server/internal/interface/admin/dto/open_platform/
└── app.go              # Admin端应用DTO（含StorageID字段）
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
- `msg_read`: 站内信读取。
- `content_view`: 内容查看（分类、文章、Banner）。
- `storage_upload`: 文件上传（获取凭证、创建记录）。

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
    StorageID   uint   `gorm:"default:0"`          // 绑定的存储配置ID，0表示使用全局默认
}
```

> **存储绑定机制**：当 `StorageID > 0` 时，该应用的所有上传操作使用指定的存储配置；当 `StorageID = 0` 时，自动回退到全局默认存储配置。

---

## 五、API 接口 (Admin)

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/open-platform/apps | 应用列表查询 |
| POST | /admin/v1/open-platform/apps | 创建新应用 (自动生成密钥) |
| PUT | /admin/v1/open-platform/apps | 修改应用信息/权限范围/存储绑定 |
| POST | /admin/v1/open-platform/apps/reset-secret | 重置 AppSecret |
| GET | /admin/v1/open-platform/logs | API 调用审计日志查询 |

> **存储绑定**：创建和修改应用时，可通过 `storageId` 字段指定绑定的存储配置。`storageId = 0` 表示使用全局默认存储。

## 六、二次开发示例

以下 API 已通过数据迁移注册至 `sys_open_apis`，默认应用 `01JQDEFAULTAPP001` 已绑定 `content_view` / `msg_read` / `user_base` / `user_profile` 等 Scope，可直接调用。

### 6.1 用户认证（公开）

| Method | Path | 说明 |
|--------|------|------|
| POST | /client/v1/user/register | C端用户注册 |
| POST | /client/v1/user/login | C端用户登录 |
| POST | /client/v1/user/refresh-token | 刷新访问令牌 |
| POST | /client/v1/user/reset-password | 通过验证码重置密码 |

### 6.2 验证码（公开）

| Method | Path | 说明 |
|--------|------|------|
| GET | /client/v1/auth/scene-config | 获取场景验证配置（图形验证码+消息验证码开关） |
| GET | /client/v1/auth/captcha | 获取图形验证码 |
| POST | /client/v1/auth/send-code | 发送短信/邮件验证码 |

### 6.3 用户资料（需签名）

| Method | Path | 说明 |
|--------|------|------|
| GET | /client/v1/user/profile | 获取当前用户资料 |
| PUT | /client/v1/user/profile | 修改当前用户资料 |
| PUT | /client/v1/user/password | 修改当前用户密码 |
| DELETE | /client/v1/user/account | 注销当前用户账户 |
| GET | /client/v1/user/upload-token | 获取存储上传凭证 |
| POST | /client/v1/user/logout | 退出登录使令牌失效 |

### 6.4 站内信（需签名，Scope: `msg_read`）

| Method | Path | 说明 |
|--------|------|------|
| GET | /client/v1/message/internal | 获取当前用户站内信列表 |
| GET | /client/v1/message/internal/:id | 获取单条站内信内容 |
| PUT | /client/v1/message/internal/read | 标记指定站内信为已读 |
| PUT | /client/v1/message/internal/read-all | 标记所有站内信为已读 |
| GET | /client/v1/message/internal/unread-count | 获取未读站内信数量 |

### 6.5 内容管理（公开，Scope: `content_view`）

| Method | Path | 说明 |
|--------|------|------|
| GET | /client/v1/content/categories/tree | 获取启用的分类树 |
| GET | /client/v1/content/articles?categoryId=1 | 获取指定分类及子分类下的已发布文章列表 |
| GET | /client/v1/content/article/:id | 获取已发布文章详情 |
| GET | /client/v1/content/banners/:code | 通过编码获取Banner组及有效项 |

### 6.6 内容管理（需签名，Scope: `content_view`）

| Method | Path | 说明 |
|--------|------|------|
| POST | /client/v1/content/article/:id/like | 点赞指定文章 |
| POST | /client/v1/content/banners/:id/click | 记录Banner点击 |

### 6.7 存储上传（需签名，Scope: `storage_upload`）

| Method | Path | 说明 |
|--------|------|------|
| POST | /client/v1/storage/credentials | 获取上传凭证（自动使用应用绑定的存储配置） |
| POST | /client/v1/storage/records | 创建上传记录 |

> **存储绑定自动适配**：Client 端上传接口会根据请求中的应用身份自动选择存储配置。若应用绑定了 `storageId`，则使用该配置；否则回退到全局默认存储。

---

## 七、二次开发示例

### 7.1 为 API 增加 Scope 限制

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

### 7.2 客户端签名计算 (Node.js 示例)

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

## 八、最佳实践

1. **Secret 安全**：`AppSecret` 在数据库中必须 AES 加密存储，在 UI 界面默认不回显。
2. **时钟同步**：客户端与服务器时间偏差不得超过 60s，否则请求失效。
3. **日志清理**：开放平台日志增长较快，建议结合 `task_config` 设置 30 天自动清理。
4. **IPAC 联动**：建议为合作伙伴应用开启白名单模式（`IPStrategy=2`），仅允许其固定服务器 IP 访问。
5. **缓存同步**：应用创建、更新、删除时，系统会自动触发 IPAC 缓存重载，确保 IP 策略实时生效。
6. **存储绑定**：为需要资源隔离的应用绑定独立存储配置，避免不同应用的文件混存于同一存储桶。未绑定的应用自动使用全局默认存储，无需额外配置。

---

## 九、相关文档

- [Server架构设计](./server-architecture.md)
- [IP 访问控制](./server-module-ipac.md)
- [缓存模块详解](./server-module-cache.md)
- [客户端API文档](./client-api/00-authentication.md)
