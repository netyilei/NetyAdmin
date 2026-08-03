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
| platform | string | **是** | 登录平台标识，用于多端会话隔离（见下方「多端登录说明」）。取值由客户端自定义，如 `web` / `mobile` / `miniapp` |
| captchaKey | string | 条件必填 | 图形验证码 ID（captchaEnabled=true 时必填） |
| captchaCode | string | 条件必填 | 图形验证码值（captchaEnabled=true 时必填） |
| code | string | 条件必填 | 短信/邮箱验证码（verifyEnabled=true 时必填） |

> **多端登录说明**：`platform` 字段控制会话隔离粒度：
> - **同 platform 再次登录** → 顶掉该 platform 的旧会话（旧 token 立即失效）
> - **不同 platform** → 各自独立会话，互不影响（如 web + mobile 可同时在线）
> - 管理员敏感操作（改密/禁用/删除）→ 顶掉该用户**所有 platform** 的会话
>
> 详见 [多端登录与会话管理](#多端登录与会话管理)。

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
| `100006` | 请求过于频繁（IP 维度限流，HTTP 429） |
| `100009` | 验证码错误 |
| `100010` | 验证码必填（启用了验证码但未提供） |
| `101001` | 用户不存在（msg 统一为「用户名或密码错误」） |
| `101002` | 用户已禁用（msg 统一为「用户名或密码错误」） |
| `101003` | 密码错误（msg 统一为「用户名或密码错误」） |
| `101007` | 账户已锁定（密码错误次数过多） |
| `101009` | 密码强度不足 |

> **安全说明**：
> - **IP 维度限流**（Task 3）：单 IP 在 `[login_ratelimit].window`（默认 1 分钟）内最多 `[login_ratelimit].max`（默认 10）次 Login / RefreshToken 请求，超限返回 `100006`（请求过于频繁，HTTP 429）；与 per-account 锁定独立，针对单 IP 撞库场景
> - **Login 失败文案统一**（Task 3.4）：为消除用户名枚举，`101001` / `101002` / `101003` 在 Login 路径统一返回 msg「用户名或密码错误」，前端不应据 msg 区分用户不存在 / 已禁用 / 密码错误

---

## 四、刷新令牌

`accessToken` 过期后，使用 `refreshToken` 获取新的 Token 对。

```
POST /client/v1/user/refresh-token
```

**权限**：开放平台签名

> **⚠️ BREAKING 变更（2026-07-05）**：
> 此接口从 URL query `?refreshToken=xxx` 改为 JSON body `{"refreshToken":"xxx"}` 传递。
> 旧版前端需同步调整，避免 refresh token 泄露到 access log / URL / 浏览器历史。

**请求头**：

| Header | 必填 | 说明 |
|--------|------|------|
| `Content-Type` | 是 | `application/json` |

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| refreshToken | string | 是 | 刷新令牌 |

**请求示例**：

```http
POST /client/v1/user/refresh-token
Content-Type: application/json

{
  "refreshToken": "eyJhbGciOiJIUzI1NiIs..."
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
| accessToken | string | 新的访问令牌 |
| refreshToken | string | 新的刷新令牌 |
| expiresIn | int64 | accessToken 剩余有效时间（秒） |

> **安全说明**：每次刷新令牌后，旧的 RefreshToken 会被加入黑名单（缓存 TTL 等于 Token 剩余有效期），不可再次使用。客户端必须在刷新成功后立即替换本地存储的 RefreshToken。
>
> **多端会话校验**：刷新时会校验 refresh token 携带的端级版本号（`ptv`）与 DB `user_tokens` 当前版本：
> - 同 platform 重新登录后，旧 refresh token 的 `ptv` < DB 当前版本 → 拒绝刷新（顶号生效）
> - 刷新不递增版本号（会话延续语义），仅更新当前 platform 的 access hash

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 缺少刷新令牌（body 未提供 `refreshToken` 或为空字符串） |
| `101005` | 令牌已过期 |
| `101006` | 令牌无效 |
| `101002` | 该设备已有新登录（同 platform 被顶号），需重新登录 |

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
| email | string | 否 | 邮箱（格式校验，最大 100 字符；修改时需配合 `emailCode`） |
| phone | string | 否 | 手机号（修改时需配合 `phoneCode`） |
| emailCode | string | 条件必填 | 邮箱验证码（修改 `email` 时必填，需先调 `POST /client/v1/auth/send-code` 发送邮箱验证码） |
| phoneCode | string | 条件必填 | 手机验证码（修改 `phone` 时必填，需先调 `POST /client/v1/auth/send-code` 发送短信验证码） |

> **安全说明**：修改 `email` / `phone` 必须提供对应渠道的验证码，确保新联系方式归属当前用户。仅修改 `nickName` / `avatar` / `gender` 无需验证码。

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

使当前 access token 失效，并将 refresh token 加入黑名单，退出登录状态。

```
POST /client/v1/user/logout
```

**权限**：开放平台签名 + 用户 JWT

**请求头**：

| Header | 必填 | 说明 |
|--------|------|------|
| `Authorization` | 是 | `Bearer <accessToken>` |
| `X-Refresh-Token` | 是 | 当前用户的 refresh token。服务端会将其加入黑名单（TTL = 剩余有效期），避免登出后 refresh token 仍可换发新 access token。 |

> **强制要求**：`X-Refresh-Token` 为必填项。缺失或为空字符串时接口返回 `100001`（缺少刷新令牌），拒绝执行登出。这确保登出操作能完整失效 refresh token，避免登出后 refresh token 仍可换发新 access token 的安全风险。

**请求参数**：无

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": null
}
```

> **说明**：退出登录后：
> - 当前 platform 的会话立即失效（清空 user_tokens 该 platform 行的 hash）。
> - refresh token 会写入黑名单（TTL = 剩余有效期），后续刷新令牌请求会被拒绝。
> - **仅影响当前 platform**：如用户同时在 web 和 mobile 登录，web 端 Logout 不会影响 mobile 端会话。

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数错误（缺少 `X-Refresh-Token` header 或为空，msg 为「缺少刷新令牌」） |
| `100002` | 未授权 |

---

## 多端登录与会话管理

NetyAdmin 客户端支持同一账号在多个平台（web / mobile / miniapp 等）同时登录，每个平台维护独立的会话。

### 会话模型

| 维度 | 存储表 | 说明 |
|------|--------|------|
| 客户端（C 端）多端会话 | `user_tokens` | 按 `(user_id, platform)` 唯一，每个 platform 一行，存 token_version + access/refresh hash |
| 管理端（admin）会话 | `admin_tokens` | admin 专用，与客户端会话隔离 |

### platform 字段

`platform` 由客户端自定义传入（Login 请求必填），基座不限制枚举值。常见取值：

| platform | 含义 | 顶号行为 |
|----------|------|---------|
| `web` | Web 浏览器 | 同浏览器再次登录顶掉旧 web 会话 |
| `mobile` | 移动 App | 同 App 再次登录顶掉旧 mobile 会话 |
| `miniapp` | 小程序 | 同小程序再次登录顶掉旧 miniapp 会话 |

> 自定义 platform 同样适用：基座按 `(user_id, platform)` 字符串维度隔离，任意取值都会独立管理。

### 顶号语义

| 场景 | 触发动作 | 影响 |
|------|---------|------|
| **同 platform 重新登录** | Login 递增该 platform 的 `token_version` | 旧 token 立即失效（携带的 `ptv` < DB 当前版本）→ 下次请求被拒 |
| **不同 platform 登录** | 各自独立行，互不干扰 | web 与 mobile 可同时在线 |
| **管理员改密/禁用/删除** | 递增 `users.token_version`（用户级） | 该用户**所有 platform** 的会话全部失效 |
| **Logout** | 清空当前 platform 的 hash | 仅当前 platform 失效，其他 platform 不受影响 |

### Token 校验链

每次携带 access token 的请求，中间件执行双重校验：

1. **用户级版本校验**：`claims.tv < users.token_version` → 拒绝（管理员操作触发全端失效）
2. **端级版本校验**：`claims.ptv < user_tokens.token_version` → 拒绝（同 platform 顶号）
3. **hash 校验**：`access_hash` 不匹配 → 拒绝（Logout 后纵深防御）

> 校验走 L2 Redis 缓存（key 按 `user_id:platform` 维度），DB QPS 降低 30x+；缓存变更（Login/Refresh/Logout）实时失效，TTL 30s 兜底。

### JWT Claims 字段

| 字段 | JSON key | 说明 |
|------|----------|------|
| UID | `uid` | 用户 ID |
| Platform | `platform` | 登录平台 |
| TokenVersion | `tv` | 用户级版本号（管理员操作递增） |
| PlatTokenVersion | `ptv` | 端级版本号（Login 递增，顶号依据） |

### 相关接口

| 接口 | 多端行为 |
|------|---------|
| `POST /client/v1/user/login` | 必填 `platform`，递增该 platform 版本号 |
| `POST /client/v1/user/refresh-token` | 校验 `ptv`，续签不递增版本（会话延续） |
| `POST /client/v1/user/logout` | 仅清当前 platform 的会话 |
| `PUT /admin/v1/systemManage/users/:id`（改密） | 递增用户级版本号，顶掉所有 platform |
| `PATCH /admin/v1/systemManage/users/:id/status`（禁用） | 同上 |
