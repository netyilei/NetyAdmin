# 错误码参考

本文档列出 NetyAdmin GM Admin API 的全部业务错误码。所有错误码均来源于 `server/internal/pkg/errorx/errorx.go`。

## 响应格式

所有接口返回统一的 JSON 响应结构，`code` 字段为字符串类型的业务状态码：

```json
{
  "code": "100000",
  "msg": "操作成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

- `code` 为 `100000` 时表示请求成功
- `code` 为其他值时表示业务错误，`msg` 字段携带错误描述

---

## 一、通用错误码（100000-100010）

| 错误码 | 常量名 | 描述 |
|--------|--------|------|
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

## 二、管理员认证模块（101001-101009）

| 错误码 | 常量名 | 描述 |
|--------|--------|------|
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

## 三、角色管理模块（102001-102006）

| 错误码 | 常量名 | 描述 |
|--------|--------|------|
| `102001` | CodeRoleNotFound | 角色不存在 |
| `102002` | CodeRoleInUse | 角色正在使用中 |
| `102003` | CodeRoleAlreadyExists | 角色已存在 |
| `102004` | CodeRoleCodeDuplicate | 角色编码重复 |
| `102005` | CodeCannotDeleteSuper | 不能删除超级管理员 |
| `102006` | CodeCannotModifySuper | 不能修改超级管理员 |

---

## 四、菜单管理模块（103001-103004）

| 错误码 | 常量名 | 描述 |
|--------|--------|------|
| `103001` | CodeMenuNotFound | 菜单不存在 |
| `103002` | CodeMenuHasChildren | 菜单存在子菜单 |
| `103003` | CodeMenuAlreadyExists | 菜单已存在 |
| `103004` | CodeMenuRouteDuplicate | 菜单路由重复 |

---

## 五、按钮管理模块（104001-104003）

| 错误码 | 常量名 | 描述 |
|--------|--------|------|
| `104001` | CodeButtonNotFound | 按钮不存在 |
| `104002` | CodeButtonAlreadyExists | 按钮已存在 |
| `104003` | CodeButtonCodeDuplicate | 按钮编码重复 |

---

## 六、API 管理模块（105001-105003）

| 错误码 | 常量名 | 描述 |
|--------|--------|------|
| `105001` | CodeApiNotFound | API 不存在 |
| `105002` | CodeApiAlreadyExists | API 已存在 |
| `105003` | CodeApiPathDuplicate | API 路径重复 |

---

## 七、客户端用户模块（101101-101104）

| 错误码 | 常量名 | 描述 |
|--------|--------|------|
| `101101` | CodeClientUserNotFound | 用户不存在 |
| `101102` | CodeClientUserAlreadyExists | 用户名已存在 |
| `101103` | CodeEmailAlreadyExists | 邮箱已存在 |
| `101104` | CodePhoneAlreadyExists | 手机号已存在 |

---

## 八、消息模块（101201-101205）

| 错误码 | 常量名 | 描述 |
|--------|--------|------|
| `101201` | CodeMsgTemplateNotFound | 消息模板不存在 |
| `101202` | CodeMsgTemplateCodeExists | 模板编码已存在 |
| `101203` | CodeMsgSendFailed | 消息发送失败 |
| `101204` | CodeMsgRecordNotFound | 消息记录不存在 |
| `101205` | CodeMsgDriverNotFound | 消息驱动未配置或不存在 |

---

## 九、开放平台模块（101301-101306）

| 错误码 | 常量名 | 描述 |
|--------|--------|------|
| `101301` | CodeAppKeyInvalid | AppKey 无效 |
| `101302` | CodeSignatureFailed | 签名验证失败 |
| `101303` | CodeRequestExpired | 请求已过期 |
| `101304` | CodeScopeMismatch | 权限不足（Scope 不匹配） |
| `101305` | CodeRateLimited | 已触发流量限制 |
| `101306` | CodeAppDisabled | 应用已被禁用 |

---

## 十、IP 访问控制模块（101401-101403）

| 错误码 | 常量名 | 描述 |
|--------|--------|------|
| `101401` | CodeIPBlocked | 访问受限（您的 IP 已被封锁） |
| `101402` | CodeIPInvalid | 非法 IP/CIDR 格式 |
| `101403` | CodeWhitelistMode | 系统处于白名单模式，您的 IP 未被授权 |

---

## 十一、存储上传模块（101501-101505）

| 错误码 | 常量名 | 描述 |
|--------|--------|------|
| `101501` | CodeUploadRecordNotFound | 上传记录不存在 |
| `101502` | CodeUploadSignatureInvalid | 上传凭证校验失败 |
| `101503` | CodeUploadRecordCompleted | 该上传记录已完成，不可重复提交 |
| `101504` | CodeUploadRecordExpired | 上传凭证已过期 |
| `101505` | CodeUploadRecordMismatch | 上传记录与请求不匹配 |

---

## 十二、任务调度与系统配置模块（109005-109008）

| 错误码 | 常量名 | 描述 |
|--------|--------|------|
| `109005` | CodeTaskNotFound | 任务不存在 |
| `109006` | CodeTaskAlreadyRunning | 任务已在运行中 |
| `109007` | CodeTaskNotRunning | 任务未运行 |
| `109008` | CodeEmailTestFailed | 邮件测试发送失败 |

---

## 十三、验证码与用户集成模块（200601-200605）

| 错误码 | 常量名 | 描述 |
|--------|--------|------|
| `200601` | CodeCaptchaExpired | 验证码已过期 |
| `200604` | CodeCaptchaSendTooFrequent | 发送过于频繁，请稍后再试 |
| `200605` | CodeVerifyTypeNotConfigured | 未配置验证方式，请联系管理员 |

---

## 错误码速查表

按模块分类的快速检索表：

| 模块 | 错误码范围 | 文档 |
|------|------------|------|
| 通用 | 100000-100010 | 所有接口 |
| 管理员认证 | 101001-101009 | [01-auth.md](01-auth.md)、[02-admin.md](02-admin.md) |
| 角色管理 | 102001-102006 | [03-system-rbac.md](03-system-rbac.md) |
| 菜单管理 | 103001-103004 | [03-system-rbac.md](03-system-rbac.md) |
| 按钮管理 | 104001-104003 | [03-system-rbac.md](03-system-rbac.md) |
| API 管理 | 105001-105003 | [03-system-rbac.md](03-system-rbac.md) |
| 客户端用户 | 101101-101104 | 用户管理 |
| 消息模块 | 101201-101205 | [09-message.md](09-message.md) |
| 开放平台 | 101301-101306 | [08-open-platform.md](08-open-platform.md) |
| IP 访问控制 | 101401-101403 | [07-ops.md](07-ops.md) |
| 存储上传 | 101501-101505 | [06-storage.md](06-storage.md) |
| 任务调度/配置 | 109005-109008 | [10-task.md](10-task.md)、[12-config.md](12-config.md) |
| 验证码集成 | 200601-200605 | [13-common.md](13-common.md) |
