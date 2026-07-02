# 认证与验证码 API

> 本文档包含图形验证码、场景验证配置、发送短信/邮箱验证码相关的接口。登录、注册、找回密码等用户操作接口见 [02-user.md](./02-user.md)。

---

## 一、接口总览

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | /client/v1/auth/captcha | 签名 | 获取图形验证码 |
| GET | /client/v1/auth/scene-config | 签名 | 获取场景验证配置 |
| POST | /client/v1/auth/send-code | 签名 | 发送短信/邮箱验证码 |

---

## 二、流程总览

### 2.1 登录流程

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

### 2.2 注册流程

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

### 2.3 找回密码流程

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

> 步骤 4 的接口详情见 [02-user.md](./02-user.md)。

---

## 三、获取图形验证码

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
  "msg": "",
  "data": {
    "captchaId": "captcha_01HXYZ...",
    "img": "data:image/png;base64,iVBORw0KGgo..."
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| captchaId | string | 验证码 ID，后续提交时需携带 |
| img | string | Base64 编码的验证码图片（可直接用于 `<img>` 标签的 src） |

**验证码规格**：数字类型，4 位字符，图片尺寸 120x40px。

**可能错误码**：

| code | 说明 |
|------|------|
| `100005` | 验证码生成失败（服务器内部错误） |

---

## 四、获取场景验证配置

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
  "msg": "",
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

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | scene 不能为空 / 不支持的场景 |

---

## 五、发送短信/邮箱验证码

向手机或邮箱发送 6 位数字验证码。

- **登录场景**：客户端只需提供 `username`，后端根据 `verifyType` 配置自动查找用户绑定的邮箱或手机号发送验证码。
- **注册/找回密码场景**：客户端需提供 `target`（手机号或邮箱地址）。

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

> **安全说明**：图形验证码校验在发送验证码时完成，短信/邮箱验证码校验在最终提交接口（login/register/reset-password）中完成。这确保用户无法通过直接调用提交接口绕过验证码验证。

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
  "msg": "",
  "data": null
}
```

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败（scene 必填 / 登录场景缺 username / 注册场景缺 target / 当前场景未启用消息验证 / 该用户未绑定邮箱或手机号） |
| `100009` | 图形验证码错误 |
| `101001` | 用户不存在（登录场景 username 无效时） |
| `101002` | 用户已禁用 |
| `200601` | 验证码已过期 |
| `200604` | 发送过于频繁，请稍后再试 |
| `200605` | 未配置验证方式 / 当前场景未启用消息验证 |
| `101203` | 消息发送失败 |
| `101205` | 消息驱动未配置 |

> **频率限制**：同一目标 60 秒内只能发送一次。验证码有效期为 10 分钟。
