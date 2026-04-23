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

---

## 二、目录结构

```
server/internal/domain/entity/user/
├── user.go             # 用户实体与 Token 哈希实体

server/internal/repository/user/
├── user.go             # 用户仓储实现

server/internal/service/user/
├── user.go             # 用户业务逻辑 (Register/Login/Profile/Lock/Unlock)
└── verification.go     # 验证码逻辑 (SMS/Email)

server/internal/interface/client/http/handler/v1/
├── auth_handler.go     # 登录/验证码 Handler
└── user_handler.go     # 资料/密码 Handler
```

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

### 3.2 Token 哈希表 (`user_token_hashes`)

用于存储已签发的 AccessToken 哈希，支持多端登录管理。

```go
type UserTokenHash struct {
    ID        uint      `gorm:"primaryKey"`
    UserID    string    `gorm:"size:26;index"`
    TokenHash string    `gorm:"size:64;not null"` // SHA256(token)
    ExpiredAt time.Time `gorm:"index"`
}
```

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

## 五、核心流程

### 5.1 注册流程

1. 调用 `GET /client/v1/auth/scene-config?scene=register` 获取验证配置。
2. 若 `captchaEnabled=true`，调用 `GET /client/v1/auth/captcha` 获取图形验证码。
3. 若 `verifyEnabled=true`，调用 `POST /client/v1/auth/send-code` 发送验证码（需携带图形验证码）。
4. 调用 `POST /client/v1/user/register` 提交注册信息。
5. 后端 `UserService.Register` 校验验证码 -> 检查唯一性 -> Bcrypt 加密 -> 存库。

### 5.2 登录与 Token 管理

- **双 Token 机制**：登录成功返回 `accessToken` (短效) 和 `refreshToken` (长效)。
- **Token 存储**：通过 `TokenStore` 抽象层管理，支持缓存和数据库两种存储后端。
- **校验**：`UserJWTAuth` 中间件解析 Token 后，根据存储后端校验 Token 有效性。

### 5.3 登录存储介质

通过 `sys_configs` 表 `user_config` 分组的 `login_storage` 配置项控制 Token 存储方式：

| 值 | 说明 | 适用场景 |
|---|---|---|
| `cache` | 缓存模式（Redis/BigCache） | 推荐，支持多机部署，自动过期清理 |
| `db` | 数据库模式（user_token_hashes 表） | 无 Redis 环境，数据持久化 |

> **注意**：`cache` 模式使用系统的缓存模块（LazyCacheManager），自动适配单机（BigCache）和集群（Redis）部署。多机部署时必须选择 `cache` 模式并启用 Redis。

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

**1. 创建三方关联表**

```go
type UserOAuth struct {
    ID       uint   `gorm:"primaryKey"`
    UserID   string `gorm:"size:26;index"`
    Provider string `gorm:"size:20"` // wechat, github
    OpenID   string `gorm:"size:100;uniqueIndex"`
}
```

**2. 编写登录逻辑**

```go
func (s *userService) LoginByWechat(ctx context.Context, code string) (*userVO.UserLoginVO, error) {
    // 1. 调用微信接口获取 OpenID
    openID := s.wechat.GetOpenID(code)
    
    // 2. 查找关联用户
    oauth, _ := s.repo.GetOAuth(ctx, "wechat", openID)
    if oauth == nil {
        // 执行自动注册或返回引导绑定错误
        return nil, errorx.CodeUserNotFound
    }
    
    // 3. 执行常规登录发放 Token 流程
    user, _ := s.repo.GetByID(ctx, oauth.UserID)
    return s.issueTokens(ctx, user)
}
```

---

## 七、最佳实践

1. **唯一性检查**：注册时务必并发安全地检查 `username`、`email`、`phone`。
2. **软删除隔离**：使用 `soft_delete` 插件时，确保唯一索引包含 `deleted_at` 字段。
3. **敏感操作**：修改密码或注销账号前，建议二次校验消息验证码。
4. **Token 清理**：使用 `db` 存储模式时，建议定期运行任务清理 `user_token_hashes` 中过期的记录；使用 `cache` 模式时自动过期。
5. **锁定策略**：生产环境建议 `login_max_retry` 设为 5，`login_lock_duration` 设为 3600 秒以上。
6. **多机部署**：多机部署时务必使用 `cache` 存储模式并启用 Redis，确保 Token 和锁定状态在所有节点间共享。

---

## 八、相关文档

- [Server架构设计](./server-architecture.md)
- [统一消息模块](./server-module-message.md)
- [IP 访问控制](./server-module-ipac.md)
- [缓存模块详解](./server-module-cache.md)
- [客户端API文档](./client-api/00-authentication.md)
