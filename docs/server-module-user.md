# 用户模块详解

本文档详细介绍 NetyAdmin 用户模块（面向终端用户/Client）的架构设计、接口规范及二次开发指南。

---

## 一、模块概述

用户模块是面向 C 端或移动端用户的独立体系，与管理员（Admin）完全物理隔离。支持多端登录、Token 哈希验证、软删除及安全加密。

### 1.1 核心特性

- **多端隔离**：独立于 Admin 的认证体系，使用独立的 JWT Secret 和存储表。
- **ULID 支持**：用户 ID 采用 ULID，具备可排序性且对数据库友好。
- **Token 哈希**：数据库存储 Token 哈希而非原文，支持主动失效特定终端的登录状态。
- **安全加固**：密码使用 bcrypt 加密，支持图形验证码与消息验证码协同校验。
- **灵活适配**：支持手机号、邮箱、用户名多种注册/登录方式。

---

## 二、目录结构

```
server/internal/domain/entity/user/
├── user.go             # 用户实体与 Token 哈希实体

server/internal/repository/user/
├── user.go             # 用户仓储实现

server/internal/service/user/
├── user.go             # 用户业务逻辑 (Register/Login/Profile)
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
| GET | /client/v1/auth/captcha | 获取图形验证码 |
| POST | /client/v1/auth/send-code | 发送业务验证码 (SMS/Email) |
| GET | /client/v1/user/refresh-token | 刷新令牌 |

### 4.2 资料接口 (Auth Required)

| Method | Path | 说明 |
|--------|------|------|
| GET | /client/v1/user/profile | 获取当前登录用户信息 |
| PUT | /client/v1/user/profile | 修改个人资料 |
| PUT | /client/v1/user/password | 修改密码 |
| POST | /client/v1/user/logout | 退出登录 (失效当前 Token) |

---

## 五、核心流程

### 5.1 注册流程

1. 调用 `fetchGetVerifyConfig` 判断是否开启消息验证。
2. 调用 `fetchSendVerifyCode` 发送验证码（需携带图形验证码）。
3. 调用 `fetchClientRegister` 提交注册信息。
4. 后端 `UserService.Register` 校验验证码 -> 检查唯一性 -> Bcrypt 加密 -> 存库。

### 5.2 登录与 Token 管理

- **双 Token 机制**：登录成功返回 `accessToken` (短效) 和 `refreshToken` (长效)。
- **Token 存储**：后端计算 `SHA256(accessToken)` 并存入 `user_token_hashes`。
- **校验**：`UserJWTAuth` 中间件解析 Token 后，会对比数据库中的哈希，若记录不存在则视为 Token 已失效。

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
4. **Token 清理**：建议定期运行任务清理 `user_token_hashes` 中过期的记录。

---

## 八、相关文档

- [Server架构设计](./server-architecture.md)
- [统一消息模块](./server-module-message.md)
- [IP 访问控制](./server-module-ipac.md)
