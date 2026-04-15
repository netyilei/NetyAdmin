# NetyAdmin 状态码全量文档

本文档为 NetyAdmin 系统的**状态码唯一权威来源 (Single Source of Truth)**。所有新增业务模块的状态码必须基于本文档的编码规则进行拓展，并同步更新本文档。

---

## 一、统一原则

- HTTP 统一返回结构：`{ code, msg, data, request_id }`
- `code`：字符串类型，6 位数字（如 `"100000"`、`"101001"`）
- `msg`：强制返回空字符串，不承载用户可读文本
- `request_id`：链路追踪 ID，配合响应 Header `X-Request-ID`
- 前端根据 `code` 在请求拦截层映射为当前语言的 i18n 提示文本
- 成功码 `"100000"` 不需要 i18n 映射

---

## 二、编码规则

状态码采用 **6 位数字** 编码，结构为 `XX-YY-ZZ`：

| 段位 | 含义 | 说明 |
|---|---|---|
| `XX` | 系统前缀 | 标识所属系统端，`10` = Admin 端，`20` = Client 端，`30` = EA 端（预留） |
| `YY` | 业务域 | 标识业务域，每个系统端内独立编号 |
| `ZZ` | 序号 | 从 `01` 开始递增，同一业务域内唯一 |

### 系统前缀分配表

| 前缀 | 系统端 | 说明 |
|---|---|---|
| `10` | Admin 端 | 面向后台管理系统的所有状态码 |
| `20` | Client 端 | 面向 C 端用户的所有状态码 |
| `30` | EA 端 | 面向 EA/客户端（预留） |

### Admin 端 (10) 业务域分配表

| YY | 业务域 | 说明 | 码段范围 |
|---|---|---|---|
| `00` | 通用 (Common) | 参数错误、鉴权、限流等全局错误 | `100001` ~ `100099` |
| `10` | 管理员 (Admin User) | 后台管理员认证、账号管理 | `101001` ~ `101099` |
| `20` | 角色 (Role) | 角色管理 | `102001` ~ `102099` |
| `30` | 菜单 (Menu) | 菜单管理 | `103001` ~ `103099` |
| `40` | 按钮 (Button) | 按钮管理 | `104001` ~ `104099` |
| `50` | API | API 管理 | `105001` ~ `105099` |
| `60` | 内容 (Content) | 分类、文章、Banner | `106001` ~ `106099` |
| `70` | 存储 (Storage) | 对象存储、上传凭证、上传记录 | `107001` ~ `107099` |
| `80` | 日志 (Log) | 操作日志、错误日志 | `108001` ~ `108099` |
| `90` | 系统 (System) | 系统配置、字典、任务 | `109001` ~ `109099` |

### Client 端 (20) 业务域分配表

| YY | 业务域 | 说明 | 码段范围 |
|---|---|---|---|
| `00` | 通用 (Common) | Client 端全局错误 | `200001` ~ `200099` |
| `01` | 注册 (Registration) | 用户注册相关 | `200101` ~ `200199` |
| `02` | 登录 (Login) | 用户登录相关 | `200201` ~ `200299` |
| `03` | 鉴权 (Auth) | 身份认证与权限 | `200301` ~ `200399` |
| `04` | 令牌 (Token) | Token 刷新与管理 | `200401` ~ `200499` |
| `05` | 资料与密码 (Profile) | 个人信息与密码修改 | `200501` ~ `200599` |
| `06` | 验证码 (Captcha) | 短信/图形验证码 | `200601` ~ `200699` |
| `10` | 内容 (Content) | C 端内容浏览（预留） | `201001` ~ `201099` |
| `20` | 存储 (Storage) | C 端文件上传（预留） | `202001` ~ `202099` |
| `30` | 邮件 (Email) | 邮件发送与验证 | `203001` ~ `203099` |

> **新增业务域时**，在对应系统端的分配表中追加新 YY 值，并在本文档中创建对应的章节。

---

## 三、新增状态码流程

新增状态码时，**必须**按以下顺序同步完成 5 处修改：

1. **后端错误码定义**：在 `server/internal/pkg/errorx/errorx.go` 中新增 `Code` 常量和 `codeMessages` 映射
2. **前端错误码常量**：在 `admin-web/src/service/request/backend-error.ts` 的 `BackendErrorCode` 和 `backendErrorI18nKeyMap` 中新增条目
3. **中文语言包**：在 `admin-web/src/locales/langs/zh-cn/request.ts` 的 `backend` 对象中新增条目
4. **英文语言包**：在 `admin-web/src/locales/langs/en-us/request.ts` 的 `backend` 对象中新增条目
5. **本文档**：在对应业务域章节中追加状态码记录

---

## 四、Admin 端状态码 (10xxxx)

### 4.1 通用 (1000xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `100000` | `CodeSuccess` | — | 操作成功 | Success |
| `100001` | `CodeInvalidParams` | `invalidParams` | 参数错误 | Invalid parameters |
| `100002` | `CodeUnauthorized` | `unauthorized` | 未授权，请先登录 | Unauthorized, please login |
| `100003` | `CodeForbidden` | `forbidden` | 无权限访问 | Forbidden |
| `100004` | `CodeNotFound` | `notFound` | 资源不存在 | Resource not found |
| `100005` | `CodeInternalError` | `internalError` | 服务器内部错误 | Internal server error |
| `100006` | `CodeTooManyRequest` | `tooManyRequest` | 请求过于频繁 | Too many requests |
| `100007` | `CodeBadRequest` | `badRequest` | 请求错误 | Bad request |
| `100008` | `CodeAlreadyExists` | `alreadyExists` | 资源已存在 | Resource already exists |
| `100009` | `CodeCaptchaWrong` | `captchaWrong` | 验证码错误 | Captcha is incorrect |
| `100010` | `CodeCaptchaRequired` | `captchaRequired` | 验证码必填 | Captcha is required |

### 4.2 管理员 (1010xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `101001` | `CodeUserNotFound` | `userNotFound` | 用户不存在 | User not found |
| `101002` | `CodeUserDisabled` | `userDisabled` | 用户已禁用 | User disabled |
| `101003` | `CodePasswordWrong` | `passwordWrong` | 密码错误 | Incorrect password |
| `101004` | `CodeUserAlreadyExists` | `userAlreadyExists` | 用户名已存在 | Username already exists |
| `101005` | `CodeTokenExpired` | `tokenExpired` | 令牌已过期 | Token expired |
| `101006` | `CodeTokenInvalid` | `tokenInvalid` | 令牌无效 | Invalid token |
| `101007` | `CodeOldPasswordWrong` | `oldPasswordWrong` | 原密码错误 | Incorrect old password |

### 4.3 角色 (1020xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `102001` | `CodeRoleNotFound` | `roleNotFound` | 角色不存在 | Role not found |
| `102002` | `CodeRoleInUse` | `roleInUse` | 角色正在使用中 | Role is in use |
| `102003` | `CodeRoleAlreadyExists` | `roleAlreadyExists` | 角色已存在 | Role already exists |
| `102004` | `CodeRoleCodeDuplicate` | `roleCodeDuplicate` | 角色编码重复 | Duplicate role code |
| `102005` | `CodeCannotDeleteSuper` | `cannotDeleteSuper` | 不能删除超级管理员 | Cannot delete super admin |
| `102006` | `CodeCannotModifySuper` | `cannotModifySuper` | 不能修改超级管理员 | Cannot modify super admin |

### 4.4 菜单 (1030xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `103001` | `CodeMenuNotFound` | `menuNotFound` | 菜单不存在 | Menu not found |
| `103002` | `CodeMenuHasChildren` | `menuHasChildren` | 菜单存在子菜单 | Menu has children |
| `103003` | `CodeMenuAlreadyExists` | `menuAlreadyExists` | 菜单已存在 | Menu already exists |
| `103004` | `CodeMenuRouteDuplicate` | `menuRouteDuplicate` | 菜单路由重复 | Duplicate menu route |

### 4.5 按钮 (1040xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `104001` | `CodeButtonNotFound` | `buttonNotFound` | 按钮不存在 | Button not found |
| `104002` | `CodeButtonAlreadyExists` | `buttonAlreadyExists` | 按钮已存在 | Button already exists |
| `104003` | `CodeButtonCodeDuplicate` | `buttonCodeDuplicate` | 按钮编码重复 | Duplicate button code |

### 4.6 API (1050xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `105001` | `CodeApiNotFound` | `apiNotFound` | API不存在 | API not found |
| `105002` | `CodeApiAlreadyExists` | `apiAlreadyExists` | API已存在 | API already exists |
| `105003` | `CodeApiPathDuplicate` | `apiPathDuplicate` | API路径重复 | Duplicate API path |

### 4.7 内容 (1060xx) — 待拓展

> 内容模块（分类、文章、Banner）的业务错误码预留空间为 `106001` ~ `106099`。

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| — | — | — | — | — |

### 4.8 存储 (1070xx) — 待拓展

> 存储模块（对象存储配置、上传凭证、上传记录）的业务错误码预留空间为 `107001` ~ `107099`。

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| — | — | — | — | — |

### 4.9 日志 (1080xx) — 待拓展

> 日志模块（操作日志、错误日志）的业务错误码预留空间为 `108001` ~ `108099`。

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| — | — | — | — | — |

### 4.10 系统 (1090xx) — 待拓展

> 系统模块（系统配置、字典、任务）的业务错误码预留空间为 `109001` ~ `109099`。

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| — | — | — | — | — |

---

## 五、Client 端状态码 (20xxxx)

### 5.1 通用 (2000xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `200001` | `CodeClientInvalidParams` | `clientInvalidParams` | 参数错误 | Invalid parameters |
| `200002` | `CodeClientUnauthorized` | `clientUnauthorized` | 未登录 | Not logged in |
| `200003` | `CodeClientForbidden` | `clientForbidden` | 无权限访问 | Access denied |
| `200004` | `CodeClientNotFound` | `clientNotFound` | 资源不存在 | Resource not found |
| `200005` | `CodeClientInternalError` | `clientInternalError` | 服务器内部错误 | Internal server error |
| `200006` | `CodeClientTooManyRequest` | `clientTooManyRequest` | 请求过于频繁 | Too many requests |

### 5.2 注册 (2001xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `200101` | `CodeRegisterEmailExists` | `registerEmailExists` | 邮箱已注册 | Email already registered |
| `200102` | `CodeRegisterPhoneExists` | `registerPhoneExists` | 手机号已注册 | Phone already registered |
| `200103` | `CodeRegisterDisabled` | `registerDisabled` | 注册功能已关闭 | Registration is disabled |
| `200104` | `CodeRegisterInvalidInvite` | `registerInvalidInvite` | 邀请码无效 | Invalid invitation code |
| `200105` | `CodeRegisterCaptchaExpired` | `registerCaptchaExpired` | 注册验证码已过期 | Registration captcha expired |
| `200106` | `CodeRegisterCaptchaInvalid` | `registerCaptchaInvalid` | 注册验证码无效 | Registration captcha invalid |

### 5.3 登录 (2002xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `200201` | `CodeLoginAccountNotFound` | `loginAccountNotFound` | 账号不存在 | Account not found |
| `200202` | `CodeLoginPasswordWrong` | `loginPasswordWrong` | 密码错误 | Incorrect password |
| `200203` | `CodeLoginAccountDisabled` | `loginAccountDisabled` | 账号已禁用 | Account disabled |
| `200204` | `CodeLoginAccountLocked` | `loginAccountLocked` | 账号已锁定 | Account locked |
| `200205` | `CodeLoginTooManyAttempts` | `loginTooManyAttempts` | 登录尝试次数过多 | Too many login attempts |
| `200206` | `CodeLoginCaptchaExpired` | `loginCaptchaExpired` | 登录验证码已过期 | Login captcha expired |
| `200207` | `CodeLoginCaptchaInvalid` | `loginCaptchaInvalid` | 登录验证码无效 | Login captcha invalid |

### 5.4 鉴权 (2003xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `200301` | `CodeAuthNotLoggedIn` | `authNotLoggedIn` | 未登录，请先登录 | Not logged in, please login |
| `200302` | `CodeAuthSessionExpired` | `authSessionExpired` | 登录已过期，请重新登录 | Session expired, please login again |
| `200303` | `CodeAuthCredentialInvalid` | `authCredentialInvalid` | 登录凭证无效 | Invalid credentials |
| `200304` | `CodeAuthCredentialRevoked` | `authCredentialRevoked` | 登录凭证已撤销 | Credentials revoked |
| `200305` | `CodeAuthAccessDenied` | `authAccessDenied` | 无权限访问该资源 | Access denied to resource |

### 5.5 令牌 (2004xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `200401` | `CodeRefreshTokenExpired` | `refreshTokenExpired` | 刷新令牌已过期，请重新登录 | Refresh token expired, please login again |
| `200402` | `CodeRefreshTokenInvalid` | `refreshTokenInvalid` | 刷新令牌无效 | Invalid refresh token |
| `200403` | `CodeRefreshTokenRevoked` | `refreshTokenRevoked` | 刷新令牌已撤销 | Refresh token revoked |
| `200404` | `CodeRefreshTokenMismatch` | `refreshTokenMismatch` | 刷新令牌不匹配 | Refresh token mismatch |

### 5.6 资料与密码 (2005xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `200501` | `CodeProfileOldPasswordWrong` | `profileOldPasswordWrong` | 原密码错误 | Incorrect old password |
| `200502` | `CodeProfilePasswordTooWeak` | `profilePasswordTooWeak` | 密码强度不足 | Password is too weak |
| `200503` | `CodeProfilePasswordFormatInvalid` | `profilePasswordFormatInvalid` | 密码格式不正确 | Invalid password format |
| `200504` | `CodeProfilePhoneFormatInvalid` | `profilePhoneFormatInvalid` | 手机号格式不正确 | Invalid phone format |
| `200505` | `CodeProfileEmailFormatInvalid` | `profileEmailFormatInvalid` | 邮箱格式不正确 | Invalid email format |
| `200506` | `CodeProfileNicknameTooLong` | `profileNicknameTooLong` | 昵称过长 | Nickname too long |
| `200507` | `CodeProfileAvatarTooLarge` | `profileAvatarTooLarge` | 头像文件过大 | Avatar file too large |

### 5.7 验证码 (2006xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `200601` | `CodeCaptchaExpired` | `captchaExpired` | 验证码已过期 | Captcha expired |
| `200602` | `CodeCaptchaInvalid` | `captchaInvalid` | 验证码无效 | Captcha invalid |
| `200603` | `CodeCaptchaMaxAttempts` | `captchaMaxAttempts` | 验证码尝试次数超限 | Captcha max attempts exceeded |
| `200604` | `CodeCaptchaSendTooFrequent` | `captchaSendTooFrequent` | 验证码发送过于频繁 | Captcha sent too frequently |

### 5.8 邮件 (2030xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `203001` | `CodeEmailSendFailed` | `emailSendFailed` | 邮件发送失败 | Email send failed |
| `203002` | `CodeEmailTemplateNotFound` | `emailTemplateNotFound` | 邮件模板不存在 | Email template not found |
| `203003` | `CodeEmailServiceUnavailable` | `emailServiceUnavailable` | 邮件服务不可用 | Email service unavailable |
| `203004` | `CodeEmailRateLimitExceeded` | `emailRateLimitExceeded` | 邮件发送频率超限 | Email rate limit exceeded |
| `203005` | `CodeEmailAddressInvalid` | `emailAddressInvalid` | 邮箱地址无效 | Invalid email address |
| `203006` | `CodeEmailSendTimeout` | `emailSendTimeout` | 邮件发送超时 | Email send timeout |
| `203101` | `CodeEmailVerifyCodeExpired` | `emailVerifyCodeExpired` | 邮件验证码已过期 | Email verification code expired |
| `203102` | `CodeEmailVerifyCodeInvalid` | `emailVerifyCodeInvalid` | 邮件验证码无效 | Email verification code invalid |
| `203103` | `CodeEmailVerifyMaxAttempts` | `emailVerifyMaxAttempts` | 邮件验证码尝试次数超限 | Email verification max attempts exceeded |
| `203104` | `CodeEmailVerifyCodeUsed` | `emailVerifyCodeUsed` | 邮件验证码已使用 | Email verification code already used |

---

## 六、关键文件索引

| 用途 | 路径 |
|---|---|
| 后端错误码定义 | `server/internal/pkg/errorx/errorx.go` |
| 前端错误码常量与映射 | `admin-web/src/service/request/backend-error.ts` |
| 中文语言包 | `admin-web/src/locales/langs/zh-cn/request.ts` |
| 英文语言包 | `admin-web/src/locales/langs/en-us/request.ts` |
