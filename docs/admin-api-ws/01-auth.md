# 认证管理 API

本模块提供管理员登录、令牌刷新、个人信息管理、密码修改及退出登录等接口。

## 接口概览

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| POST | `/admin/v1/auth/login` | 公开 | 管理员登录 |
| POST | `/admin/v1/auth/refresh-token` | 公开 | 刷新令牌 |
| GET | `/admin/v1/auth/user-info` | 认证 | 获取当前登录用户信息 |
| GET | `/admin/v1/auth/profile` | 认证 | 获取个人资料 |
| PUT | `/admin/v1/auth/profile` | 认证 | 更新个人资料 |
| POST | `/admin/v1/auth/change-password` | 认证 | 修改密码 |
| POST | `/admin/v1/auth/logout` | 认证 | 退出登录 |

---

## 1. 管理员登录

管理员使用用户名和密码登录，可选验证码校验，成功后返回访问令牌与刷新令牌。

```
POST /admin/v1/auth/login
```

### 认证级别

公开接口（无需 Token）

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | string | 否* | 用户名（与 `userName` 二选一） |
| `userName` | string | 否* | 用户名（与 `username` 二选一） |
| `password` | string | 是 | 密码 |
| `captchaId` | string | 否 | 验证码 ID（开启验证码时必填） |
| `captchaValue` | string | 否 | 验证码值（开启验证码时必填） |

> *`username` 和 `userName` 两个字段任选其一，`username` 优先。若系统配置 `captcha_config.admin_login_enabled` 为 `true` 或 `1`，则 `captchaId` 和 `captchaValue` 必填。

### 请求示例

```json
{
  "username": "admin",
  "password": "123456",
  "captchaId": "abc123def456",
  "captchaValue": "8g3k"
}
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `token` | string | 访问令牌（AccessToken），有效期 30 分钟（按 `[jwt].access_token_ttl` 配置） |
| `refreshToken` | string | 刷新令牌（RefreshToken），有效期 168 小时/7 天（按 `[jwt].refresh_token_ttl` 配置） |

### 安全机制

- 连续 **5 次**密码错误将锁定账户 **15 分钟**
- 锁定期间登录直接返回 `101007`（账户已锁定）
- 登录成功后清除失败计数并更新最后登录时间
- **IP 维度限流**（Task 3）：单 IP 在 `[login_ratelimit].window`（默认 1 分钟）内最多 `[login_ratelimit].max`（默认 10）次 Login / RefreshToken 请求，超限返回 `100006`（请求过于频繁，HTTP 429）；与 per-account 锁定独立，针对单 IP 撞库场景
- **Login 失败文案统一**（Task 3.4）：为消除用户名枚举，`101001` / `101002` / `101003` 在 Login 路径统一返回 msg 「用户名或密码错误」，前端不应据 msg 区分用户不存在 / 已禁用 / 密码错误

### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `100001` | 参数错误（用户名或密码为空） |
| `100006` | 请求过于频繁（IP 维度限流，HTTP 429） |
| `100009` | 验证码错误 |
| `100010` | 验证码必填 |
| `101001` | 用户不存在（msg 统一为「用户名或密码错误」） |
| `101002` | 用户已禁用（msg 统一为「用户名或密码错误」） |
| `101003` | 密码错误（msg 统一为「用户名或密码错误」） |
| `101007` | 账户已锁定 |

---

## 2. 刷新令牌

使用刷新令牌换取新的访问令牌与刷新令牌。

```
POST /admin/v1/auth/refresh-token
```

### 认证级别

公开接口（无需 Token）

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `refreshToken` | string | 是 | 刷新令牌 |

### 请求示例

```json
{
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `100001` | 参数错误 |
| `101005` | 令牌已过期 |
| `101006` | 令牌无效 |

---

## 3. 获取当前登录用户信息

获取当前登录管理员的用户 ID、用户名、角色及按钮权限信息。

```
GET /admin/v1/auth/user-info
```

### 认证级别

认证接口（需 Bearer Token）

### 请求示例

```
GET /admin/v1/auth/user-info
Authorization: Bearer <token>
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "userId": "1",
    "userName": "admin",
    "roles": ["super_admin"],
    "buttons": ["btn:add", "btn:edit", "btn:delete"]
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `userId` | string | 管理员 ID |
| `userName` | string | 用户名 |
| `roles` | []string | 角色编码列表 |
| `buttons` | []string | 按钮权限编码列表 |

---

## 4. 获取个人资料

获取当前登录管理员的个人资料信息。

```
GET /admin/v1/auth/profile
```

### 认证级别

认证接口（需 Bearer Token）

### 请求示例

```
GET /admin/v1/auth/profile
Authorization: Bearer <token>
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "id": 1,
    "userName": "admin",
    "nickName": "超级管理员",
    "userPhone": "13800138000",
    "userEmail": "admin@example.com",
    "userGender": "1",
    "status": "1",
    "createTime": "2025-01-01 12:00:00"
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uint | 管理员 ID |
| `userName` | string | 用户名 |
| `nickName` | string | 昵称 |
| `userPhone` | string | 手机号 |
| `userEmail` | string | 邮箱 |
| `userGender` | string | 性别（`0`:未知 `1`:男 `2`:女） |
| `status` | string | 状态（`0`:禁用 `1`:启用） |
| `createTime` | string | 创建时间 |

---

## 5. 更新个人资料

更新当前登录管理员的个人资料。

```
PUT /admin/v1/auth/profile
```

### 认证级别

认证接口（需 Bearer Token）

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `nickname` | string | 否 | 昵称 |
| `phone` | string | 否 | 手机号（修改时需配合 `phoneCode`） |
| `email` | string | 否 | 邮箱（需符合邮箱格式，最大 100 字符；修改时需配合 `emailCode`） |
| `gender` | string | 否 | 性别（`0`:未知 `1`:男 `2`:女） |
| `emailCode` | string | 条件必填 | 邮箱验证码（修改 `email` 时必填，需先调发送邮箱验证码接口获取） |
| `phoneCode` | string | 条件必填 | 手机验证码（修改 `phone` 时必填，需先调发送短信验证码接口获取） |

> **安全说明**：修改 `email` / `phone` 必须提供对应渠道的验证码，确保新联系方式归属当前用户。仅修改 `nickname` / `gender` 无需验证码。

### 请求示例

```json
{
  "nickname": "新昵称",
  "phone": "13900139000",
  "phoneCode": "123456",
  "email": "newadmin@example.com",
  "emailCode": "654321",
  "gender": "1"
}
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "资料修改成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

---

## 6. 修改密码

当前登录管理员通过旧密码修改登录密码。

```
POST /admin/v1/auth/change-password
```

### 认证级别

认证接口（需 Bearer Token）

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `oldPassword` | string | 是 | 原密码 |
| `newPassword` | string | 是 | 新密码（长度 8-32 字符） |

### 请求示例

```json
{
  "oldPassword": "old123456",
  "newPassword": "new123456"
}
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "密码修改成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `100001` | 参数错误 |
| `101008` | 原密码错误 |
| `101009` | 密码强度不足 |

> 修改密码后，当前 Token 会被吊销，需要重新登录。

---

## 7. 退出登录

当前登录管理员退出登录，注销访问令牌并将刷新令牌加入黑名单。

```
POST /admin/v1/auth/logout
```

### 认证级别

认证接口（需 Bearer Token）

### 请求头

| Header | 必填 | 说明 |
|--------|------|------|
| `Authorization` | 是 | `Bearer <accessToken>` |
| `X-Refresh-Token` | 是 | 当前用户的 refresh token。服务端会将其加入黑名单（TTL = 剩余有效期），避免登出后 refresh token 仍可换发新 access token。 |

> **强制要求**：`X-Refresh-Token` 为必填项。缺失或为空字符串时接口返回 `100001`（缺少刷新令牌），拒绝执行登出。这确保登出操作能完整失效 refresh token，避免登出后 refresh token 仍可换发新 access token 的安全风险。

### 请求示例

```
POST /admin/v1/auth/logout
Authorization: Bearer <token>
X-Refresh-Token: <refreshToken>
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "退出登录成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `100001` | 参数错误（缺少 `X-Refresh-Token` header 或为空，msg 为「缺少刷新令牌」） |
| `100002` | 未授权 |

> 退出登录后：
> - 当前 access token 会从 TokenStore 中移除，立即失效。
> - refresh token 会写入黑名单（TTL = 剩余有效期），后续刷新令牌请求会被拒绝。
