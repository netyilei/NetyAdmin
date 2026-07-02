# 认证管理 API

本模块提供管理员登录、令牌刷新、个人信息管理、密码修改及退出登录等接口。

## 接口概览

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| POST | `/admin/v1/auth/login` | 公开 | 管理员登录 |
| POST | `/admin/v1/auth/refreshToken` | 公开 | 刷新令牌 |
| GET | `/admin/v1/auth/getUserInfo` | 认证 | 获取当前登录用户信息 |
| GET | `/admin/v1/auth/profile` | 认证 | 获取个人资料 |
| PUT | `/admin/v1/auth/profile` | 认证 | 更新个人资料 |
| POST | `/admin/v1/auth/changePassword` | 认证 | 修改密码 |
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
| `token` | string | 访问令牌（AccessToken），有效期 168 小时 |
| `refreshToken` | string | 刷新令牌（RefreshToken），有效期 336 小时 |

### 安全机制

- 连续 **5 次**密码错误将锁定账户 **15 分钟**
- 锁定期间登录直接返回 `101007`（账户已锁定）
- 登录成功后清除失败计数并更新最后登录时间

### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `100001` | 参数错误（用户名或密码为空） |
| `100009` | 验证码错误 |
| `100010` | 验证码必填 |
| `101001` | 用户不存在 |
| `101002` | 用户已禁用 |
| `101003` | 密码错误 |
| `101007` | 账户已锁定 |

---

## 2. 刷新令牌

使用刷新令牌换取新的访问令牌与刷新令牌。

```
POST /admin/v1/auth/refreshToken
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
GET /admin/v1/auth/getUserInfo
```

### 认证级别

认证接口（需 Bearer Token）

### 请求示例

```
GET /admin/v1/auth/getUserInfo
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
| `phone` | string | 否 | 手机号 |
| `email` | string | 否 | 邮箱（需符合邮箱格式，最大 100 字符） |
| `gender` | string | 否 | 性别（`0`:未知 `1`:男 `2`:女） |

### 请求示例

```json
{
  "nickname": "新昵称",
  "phone": "13900139000",
  "email": "newadmin@example.com",
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
POST /admin/v1/auth/changePassword
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

当前登录管理员退出登录，注销访问令牌。

```
POST /admin/v1/auth/logout
```

### 认证级别

认证接口（需 Bearer Token）

### 请求示例

```
POST /admin/v1/auth/logout
Authorization: Bearer <token>
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

> 退出登录后，当前 Token 会从 TokenStore 中移除，立即失效。
