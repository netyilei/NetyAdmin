# 认证与验证码 API

> 本文档包含验证码、登录、注册、找回密码相关的所有接口。登录/注册/找回密码流程受系统配置控制，可能需要先获取图形验证码或短信/邮箱验证码。

---

## 一、流程总览

### 1.1 登录流程

```
┌─────────────────────────────────────────────────────────┐
│ 1. GET /client/v1/auth/scene-config?scene=login         │
│    → 一次请求获取：captchaEnabled + verifyEnabled + 类型  │
├─────────────────────────────────────────────────────────┤
│ 2. [若 captchaEnabled=true]                             │
│    GET /client/v1/auth/captcha                          │
│    → 获取图形验证码 (captchaId + 图片)                    │
├─────────────────────────────────────────────────────────┤
│ 3. [若 verifyEnabled=true]                              │
│    POST /client/v1/auth/send-code                       │
│    → 发送短信/邮箱验证码                                  │
├─────────────────────────────────────────────────────────┤
│ 4. POST /client/v1/user/login                           │
│    → 提交用户名+密码+验证码(按需)                         │
│    → 返回 accessToken + refreshToken                    │
└─────────────────────────────────────────────────────────┘
```

### 1.2 注册流程

```
┌──────────────────────────────────────────────────────────────┐
│ 1. GET /client/v1/auth/scene-config?scene=register           │
│    → 一次请求获取：captchaEnabled + verifyEnabled + 类型      │
├──────────────────────────────────────────────────────────────┤
│ 2. GET /client/v1/auth/captcha                               │
│    → 获取图形验证码 (用于发送验证码前的二次校验)               │
├──────────────────────────────────────────────────────────────┤
│ 3. POST /client/v1/auth/send-code                            │
│    → 发送短信/邮箱验证码 (需携带图形验证码)                   │
├──────────────────────────────────────────────────────────────┤
│ 4. POST /client/v1/user/register                             │
│    → 提交注册信息 + 验证码                                   │
└──────────────────────────────────────────────────────────────┘
```

### 1.3 找回密码流程

```
┌──────────────────────────────────────────────────────────────────┐
│ 1. GET /client/v1/auth/scene-config?scene=reset_password         │
│    → 一次请求获取：captchaEnabled + verifyEnabled + 类型          │
├──────────────────────────────────────────────────────────────────┤
│ 2. GET /client/v1/auth/captcha                                   │
│    → 获取图形验证码                                               │
├──────────────────────────────────────────────────────────────────┤
│ 3. POST /client/v1/auth/send-code                                │
│    → 发送验证码到手机/邮箱                                        │
├──────────────────────────────────────────────────────────────────┤
│ 4. POST /client/v1/user/reset-password                           │
│    → 提交新密码 + 验证码                                         │
└──────────────────────────────────────────────────────────────────┘
```

---

## 二、场景配置接口（核心）

### 2.1 获取场景验证配置

**一个请求同时返回图形验证码开关和消息验证码开关**，客户端据此决定 UI 展示和提交哪些字段。

```
GET /client/v1/auth/scene-config
```

**权限**：开放平台签名

**请求参数**（Query）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| scene | string | 是 | 业务场景：`login` / `register` / `reset_password` |

**响应示例**：

```json
{
  "code": "100000",
  "data": {
    "scene": "login",
    "captchaEnabled": true,
    "verifyEnabled": false,
    "verifyType": ""
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| scene | string | 业务场景 |
| captchaEnabled | boolean | 是否需要图形验证码 |
| verifyEnabled | boolean | 是否需要短信/邮箱验证码 |
| verifyType | string | 验证方式：`email` / `sms` / `""`（verifyEnabled=false 时为空） |

**客户端决策逻辑**：

| captchaEnabled | verifyEnabled | 客户端行为 |
|----------------|---------------|-----------|
| false | false | 仅需用户名+密码 |
| true | false | 需用户名+密码+图形验证码 |
| false | true | 需用户名+密码+短信/邮箱验证码 |
| true | true | 需用户名+密码+图形验证码+短信/邮箱验证码 |

**可能错误码**：`100001`（scene 不能为空 / 不支持的场景）

---

## 三、图形验证码接口

### 3.1 获取图形验证码

获取一张数字图形验证码图片。

```
GET /client/v1/auth/captcha
```

**权限**：开放平台签名

**请求参数**：无

**响应示例**：

```json
{
  "code": "100000",
  "data": {
    "captchaId": "captcha_01HXYZ...",
    "img": "data:image/png;base64,iVBORw0KGgo..."
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| captchaId | string | 验证码 ID，后续提交时需携带 |
| img | string | Base64 编码的验证码图片 |

**可能错误码**：`100005`（验证码生成失败）

---

## 四、发送验证码接口

### 4.1 发送短信/邮箱验证码

向手机或邮箱发送 6 位数字验证码。

**登录场景**：客户端只需提供 `username`，后端根据 `verifyType` 配置自动查找用户绑定的邮箱或手机号发送验证码。
**注册/找回密码场景**：客户端需提供 `target`（手机号或邮箱地址）。

```
POST /client/v1/auth/send-code
```

**权限**：开放平台签名

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| scene | string | 是 | 业务场景：`login` / `register` / `reset_password` |
| username | string | 条件必填 | 用户名（scene=login 时必填，后端自动查找用户绑定的 email/phone） |
| target | string | 条件必填 | 接收目标：手机号或邮箱地址（scene=register/reset_password 时必填） |
| captchaKey | string | 条件必填 | 图形验证码 ID（captchaEnabled=true 时必填） |
| captchaCode | string | 条件必填 | 图形验证码值（captchaEnabled=true 时必填） |

> **安全说明**：验证码校验在最终提交接口（login/register/reset-password）中完成，而非在 send-code 中完成。这确保用户无法通过直接调用提交接口绕过验证码验证。

**请求示例（登录场景）**：

```json
{
  "username": "testuser",
  "scene": "login",
  "captchaKey": "captcha_01HXYZ...",
  "captchaCode": "1234"
}
```

**请求示例（注册/找回密码场景）**：

```json
{
  "target": "user@example.com",
  "scene": "register",
  "captchaKey": "captcha_01HXYZ...",
  "captchaCode": "1234"
}
```

**响应示例**：

```json
{
  "code": "100000",
  "data": null
}
```

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败 |
| `100009` | 图形验证码错误 |
| `101001` | 用户不存在（登录场景 username 无效时） |
| `101002` | 用户已禁用 |
| `101003` | 该用户未绑定邮箱/手机号 |
| `200601` | 验证码已过期 |
| `200604` | 发送过于频繁，请稍后再试 |
| `200605` | 未配置验证方式 / 当前场景未启用消息验证 |
| `101203` | 消息发送失败 |
| `101205` | 消息驱动未配置 |

> **频率限制**：同一目标 60 秒内只能发送一次。验证码有效期为 10 分钟。

---

## 五、用户登录

```
POST /client/v1/user/login
```

**权限**：开放平台签名

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | 用户名 |
| password | string | 是 | 密码 |
| platform | string | 否 | 登录平台标识：`web` / `app` / `mini_program` 等 |
| captchaKey | string | 条件必填 | 图形验证码 ID（captchaEnabled=true 时必填） |
| captchaCode | string | 条件必填 | 图形验证码值（captchaEnabled=true 时必填） |

> **注意**：登录场景下如果 `verifyEnabled=true`，则还需额外字段 `code`（短信/邮箱验证码）。具体见下方请求示例。

| code | string | 条件必填 | 短信/邮箱验证码（verifyEnabled=true 时必填） |

**请求示例（仅用户名+密码）**：

```json
{
  "username": "testuser",
  "password": "MyPassword123",
  "platform": "web"
}
```

**请求示例（需要图形验证码）**：

```json
{
  "username": "testuser",
  "password": "MyPassword123",
  "platform": "web",
  "captchaKey": "captcha_01HXYZ...",
  "captchaCode": "1234"
}
```

**请求示例（需要图形验证码 + 消息验证码）**：

```json
{
  "username": "testuser",
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

---

## 六、用户注册

```
POST /client/v1/user/register
```

**权限**：开放平台签名

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | 用户名，4-20 位 |
| password | string | 是 | 密码，6-20 位（受系统配置约束强度） |
| nickname | string | 是 | 昵称 |
| phone | string | 条件必填 | 手机号（与 email 至少填一个） |
| email | string | 条件必填 | 邮箱（与 phone 至少填一个） |
| code | string | 条件必填 | 短信/邮箱验证码（verifyEnabled=true 时必填） |

**请求示例**：

```json
{
  "username": "newuser",
  "password": "MyPassword123",
  "nickname": "新用户",
  "email": "user@example.com",
  "code": "123456"
}
```

**响应示例**：

```json
{
  "code": "100000",
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
| `101103` | 邮箱已存在 |
| `101104` | 手机号已存在 |

---

## 七、找回密码

```
POST /client/v1/user/reset-password
```

**权限**：开放平台签名

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| target | string | 是 | 手机号或邮箱地址 |
| code | string | 是 | 短信/邮箱验证码 |
| newPassword | string | 是 | 新密码，6-20 位 |

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
  "data": null
}
```

> **注意**：找回密码成功后，该用户的所有在线会话将被强制下线（Token 全部失效），同时解除登录锁定状态。

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败 |
| `100009` | 验证码错误或已过期 |
| `100010` | 验证码必填 |
| `101001` | 用户不存在 |
| `101002` | 账户已禁用，无法找回密码 |

---

## 八、刷新令牌

```
POST /client/v1/user/refresh-token
```

**权限**：开放平台签名

**请求参数**（Query）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| refreshToken | string | 是 | 刷新令牌 |

**响应示例**：

```json
{
  "code": "100000",
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIs...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIs...",
    "expiresIn": 7200
  }
}
```

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 缺少刷新令牌 |
| `100002` | 刷新令牌无效 |
| `101001` | 用户不存在 |
| `101002` | 用户已禁用 |
