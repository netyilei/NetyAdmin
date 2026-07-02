# 错误码参考表

> 本文档列出 NetyAdmin 客户端 API 可能返回的所有错误码及其说明。所有接口统一返回 HTTP 200，通过响应体中的 `code` 字段区分业务状态。

---

## 一、响应格式

```json
{
  "code": "100001",
  "msg": "参数错误",
  "request_id": "req_01HXYZ..."
}
```

- 成功时 `code` 为 `"100000"`，`msg` 为空字符串
- 失败时 `code` 为对应错误码，`msg` 为错误描述，`data` 字段不返回

---

## 二、通用错误码（100000-100010）

| code | 常量名 | 说明 |
|------|--------|------|
| `100000` | CodeSuccess | 操作成功 |
| `100001` | CodeInvalidParams | 参数错误 |
| `100002` | CodeUnauthorized | 未授权 |
| `100003` | CodeForbidden | 无权限 |
| `100004` | CodeNotFound | 资源不存在 |
| `100005` | CodeInternalError | 服务器内部错误 |
| `100006` | CodeTooManyRequest | 请求过于频繁 |
| `100007` | CodeBadRequest | 请求错误 |
| `100008` | CodeAlreadyExists | 资源已存在 |
| `100009` | CodeCaptchaInvalid | 验证码错误 |
| `100010` | CodeCaptchaRequired | 验证码必填 |

---

## 三、用户模块错误码（101001-101009）

| code | 常量名 | 说明 |
|------|--------|------|
| `101001` | CodeUserNotFound | 用户不存在 |
| `101002` | CodeUserDisabled | 用户已禁用 |
| `101003` | CodePasswordWrong | 密码错误 |
| `101004` | CodeUserAlreadyExists | 用户名已存在 |
| `101005` | CodeTokenExpired | 令牌已过期 |
| `101006` | CodeTokenInvalid | 令牌无效 |
| `101007` | CodeUserLocked | 账户已锁定 |
| `101008` | CodeOldPasswordWrong | 原密码错误 |
| `101009` | CodePasswordTooWeak | 密码强度不足 |

---

## 四、客户端用户错误码（101101-101104）

| code | 常量名 | 说明 |
|------|--------|------|
| `101101` | CodeClientUserNotFound | 用户不存在 |
| `101102` | CodeClientUserAlreadyExists | 用户名已存在 |
| `101103` | CodeEmailAlreadyExists | 邮箱已存在 |
| `101104` | CodePhoneAlreadyExists | 手机号已存在 |

---

## 五、消息模块错误码（101201-101205）

| code | 常量名 | 说明 |
|------|--------|------|
| `101201` | CodeMsgTemplateNotFound | 消息模板不存在 |
| `101202` | CodeMsgTemplateCodeExists | 模板编码已存在 |
| `101203` | CodeMsgSendFailed | 消息发送失败 |
| `101204` | CodeMsgRecordNotFound | 消息记录不存在 |
| `101205` | CodeMsgDriverNotFound | 消息驱动未配置或不存在 |

---

## 六、开放平台错误码（101301-101306）

| code | 常量名 | 说明 |
|------|--------|------|
| `101301` | CodeAppKeyInvalid | AppKey 无效 |
| `101302` | CodeSignatureFailed | 签名验证失败（含 Nonce 重复） |
| `101303` | CodeRequestExpired | 请求已过期（时间戳超出 ±60s） |
| `101304` | CodeScopeMismatch | 权限不足（Scope 不匹配） |
| `101305` | CodeRateLimited | 已触发流量限制 |
| `101306` | CodeAppDisabled | 应用已被禁用 |

---

## 七、IP 访问控制错误码（101401-101403）

| code | 常量名 | 说明 |
|------|--------|------|
| `101401` | CodeIPBlocked | 访问受限（您的 IP 已被封锁或校验服务异常） |
| `101402` | CodeIPInvalid | 非法 IP/CIDR 格式 |
| `101403` | CodeWhitelistMode | 系统处于白名单模式，您的 IP 未被授权 |

---

## 八、存储上传错误码（101501-101505）

| code | 常量名 | 说明 |
|------|--------|------|
| `101501` | CodeUploadRecordNotFound | 上传记录不存在 |
| `101502` | CodeUploadSignatureInvalid | 上传凭证校验失败（recordId + secret 不匹配） |
| `101503` | CodeUploadRecordCompleted | 该上传记录已完成，不可重复提交 |
| `101504` | CodeUploadRecordExpired | 上传凭证已过期 |
| `101505` | CodeUploadRecordMismatch | 上传记录与请求不匹配 |

---

## 九、验证码错误码（200601-200605）

| code | 常量名 | 说明 |
|------|--------|------|
| `200601` | CodeCaptchaExpired | 验证码已过期 |
| `200604` | CodeCaptchaSendTooFrequent | 发送过于频繁，请稍后再试 |
| `200605` | CodeVerifyTypeNotConfigured | 未配置验证方式，请联系管理员 |

> **说明**：`200602` 和 `200603` 为预留码位，当前未使用。

---

## 十、错误码分类速查

| 错误码范围 | 模块 | 常见场景 |
|-----------|------|----------|
| 100000-100010 | 通用 | 参数校验、鉴权、资源查找、限流 |
| 101001-101009 | 用户 | 登录、注册、改密、Token |
| 101101-101104 | 客户端用户 | 注册唯一性校验 |
| 101201-101205 | 消息 | 发送验证码、站内信查询 |
| 101301-101306 | 开放平台 | 签名、权限、应用状态 |
| 101401-101403 | IP 访问控制 | IP 封禁、白名单 |
| 101501-101505 | 存储上传 | 上传凭证校验、记录状态 |
| 200601-200605 | 验证码 | 过期、频率限制、配置缺失 |

---

## 十一、常见错误排查

### 签名验证失败 (101302)

1. 检查 AppSecret 是否正确（是否使用了错误的密钥）
2. 检查 StringToSign 的拼接顺序：`Method\nPath\nTimestamp\nNonce\nPayload`
3. GET 请求：确认 Query 参数按 key 字典序排列
4. POST 请求：确认 Payload 为 Body 的 SHA256 十六进制小写
5. 确认时间戳为秒级 Unix 时间戳

### 请求已过期 (101303)

- 服务器时间与客户端时间偏差超过 60 秒，请同步系统时间

### 权限不足 (101304)

- 应用未授权该 API 的访问权限，需在管理后台「应用管理」中配置 API 权限

### 未授权 (100002)

- 需要用户 JWT 的接口未携带 `Authorization: Bearer <token>` 或 Token 已失效
