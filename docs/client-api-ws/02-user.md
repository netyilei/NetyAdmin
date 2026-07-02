# 用户中心 API

> 本文档包含用户注册、登录、找回密码、刷新令牌、个人资料、修改密码、注销账号、上传凭证等接口。验证码相关接口见 [01-auth.md](./01-auth.md)。

---

## 一、接口总览

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | /client/v1/user/register | 签名 | 用户注册 |
| POST | /client/v1/user/login | 签名 | 用户登录 |
| POST | /client/v1/user/refresh-token | 签名 | 刷新令牌 |
| POST | /client/v1/user/reset-password | 签名 | 找回密码 |
| GET | /client/v1/user/profile | 签名 + JWT | 获取个人资料 |
| PUT | /client/v1/user/profile | 签名 + JWT | 更新个人资料 |
| PUT | /client/v1/user/password | 签名 + JWT | 修改密码 |
| DELETE | /client/v1/user/account | 签名 + JWT | 注销账号 |
| GET | /client/v1/user/upload-token | 签名 + JWT | 获取上传凭证 |
| POST | /client/v1/user/upload-record | 签名 + JWT | 记录上传结果 |
| POST | /client/v1/user/logout | 签名 + JWT | 退出登录 |

---

## 二、用户注册

```
POST /client/v1/user/register
```

**权限**：开放平台签名

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| userName | string | 是 | 用户名，4-20 位 |
| password | string | 是 | 密码，8-20 位（受系统配置约束强度） |
| nickName | string | 是 | 昵称 |
| phone | string | 否 | 手机号 |
| email | string | 否 | 邮箱（格式校验，最大 100 字符） |
| code | string | 条件必填 | 短信/邮箱验证码（verifyEnabled=true 时必填） |

**请求示例**：

```json
{
  "userName": "newuser",
  "password": "MyPassword123",
  "nickName": "新用户",
  "email": "user@example.com",
  "code": "123456"
}
```

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "id": "01HXYZ1234567890ABCDEFG"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 新创建的用户 ID (ULID) |

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败 |
| `100009` | 验证码错误或已过期 |
| `100010` | 验证码必填 |
| `101004` | 用户名已存在 |
| `101009` | 密码强度不足 |
| `101103` | 邮箱已存在 |
| `101104` | 手机号已存在 |

---

## 三、用户登录

```
POST /client/v1/user/login
```

**权限**：开放平台签名

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| userName | string | 是 | 用户名 |
| password | string | 是 | 密码 |
| platform | string | 否 | 登录平台标识：`web` / `app` / `mini_program` 等 |
| captchaKey | string | 条件必填 | 图形验证码 ID（captchaEnabled=true 时必填） |
| captchaCode | string | 条件必填 | 图形验证码值（captchaEnabled=true 时必填） |
| code | string | 条件必填 | 短信/邮箱验证码（verifyEnabled=true 时必填） |

> **提示**：登录前先调用 `GET /client/v1/auth/scene-config?scene=login` 获取当前场景的验证配置，据此决定提交哪些字段。

**请求示例（仅用户名+密码）**：

```json
{
  "userName": "testuser",
  "password": "MyPassword123",
  "platform": "web"
}
```

**请求示例（需要图形验证码 + 消息验证码）**：

```json
{
  "userName": "testuser",
  "password": "MyPassword123",
  "platform": "web",
  "captchaKey": "captcha_01HXYZ...",
  "captchaCode": "1234",
  "code": "654321"
}
```

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIs...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIs...",
    "expiresIn": 7200
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| accessToken | string | 访问令牌，用于后续接口鉴权 |
| refreshToken | string | 刷新令牌，用于续期 accessToken |
| expiresIn | int64 | accessToken 剩余有效时间（秒） |

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败 |
| `100009` | 验证码错误 |
| `100010` | 验证码必填（启用了验证码但未提供） |
| `101001` | 用户不存在 |
| `101002` | 用户已禁用 |
| `101003` | 密码错误 |
| `101007` | 账户已锁定（密码错误次数过多） |
| `101009` | 密码强度不足 |

---

## 四、刷新令牌

`accessToken` 过期后，使用 `refreshToken` 获取新的 Token 对。

```
POST /client/v1/user/refresh-token
```

**权限**：开放平台签名

**请求参数**（Query）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| refreshToken | string | 是 | 刷新令牌 |

**请求示例**：

```
POST /client/v1/user/refresh-token?refreshToken=eyJhbGciOiJIUzI1NiIs...
```

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIs...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIs...",
    "expiresIn": 7200
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| accessToken | string | 新的访问令牌 |
| refreshToken | string | 新的刷新令牌 |
| expiresIn | int64 | accessToken 剩余有效时间（秒） |

> **安全说明**：每次刷新令牌后，旧的 RefreshToken 会被加入黑名单（缓存 TTL 等于 Token 剩余有效期），不可再次使用。客户端必须在刷新成功后立即替换本地存储的 RefreshToken。

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 缺少刷新令牌 |
| `101005` | 令牌已过期 |
| `101006` | 令牌无效 |

---

## 五、找回密码

通过手机号或邮箱验证码重置密码。

```
POST /client/v1/user/reset-password
```

**权限**：开放平台签名

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| target | string | 是 | 手机号或邮箱地址 |
| code | string | 是 | 短信/邮箱验证码 |
| newPassword | string | 是 | 新密码，8-20 位 |

**请求示例**：

```json
{
  "target": "user@example.com",
  "code": "123456",
  "newPassword": "NewPassword456"
}
```

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": null
}
```

> **注意**：找回密码成功后，该用户的所有在线会话将被强制下线（Token 全部失效），同时解除登录锁定状态。

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败 |
| `100009` | 验证码错误或已过期 |
| `101001` | 用户不存在 |
| `101002` | 账户已禁用，无法找回密码 |
| `101009` | 密码强度不足 |

---

## 六、获取个人资料

```
GET /client/v1/user/profile
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**：无

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "id": "01HXYZ1234567890ABCDEFG",
    "userName": "testuser",
    "nickName": "测试用户",
    "avatar": "https://cdn.example.com/avatar/xxx.jpg",
    "phone": "13800001234",
    "email": "user@example.com",
    "gender": "1",
    "status": "1",
    "lastLoginAt": "2025-01-01T12:00:00Z"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 用户 ID (ULID) |
| userName | string | 用户名 |
| nickName | string | 昵称 |
| avatar | string | 头像 URL |
| phone | string | 手机号 |
| email | string | 邮箱 |
| gender | string | 性别：`0` 未知 / `1` 男 / `2` 女 |
| status | string | 状态：`1` 正常 / `0` 禁用 |
| lastLoginAt | string | 最后登录时间（ISO 8601），未登录过时不返回 |

**可能错误码**：

| code | 说明 |
|------|------|
| `100002` | 未授权（缺少或无效的 JWT Token） |

---

## 七、更新个人资料

```
PUT /client/v1/user/profile
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| nickName | string | 否 | 昵称 |
| avatar | string | 否 | 头像 URL |
| gender | string | 否 | 性别：`0` / `1` / `2` |
| email | string | 否 | 邮箱（格式校验，最大 100 字符） |
| phone | string | 否 | 手机号 |

**请求示例**：

```json
{
  "nickName": "新昵称",
  "gender": "1"
}
```

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": null
}
```

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败 |
| `100002` | 未授权 |

---

## 八、修改密码

用户已登录状态下修改密码，需提供原密码。

```
PUT /client/v1/user/password
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| oldPassword | string | 是 | 原密码 |
| newPassword | string | 是 | 新密码，8-20 位 |

**请求示例**：

```json
{
  "oldPassword": "OldPassword123",
  "newPassword": "NewPassword456"
}
```

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": null
}
```

> **注意**：修改密码成功后，该用户的所有在线会话将被强制下线（Token 全部失效），需重新登录。

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败 |
| `100002` | 未授权 |
| `101008` | 原密码错误 |
| `101009` | 密码强度不足 |

---

## 九、注销账号

永久注销当前用户账号，操作不可逆。

```
DELETE /client/v1/user/account
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**：无

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": null
}
```

> **注意**：注销后该用户的所有数据将被软删除，所有在线会话立即失效。

**可能错误码**：

| code | 说明 |
|------|------|
| `100002` | 未授权 |

---

## 十、获取上传凭证

获取文件上传的预签名 URL 和相关凭证。用于用户端直传文件到对象存储。

```
GET /client/v1/user/upload-token
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**（Query）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| fileName | string | 否 | 文件名（未传时使用时间戳兜底生成） |
| contentType | string | 否 | MIME 类型，如 `image/png` |
| businessType | string | 否 | 业务类型标识，如 `avatar` |
| businessId | string | 否 | 业务关联 ID |

**请求示例**：

```
GET /client/v1/user/upload-token?fileName=avatar.png&contentType=image/png&businessType=avatar
```

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "uploadUrl": "https://bucket.oss-cn-hangzhou.aliyuncs.com/uploads/xxx.png?Signature=...",
    "storageConfigId": 1,
    "objectKey": "uploads/2025/01/01HXYZ1234567890ABCDEFG.png",
    "finalUrl": "https://cdn.example.com/uploads/2025/01/01HXYZ1234567890ABCDEFG.png",
    "recordId": 42,
    "secret": "a1b2c3d4e5f6..."
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| uploadUrl | string | 预签名上传 URL，客户端直接向此地址上传文件 |
| storageConfigId | uint | 存储配置 ID |
| objectKey | string | 对象存储 Key |
| finalUrl | string | 文件最终访问 URL |
| recordId | uint | 上传记录 ID，**回调时必须携带** |
| secret | string | 上传校验密钥，**回调时必须携带** |

> **重要**：`recordId` 和 `secret` 是上传成功后回调 `/user/upload-record` 接口的必填参数，用于服务端校验上传合法性。请妥善保存。

**可能错误码**：

| code | 说明 |
|------|------|
| `100002` | 未授权 |
| `100005` | 获取上传凭证失败（存储配置不存在或不可用） |

---

## 十一、记录上传结果

用户直传文件到对象存储成功后，回调此接口通知服务端上传完成。与 `/user/upload-token` 配套使用。

```
POST /client/v1/user/upload-record
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| recordId | uint | 是 | 上传记录 ID（从获取凭证接口返回） |
| secret | string | 是 | 上传校验密钥（从获取凭证接口返回） |
| objectKey | string | 否 | 对象存储 Key |
| fileUrl | string | 否 | 文件访问 URL |
| fileSize | int64 | 否 | 文件大小（字节） |
| mimeType | string | 否 | MIME 类型 |
| md5 | string | 否 | 文件 MD5 哈希 |

**请求示例**：

```json
{
  "recordId": 42,
  "secret": "a1b2c3d4e5f6...",
  "objectKey": "uploads/2025/01/01HXYZ1234567890ABCDEFG.png",
  "fileUrl": "https://cdn.example.com/uploads/2025/01/01HXYZ1234567890ABCDEFG.png",
  "fileSize": 102400,
  "mimeType": "image/png",
  "md5": "d41d8cd98f00b204e9800998ecf8427e"
}
```

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": null
}
```

> **说明**：服务端会根据 `recordId` + `secret` 校验上传凭证的合法性，校验通过后将上传记录状态从 `pending` 置为 `uploaded`。

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败（recordId、secret 必填） |
| `100002` | 未授权 |
| `101501` | 上传记录不存在 |
| `101502` | 上传凭证校验失败（recordId + secret 不匹配） |
| `101503` | 该上传记录已完成，不可重复提交 |
| `101504` | 上传凭证已过期 |
| `101505` | 上传记录与请求不匹配 |

---

## 十二、退出登录

使当前 Token 失效，退出登录状态。

```
POST /client/v1/user/logout
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**：无

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": null
}
```

**可能错误码**：

| code | 说明 |
|------|------|
| `100002` | 未授权 |
