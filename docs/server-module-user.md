# 用户模块详解

本文档详细介绍 NetyAdmin 用户模块（面向终端用户/Client）的架构设计、接口规范及二次开发指南。

---

## 一、模块概述

用户模块是面向 C 端或移动端用户的独立体系，与管理员（Admin）完全物理隔离。支持多端登录、Token 哈希验证、软删除及安全加密。

### 1.1 核心特性

- **多端隔离**：独立于 Admin 的认证体系，使用独立的 JWT Secret 和存储表。
- **ULID 支持**：用户 ID 采用 ULID，具备可排序性且对数据库友好。
- **Token 哈希**：数据库存储 Token 哈希而非原文，支持主动失效特定终端的登录状态。
- **TokenStore 抽象**：支持多种存储后端（缓存/数据库），通过 `login_storage` 配置切换。
- **安全加固**：密码使用 bcrypt 加密，支持图形验证码与消息验证码协同校验。
- **登录锁定**：密码错误次数超限自动锁定账户，支持 TTL 自动解锁、管理员解锁、找回密码解锁。
- **灵活适配**：支持手机号、邮箱、用户名多种注册/登录方式。
- **OAuth 绑定基座**：内置 `OAuthBindingService`，提供第三方账号绑定关系的统一存储与查询能力（FindByOpenID / FindByUnionID / Bind / Unbind / ListByUserID），下游项目仅需实现 provider 适配（调用微信/支付宝/GitHub/Apple 等 API 换取 openid），无需重复实现绑定关系管理。读路径走 ConfigCache（L1+L2 链），写路径在事务提交后失效缓存。
- **多类型用户扩展**：通过 `TypedUserJWTAuth` 中间件 + `RegisterTypedAuthModule` 路由注册方法，支持下游项目接入角色专属鉴权路由（如 `/client/v1/{userType}/...`），基座不感知 userType 语义，保持通用性。

---

## 二、目录结构

```
server/internal/domain/entity/user/
├── user.go             # 用户实体、Token 哈希实体、OAuth 绑定实体（UserOAuthBinding）

server/internal/repository/user/
├── user.go             # 用户仓储实现（含 OAuth 绑定 CRUD：FindOAuthBinding / FindOAuthBindingByUnionID / FindOAuthBindingByUserProvider / CreateOAuthBinding / DeleteOAuthBinding / ListOAuthBindings）

server/internal/service/user/
├── user.go             # 用户业务逻辑 (核心: GetInfo/ChangePassword/UpdateProfile)
├── user_auth.go        # 认证逻辑 (Register/Login/RefreshToken/ResetPassword)
├── user_admin.go       # 管理逻辑 (List/Create/Update/Delete)
├── oauth_binding.go    # OAuth 绑定服务 (FindByOpenID/FindByUnionID/Bind/Unbind/ListByUserID，含缓存与事务)
└── verification.go     # 验证码逻辑 (SMS/Email)

server/internal/interface/client/http/handler/v1/
├── auth_handler.go     # 登录/验证码 Handler
└── user_handler.go     # 资料/密码 Handler

server/internal/middleware/
└── auth.go             # 客户端鉴权中间件 (UserJWTAuth + TypedUserJWTAuth)

server/internal/interface/client/http/router/
└── router.go           # ClientRouter.RegisterTypedAuthModule — 多类型用户路由注册入口
```

> 下游项目接入新 userType 时，无需修改基座 `ClientRouter`，只需在自身项目 wire 装配阶段调用 `clientRouter.RegisterTypedAuthModule(userType, module, accessor)` 即可。

---

## 三、数据模型

### 3.1 用户表 (`users`)

```go
type User struct {
    ID          string                `gorm:"primaryKey;size:26"`    // ULID
    Username    string                `gorm:"size:50;uniqueIndex"`   // 用户名
    Password    string                `gorm:"size:100;not null"`     // bcrypt 密文
    Nickname    string                `gorm:"size:50"`               // 昵称
    Phone       string                `gorm:"size:20"`               // 手机号
    Email       string                `gorm:"size:100"`              // 邮箱
    Avatar      string                `gorm:"size:255"`              // 头像
    Gender      string                `gorm:"size:1;default:0"`      // 0:未知, 1:男, 2:女
    Status      string                `gorm:"size:1;default:1"`      // 1:正常, 0:禁用
    LastLoginAt *time.Time            `json:"lastLoginAt"`
    LastLoginIP string                `gorm:"size:50"`
    CreatedAt   time.Time             `gorm:"autoCreateTime"`
    DeletedAt   soft_delete.DeletedAt `gorm:"softDelete:milli"`      // 毫秒级软删除
}
```

### 3.2 会话存储表

C 端采用**多端会话表**（按 platform 维度隔离），admin 端采用独立的 token 哈希表，两表职责分离：

#### 3.2.1 客户端多端会话表 (`user_tokens`)

按 `(user_id, platform)` 唯一，每个 platform 一行——同 platform 重新登录顶掉旧会话（顶号），不同 platform 各自独立（多端并存）。

```go
type UserToken struct {
    ID               uint       `gorm:"primaryKey"`
    UserID           string     `gorm:"size:26;not null;index"`
    Platform         string     `gorm:"size:50;not null;uniqueIndex"` // web/mobile/miniapp...
    TokenVersion     uint64     `gorm:"not null;default:0"`            // 端级版本号，Login 递增（顶号依据）
    AccessHash       string     `gorm:"size:64"`                       // 当前 access token 哈希
    RefreshHash      string     `gorm:"size:64"`                       // 当前 refresh token 哈希
    AccessExpiresAt  *time.Time                                        // 用于过期清理
    RefreshExpiresAt *time.Time
    CreatedAt        time.Time  `gorm:"autoCreateTime"`
    UpdatedAt        time.Time  `gorm:"autoUpdateTime"`
}
```

#### 3.2.2 管理端 Token 哈希表 (`admin_tokens`)

admin 专用，存储已签发的 AccessToken 哈希，支持登出/强制下线后立即失效。

```go
type AdminToken struct {
    ID        uint      `gorm:"primaryKey"`
    UserID    string    `gorm:"size:26;index"`                  // 存储 admin_id
    TokenHash string    `gorm:"size:64;not null"`               // SHA256(token)
    ExpiredAt time.Time `gorm:"index"`
}
```

### 3.3 OAuth 绑定表 (`user_oauth_bindings`)

存储第三方账号与系统用户的绑定关系，支持微信/支付宝/GitHub/Apple 等多 provider 接入。

```go
type UserOAuthBinding struct {
    ID        uint      `gorm:"primaryKey"`
    UserID    string    `gorm:"size:26;index;not null"`           // 系统用户 ID（ULID）
    Provider  string    `gorm:"size:32;not null"`                  // 第三方渠道：wechat / alipay / github / apple ...
    OpenID    string    `gorm:"size:128;not null"`                 // provider 内唯一标识
    UnionID   string    `gorm:"size:128"`                          // 跨应用唯一标识（微信 unionid 场景）
    CreatedAt time.Time `gorm:"autoCreateTime"`
}

// 唯一约束：(provider, openid) — 防止同一第三方账号绑定多个系统用户
// 索引：(provider, unionid) — 支持微信 unionid 跨应用登录查找
// 索引：(user_id) — 支持 ListByUserID 查询用户已绑定的所有第三方账号
```

DDL 详见迁移脚本 `server/internal/pkg/migration/migrations/0054_user_oauth_bindings.up.sql`。

---

## 四、API 接口

### 4.1 认证接口 (Public)

| Method | Path | 说明 |
|--------|------|------|
| POST | /client/v1/user/register | 用户注册 (含验证码校验) |
| POST | /client/v1/user/login | 用户登录 (返回 Access/Refresh Token) |
| GET | /client/v1/auth/scene-config | 获取场景验证配置 (图形验证码+消息验证码开关) |
| GET | /client/v1/auth/captcha | 获取图形验证码 |
| POST | /client/v1/auth/send-code | 发送业务验证码 (SMS/Email) |
| POST | /client/v1/user/reset-password | 找回密码 (含验证码校验) |
| POST | /client/v1/user/refresh-token | 刷新令牌 |

### 4.2 资料接口 (Auth Required)

| Method | Path | 说明 |
|--------|------|------|
| GET | /client/v1/user/profile | 获取当前登录用户信息 |
| PUT | /client/v1/user/profile | 修改个人资料 |
| PUT | /client/v1/user/password | 修改密码 |
| DELETE | /client/v1/user/account | 注销账号 |
| GET | /client/v1/user/upload-token | 获取上传凭证 |
| POST | /client/v1/user/logout | 退出登录 (失效当前 Token) |

### 4.3 管理员接口 (Admin, RBAC)

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/systemManage/users | 用户列表 (含锁定状态) |
| PUT | /admin/v1/systemManage/users/:id/status | 更新用户状态 (停用时清除锁定) |
| POST | /admin/v1/systemManage/users/:id/unlock | 解锁用户 |

---

### 4.4 方法文件映射

| 方法 | 源文件 | 说明 |
|------|--------|------|
| Register / Login / RefreshToken / ResetPassword | `user_auth.go` | 认证相关 |
| List / Create / Update / Delete | `user_admin.go` | 管理相关 |
| GetInfo / ChangePassword / UpdateProfile | `user.go` | 核心用户操作 |

---

## 五、核心流程

### 5.1 注册流程

1. 调用 `GET /client/v1/auth/scene-config?scene=register` 获取验证配置。
2. 若 `captchaEnabled=true`，调用 `GET /client/v1/auth/captcha` 获取图形验证码。
3. 若 `verifyEnabled=true`，调用 `POST /client/v1/auth/send-code` 发送验证码（需携带图形验证码）。
4. 调用 `POST /client/v1/user/register` 提交注册信息。
5. 后端 `UserService.Register` 校验验证码 -> 检查唯一性 -> Bcrypt 加密 -> 存库。

### 5.2 登录与 Token 管理

- **双 Token 机制**：登录成功返回 `accessToken` (短效) 和 `refreshToken` (长效)。
- **多端会话**：C 端登录时按 `platform` 维度 UPSERT `user_tokens` 表，递增端级 `token_version`。同 platform 再次登录顶掉旧会话（顶号），不同 platform 各自独立（多端并存）。详见 [客户端 API 文档 - 多端登录与会话管理](./client-api-ws/02-user.md#多端登录与会话管理)。
- **Token 哈希**：C 端 access/refresh 哈希存入 `user_tokens` 表（按 platform 维度）；admin 端存入 `admin_tokens` 表。
- **双版本校验**：中间件同时校验用户级版本号（`users.token_version`，admin 敏感操作递增顶所有端）和端级版本号（`user_tokens.token_version`，Login 递增顶同端）。
- **RefreshToken 黑名单**：刷新令牌后，旧 RefreshToken 立即加入缓存黑名单（TTL 等于 Token 剩余有效期），防止已使用的 RefreshToken 被重放。黑名单 Key 由 `cache.KeyAuthBlacklistRefreshToken(token)` 工厂函数生成。
- **校验**：`UserJWTAuth` 中间件解析 Token 后，执行双版本校验 + hash 校验（走 L2 缓存加速）。

### 5.3 登录存储介质

C 端会话固定走 `user_tokens` 表（按 platform 维度），不再依赖 `login_storage` 配置切换缓存/DB 模式（该配置保留但仅影响 admin 端 tokenStore 的缓存层）。

| 值 | 说明 | 适用场景 |
|---|---|---|
| `cache` | 缓存模式（Redis/BigCache） | admin 端 tokenStore 加速层（推荐，支持多机部署） |
| `db` | 数据库模式（admin_tokens 表） | 无 Redis 环境，admin 会话持久化 |

> **注意**：此配置仅影响 admin 端 tokenStore 的缓存层。C 端会话固定走 `user_tokens` 表 + 独立 L2 缓存层（按 platform 维度），不受此配置控制。

### 5.4 账户锁定机制

当用户登录密码错误次数超过限制时，系统自动锁定账户：

- **锁定触发**：密码错误次数达到 `login_max_retry`（默认 5 次）时自动锁定
- **锁定时长**：由 `login_lock_duration`（默认 3600 秒）控制，TTL 到期自动解锁
- **缓存 Key**：`KeyLoginLock(userID)` 和 `KeyLoginRetryCount(userID)`
- **解锁方式**：
  1. **TTL 自动过期**：锁定时间到期后自动解锁
  2. **管理员解锁**：Admin 端调用 `POST /admin/v1/systemManage/users/:id/unlock`
  3. **找回密码解锁**：成功找回密码后自动清除锁定状态
- **停用/删除联动**：管理员停用或删除用户时，自动清除该用户的锁定和重试计数缓存

---

## 六、二次开发示例

### 6.1 扩展用户属性 (以"金币余额"为例)

**1. 修改实体**

```go
// internal/domain/entity/user/user.go
type User struct {
    // ... 现有字段
    Coins int64 `gorm:"default:0"` // 新增金币字段
}
```

**2. 增加 DTO 字段**

```go
// internal/domain/vo/user/user.go
type UserInfoVO struct {
    // ...
    Coins int64 `json:"coins"`
}
```

**3. 在 Service 中处理逻辑**

```go
func (s *userService) AddCoins(ctx context.Context, userID string, amount int64) error {
    return s.repo.UpdateFields(ctx, userID, map[string]interface{}{
        "coins": gorm.Expr("coins + ?", amount),
    })
}
```

### 6.2 实现三方登录 (以"微信登录"为例)

基座已内置 `OAuthBindingService`，下游项目无需自行创建绑定表和仓储，只需实现 provider 适配器（调用第三方 API 换取 openid/unionid），然后委托给基座服务完成绑定关系的存储与查询。

**1. 注入 OAuthBindingService**

基座 `wire_services.go` 已默认装配 `OAuthBindingService`（注入 `OAuthBindingRepo`、`TxManager`、`ConfigCache`）。下游项目如需在自身 Service 中调用，通过依赖注入获取即可：

```go
// 下游项目的 Service
type myAuthService struct {
    oauthBinding userSvc.OAuthBindingService  // 基座提供
    userRepo     userRepo.UserRepository      // 基座提供
    jwt          *jwt.JWT
    // ...
}

func NewMyAuthService(oauthBinding userSvc.OAuthBindingService, ...) *myAuthService { ... }
```

**2. 编写微信登录逻辑（调用基座服务）**

```go
func (s *myAuthService) LoginByWechat(ctx context.Context, code string) (*userVO.UserLoginVO, error) {
    // 1. 调用微信接口获取 OpenID + UnionID（下游项目自实现 provider 适配）
    openID, unionID, err := s.wechatClient.ExchangeCode(ctx, code)
    if err != nil {
        return nil, errorx.New(errorx.CodeInternalError, "微信授权失败")
    }

    // 2. 走基座缓存的 FindByOpenID 查找绑定关系（高频读路径，L1+L2 缓存）
    bound, err := s.oauthBinding.FindByOpenID(ctx, "wechat", openID)
    if err != nil {
        return nil, err
    }
    if bound == nil {
        // 未绑定：可选自动注册新用户，然后调用 Bind 建立绑定关系
        user, err := s.registerNewWechatUser(ctx, openID, unionID)
        if err != nil {
            return nil, err
        }
        // Bind 内部走事务 + UNIQUE 冲突映射 CodeOAuthAlreadyBound
        if err := s.oauthBinding.Bind(ctx, user.ID, "wechat", openID, unionID); err != nil {
            return nil, err
        }
        return s.issueTokens(ctx, user)
    }

    // 3. 已绑定：签发 token
    user, err := s.userRepo.GetByID(ctx, bound.UserID)
    if err != nil {
        return nil, err
    }
    return s.issueTokens(ctx, user)
}
```

**3. 缓存与并发安全说明**

| 关注点 | 基座行为 | 下游项目需关注 |
|--------|---------|---------------|
| 读路径缓存 | `FindByOpenID` / `FindByUnionID` 走 `ConfigCache.FetchFast`（TTL 30min），自动适配 L1（BigCache）/L2（Redis） | 无需感知，缓存命中时无 DB 查询 |
| 并发绑定冲突 | `Bind` 用 `tm.WithTransaction` 包裹 check-then-create；并发下捕获 `pgconn.PgError` Code "23505" 并映射为 `CodeOAuthAlreadyBound` | 直接 return err，由 Handler 经 `response.Fail` 自动输出业务错误码 |
| 解绑缓存失效 | `Unbind` 先查 binding 拿 openid/unionid，删除后精确失效对应 cache key + 按 userID tag 失效 | 无需手动清缓存 |
| 服务层解耦 | `OAuthBindingDTO` 仅含 `UserID/Provider/OpenID/UnionID`，不含 `ID/CreatedAt` 等持久化字段 | DTO 字段不可扩展为持久化字段（违反 [server-architecture.md §5.4](./server-architecture.md#54-dtoentity-隔离规范)） |

**4. 多类型用户路由接入（可选）**

若下游项目需要为新角色（如"技师"、"商户"）接入专属鉴权路由，复用基座 `ClientRouter.RegisterTypedAuthModule`：

```go
// 下游项目 wire 装配阶段
clientRouter.RegisterTypedAuthModule("tech", techModule, techClaimsAccessor)
// 注册后路由：
//   /client/v1/tech/public  — 无鉴权（OAuth 回调、登录端点）
//   /client/v1/tech         — TypedUserJWTAuth(techClaimsAccessor) 应用
```

详见 [server-architecture.md §5.5 多类型用户路由](./server-architecture.md)。

---

## 七、最佳实践

1. **唯一性检查**：注册时务必并发安全地检查 `username`、`email`、`phone`。
2. **软删除隔离**：使用 `soft_delete` 插件时，确保唯一索引包含 `deleted_at` 字段。
3. **敏感操作**：修改密码或注销账号前，建议二次校验消息验证码。
4. **Token 清理**：`token_hash_cleanup` 定时任务（每小时）自动清理 `user_tokens` 和 `admin_tokens` 表中过期的记录，无需手动干预。
5. **锁定策略**：生产环境建议 `login_max_retry` 设为 5，`login_lock_duration` 设为 3600 秒以上。
6. **多机部署**：多机部署时务必启用 Redis，C 端会话缓存（user_tokens L2 层）和 admin 端 tokenStore 均依赖 Redis 跨节点共享。

---

## 八、相关文档

- [Server架构设计](./server-architecture.md)
- [统一消息模块](./server-module-message.md)
- [IP 访问控制](./server-module-ipac.md)
- [缓存模块详解](./server-module-cache.md)
- [客户端API文档](./client-api/00-authentication.md)
