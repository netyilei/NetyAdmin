# 开放平台模块详解

本文档详细介绍 NetyAdmin 开放平台（Open Platform）的架构设计、签名验证机制、限流策略及二次开发指南。

---

## 一、模块概述

开放平台为 Web、App、小程序及第三方合作伙伴提供标准的身份识别、安全签名、权限管控和流量治理能力。

### 1.1 核心特性

- **应用隔离**：通过 `AppKey/AppSecret` 实现应用级的物理隔离与配置。
- **签名验证**：基于 HMAC-SHA256 的强加密签名算法，防止请求被篡改。
- **权限管控**：细粒度的 `Scope` (权限范围) 校验，控制应用可调用的 API 集合。
- **分布式限流**：固定窗口计数（`pkg/ratelimit`），Redis 启用时跨节点精准（Lua 原子、单次 RTT）；Redis 关闭时自动降级为进程内计数（仅限单机部署）。
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

### 3.3 规则缓存与热更新

开放平台的校验数据分三类，热更新路径不同：

| 数据 | 存储位置 | Admin 更新后的生效方式 |
|------|----------|------------------------|
| App 信息（状态/IP 开关/限流开关/quota/存储绑定） | L1 (BigCache) + L2 (Redis)，Tag `open:app` + `app_key:{key}` | `InvalidateByTags` 清本节点 L1+L2，PubSub 广播其他节点清 L1（窗口期通常 <100ms） |
| Scopes / 可访问 API 白名单 | L1 + L2（白名单 TTL 5 分钟兜底） | 同上；scope/API 关联变更同时失效 `open:app` |
| IP 黑白名单 | 进程内 CIDR trie 快照 | `NotifyAndReload` 本地重载后广播 `ipac_reload`，各节点重载 |
| 默认限流 rate/capacity | `configWatcher` 进程内 map | 系统配置保存后广播 `config_sync`，各节点 `ForceReload` |

要点：

- **L1 命中即返回**：App 信息 / Scopes / API 白名单在 L1 命中时不会访问 Redis（`GetFast`/`FetchFast`）。
- **热更新依赖事件总线**：多节点必须 `[bus].driver = "redis"`；`bus=memory` 时广播不出本进程。
- **失效广播失败**：其他节点的 L1 会陈旧至 `local_ttl_min`（默认 10 分钟）；`App.CacheTTL=0` 时 L2 不会自动过期，因此建议在管理端为应用配置合理缓存 TTL。
- **Redis 关闭（单机）**：App/Scopes/API 仍由 L1 服务；Nonce 与限流切换为进程内实现；多节点部署被启动校验禁止。

---

## 四、数据模型

### 4.1 应用表 (`sys_apps`)

```go
type App struct {
    ID               string `gorm:"primaryKey;size:26"` // ULID
    AppKey           string `gorm:"size:26;uniqueIndex"`
    AppSecret        string `gorm:"size:255"`           // AES 加密存储
    Name             string `gorm:"size:100"`
    Status           int    `gorm:"default:1"`          // 1:启用, 0:禁用
    IPStrategy       int    `gorm:"default:1"`          // 1:黑名单, 2:白名单
    IPFilterEnabled  bool   `gorm:"default:false"`      // 是否启用 IP 过滤
    RateLimitEnabled bool   `gorm:"default:true"`       // 是否启用限流
    QuotaConfig      string `gorm:"type:jsonb"`         // 限流配置 {"rate":100,"capacity":200}
    CacheTTL         int    `gorm:"default:0"`          // App 信息缓存 TTL（秒），0=不过期（依赖失效广播）
    Remark           string `gorm:"size:255"`
    StorageID        uint   `gorm:"default:0"`          // 绑定的存储配置ID，0表示使用全局默认
}
```

> **限流配置说明**：`QuotaConfig` 为 JSONB 格式，包含 `rate`（每秒补充的请求数）和 `capacity`（窗口内最大请求数）两个字段。未配置时使用系统配置 `open_platform_config.default_rate`（默认 100）与 `default_capacity`（默认 200）。

> **存储绑定机制**：当 `StorageID > 0` 时，该应用的所有上传操作使用指定的存储配置；当 `StorageID = 0` 时，自动回退到全局默认存储配置。

---

## 五、API 接口 (Admin)

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/open-platform/apps | 应用列表查询 |
| POST | /admin/v1/open-platform/apps | 创建新应用 (自动生成密钥) |
| PUT | /admin/v1/open-platform/apps | 修改应用信息/权限范围/存储绑定/限流配置 |
| POST | /admin/v1/open-platform/apps/reset-secret | 重置 AppSecret |
| GET | /admin/v1/open-platform/logs | API 调用审计日志查询 |

> **存储绑定**：创建和修改应用时，可通过 `storageId` 字段指定绑定的存储配置。`storageId = 0` 表示使用全局默认存储。

> **限流配置**：创建和修改应用时，可通过 `quotaConfig` 字段指定限流参数，格式为 JSON 字符串，例如 `{"rate":100, "capacity":200}`。`rate` 为每秒补充的请求数，`capacity` 为窗口内最大请求数。若不传或传空字符串，则使用系统配置 `open_platform_config.default_rate`（默认 100）/ `default_capacity`（默认 200）。

**创建/修改应用请求体示例**：

```json
{
  "name": "测试应用",
  "status": 1,
  "ipFilterEnabled": false,
  "remark": "备注信息",
  "quotaConfig": "{\"rate\":10,\"capacity\":20}",
  "storageId": 0,
  "scopes": ["content_view", "user_base"]
}
```

## 六、二次开发示例

以下 API 已通过数据迁移注册至 `sys_open_apis`，默认应用 `01JQDEFAULTAPP001` 已绑定 `content_view` / `msg_read` / `user_base` / `user_profile` 等 Scope，可直接调用。

> **默认应用凭据（开箱即用）**：克隆基座后默认应用 `01JQDEFAULTAPP001` 即可签名调用，
> 无需重置密钥：
>
> | 项 | 值 |
> |----|----|
> | AppKey | `01JQDEFAULTAPP001` |
> | AppSecret | `netyadmin-default-app-secret` |
>
> 该 Secret 为公开的默认值（与默认账号 `admin/admin123` 同理），**生产环境部署后必须通过
> `PUT /admin/v1/open/apps/reset-secret` 重置**。若修改了 `[security].aes_key`，默认应用密文将无法
> 解密，同样需要先 reset-secret（详见 §八 最佳实践 2）。

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

### 7.3 配置应用限流参数

开放平台使用**固定窗口计数**进行分布式限流（`pkg/ratelimit.Limiter`，底层 `ulule/limiter`）。每个应用可独立配置 `rate`（每秒补充的请求数）和 `capacity`（窗口内最大请求数）：窗口周期 = `capacity / rate` 秒（最小 1 秒），窗口内最多 `capacity` 次请求。

**通过 Admin API 配置**：

```bash
# 创建应用时指定限流配置
curl -X POST http://localhost:8010/admin/v1/open-platform/apps \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "高流量应用",
    "status": 1,
    "quotaConfig": "{\"rate\":100,\"capacity\":200}",
    "scopes": ["content_view"]
  }'

# 修改已有应用的限流配置
curl -X PUT http://localhost:8010/admin/v1/open-platform/apps \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "<app_id>",
    "name": "高流量应用",
    "status": 1,
    "quotaConfig": "{\"rate\":50,\"capacity\":100}",
    "scopes": ["content_view"]
  }'
```

**通过 Admin 界面配置**：在应用管理页面的新增/编辑表单中，"限流配置" 区域提供"填充速率"和"桶容量"两个输入框，分别对应 `rate` 和 `capacity`。

**限流行为说明**：

- 当 `QuotaConfig` 为空或未配置时，使用系统配置 `open_platform_config.default_rate`（默认 100）/ `default_capacity`（默认 200）
- `rate` 控制每秒补充的请求数，值越大允许的稳态请求速率越高
- `capacity` 控制窗口内最大请求数，值越大允许的瞬时突发量越高
- 限流由 `service/open_platform.AppService.AllowRequest` → `pkg/ratelimit.Limiter.Allow` 执行：
  Redis 启用时通过 EVALSHA（INCRBY + PEXPIRE 原子脚本）跨节点精准计数，**单次 Redis 往返**；
  Redis 关闭时自动降级为进程内内存计数（仅单机；多节点部署会在启动期被配置校验拒绝）

---

## 八、最佳实践

1. **Secret 安全**：`AppSecret` 在数据库中必须 AES 加密存储，在 UI 界面默认不回显。默认应用的种子 Secret 是公开默认值（`netyadmin-default-app-secret`），克隆基座后开箱即用；**生产环境部署后必须重置**，不得沿用默认值。
2. **AESKey 轮换注意**：`[security].aes_key` 变更后，DB 中所有 `sys_apps.app_secret` 用旧 key 加密的密文将**无法解密**——需逐个调用 `PUT /admin/v1/open/apps/reset-secret` 重置。特别地，默认应用种子密文是用默认 aes_key（`netyadmin-aes-key-32-chars-long!`）加密的，一旦你改了 aes_key，默认应用也需 reset-secret 才能继续使用。轮换前建议批量导出 app 列表，轮换后批量重置。生产环境轮换 AESKey 是高风险操作，建议在维护窗口进行。
3. **时钟同步**：客户端与服务器时间偏差不得超过 60s，否则请求失效。
4. **日志清理**：开放平台日志增长较快，建议结合 `task_config` 设置 30 天自动清理。
5. **IPAC 联动**：建议为合作伙伴应用开启白名单模式（`IPStrategy=2`），仅允许其固定服务器 IP 访问。
6. **缓存同步**：应用创建、更新、删除时，系统会自动触发 IPAC 缓存重载，确保 IP 策略实时生效。
6. **存储绑定**：为需要资源隔离的应用绑定独立存储配置，避免不同应用的文件混存于同一存储桶。未绑定的应用自动使用全局默认存储，无需额外配置。

---

## 九、相关文档

- [Server架构设计](./server-architecture.md)
- [IP 访问控制](./server-module-ipac.md)
- [缓存模块详解](./server-module-cache.md)
- [客户端API文档](./client-api/00-authentication.md)
