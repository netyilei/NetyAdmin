# 状态码规范

本文档为 NetyAdmin 系统的**状态码唯一权威来源 (Single Source of Truth)**。所有新增业务模块的状态码必须基于本文档的编码规则进行拓展，并同步更新本文档。

---

## 一、统一原则

- HTTP 统一返回结构：`{ code, msg, data, request_id }`
- `code`：字符串类型，6 位数字（如 `"100000"`、`"101001"`）
- `msg`：成功时为空字符串；失败时携带错误描述（`response.Fail` 写入 `BizError.Message`）
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
| `00` | 通用 (Common) | 参数错误、鉴权、限流、超时等全局错误 | `100001` ~ `100099` |
| `10` | 管理员 (Admin User) | 后台管理员认证、账号管理 | `101001` ~ `101099` |
| `12` | 消息 (Message) | 消息模板、发送记录、驱动 | `101201` ~ `101299` |
| `13` | 开放平台 (Open Platform) | AppKey认证、签名验证、限流、Scope、调用统计 | `101301` ~ `101399` |
| `14` | IP 访问控制 (IPAC) | IP 黑白名单、CIDR 校验 | `101401` ~ `101499` |
| `15` | 存储 (Storage) | 上传凭证、上传记录 | `101501` ~ `101599` |
| `16` | 内容 (Content) | 分类、文章、Banner | `101601` ~ `101699` |
| `20` | 角色 (Role) | 角色管理 | `102001` ~ `102099` |
| `30` | 菜单 (Menu) | 菜单管理 | `103001` ~ `103099` |
| `40` | 按钮 (Button) | 按钮管理 | `104001` ~ `104099` |
| `50` | API | API 管理 | `105001` ~ `105099` |
| `90` | 系统/任务 (System) | 任务调度、邮件测试 | `109001` ~ `109099` |

> `60` / `70` / `80` 号段从未分配码位，新增业务域可按顺序取用。

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

> 当前 Client 端仅 `06 验证码` 号段有已定义码位（见 §5.1），其余号段为预留，尚未定义任何码位。

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
| `100009` | `CodeCaptchaInvalid` | `captchaWrong` | 验证码错误 | Captcha is incorrect |
| `100010` | `CodeCaptchaRequired` | `captchaRequired` | 验证码必填 | Captcha is required |
| `100011` | `CodeRequestTimeout` | `requestTimeout` | 请求超时 | Request timeout |

> **健康检查**：健康检查端点（`/health`）返回 HTTP 200 状态码，不经过业务状态码流程，由 `hellofresh/health-go` 中间件直接处理。

### 4.2 管理员 (1010xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `101001` | `CodeUserNotFound` | `userNotFound` | 用户不存在 | User not found |
| `101002` | `CodeUserDisabled` | `userDisabled` | 用户已禁用 | User disabled |
| `101003` | `CodePasswordWrong` | `passwordWrong` | 密码错误 | Incorrect password |
| `101004` | `CodeUserAlreadyExists` | `userAlreadyExists` | 用户名已存在 | Username already exists |
| `101005` | `CodeTokenExpired` | `tokenExpired` | 令牌已过期 | Token expired |
| `101006` | `CodeTokenInvalid` | `tokenInvalid` | 令牌无效 | Invalid token |
| `101007` | `CodeUserLocked` | `userLocked` | 账户已锁定 | Account locked |
| `101008` | `CodeOldPasswordWrong` | `oldPasswordWrong` | 原密码错误 | Incorrect old password |
| `101009` | `CodePasswordTooWeak` | `passwordTooWeak` | 密码强度不足 | Password is too weak |
| `101010` | `CodeOAuthAlreadyBound` | — | 第三方账号已绑定其他用户 | Third-party account already bound to another user |

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

### 4.7 内容 (1016xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `101601` | `CodeCategoryHasArticles` | — | 该分类下存在文章，请先移走或删除文章后再删除分类 | Category has articles; move or delete them first |
| `101602` | `CodeCategoryHasChildren` | — | 该分类下存在子分类，请先删除子分类后再删除父分类 | Category has children; delete them first |
| `101603` | `CodeBannerGroupHasItems` | — | 该 Banner 组下存在 Banner 项，请先移走或删除 Banner 项后再删除组 | Banner group has items; move or delete them first |

### 4.8 存储 (1015xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `101501` | `CodeUploadRecordNotFound` | — | 上传记录不存在 | Upload record not found |
| `101502` | `CodeUploadSignatureInvalid` | — | 上传凭证校验失败 | Upload signature verification failed |
| `101503` | `CodeUploadRecordCompleted` | — | 该上传记录已完成，不可重复提交 | Upload record already completed |
| `101504` | `CodeUploadRecordExpired` | — | 上传凭证已过期 | Upload credential expired |
| `101505` | `CodeUploadRecordMismatch` | — | 上传记录与请求不匹配 | Upload record does not match the request |

### 4.9 消息 (1012xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `101201` | `CodeMsgTemplateNotFound` | `msgTemplateNotFound` | 消息模板不存在 | Message template not found |
| `101202` | `CodeMsgTemplateCodeExists` | `msgTemplateCodeExists` | 模板编码已存在 | Template code already exists |
| `101203` | `CodeMsgSendFailed` | `msgSendFailed` | 消息发送失败 | Message send failed |
| `101204` | `CodeMsgRecordNotFound` | `msgRecordNotFound` | 消息记录不存在 | Message record not found |
| `101205` | `CodeMsgDriverNotFound` | `msgDriverNotFound` | 消息驱动未配置或不存在 | Message driver not configured or not found |

### 4.10 系统/任务 (1090xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `109005` | `CodeTaskNotFound` | — | 任务不存在 | Task not found |
| `109006` | `CodeTaskAlreadyRunning` | — | 任务已在运行中 | Task already running |
| `109007` | `CodeTaskNotRunning` | — | 任务未在运行 | Task not running |
| `109008` | `CodeEmailTestFailed` | `emailTestFailed` | 邮件测试发送失败 | Email test failed |

### 4.11 开放平台 (1013xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `101301` | `CodeAppKeyInvalid` | `appKeyInvalid` | AppKey无效 | Invalid AppKey |
| `101302` | `CodeSignatureFailed` | `signatureFailed` | 签名验证失败 | Signature verification failed |
| `101303` | `CodeRequestExpired` | `requestExpired` | 请求已过期 | Request expired |
| `101304` | `CodeScopeMismatch` | `scopeMismatch` | 权限不足 (Scope 不匹配) | Insufficient permissions (Scope mismatch) |
| `101305` | `CodeRateLimited` | `rateLimited` | 已触发流量限制 | Rate limit exceeded |
| `101306` | `CodeAppDisabled` | `appDisabled` | 应用已被禁用 | Application has been disabled |
| `101307` | `CodeStatisticsInvalidParams` | — | 统计参数无效 | Invalid statistics parameters |

### 4.12 IP 访问控制 (1014xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `101401` | `CodeIPBlocked` | `ipBlocked` | 访问受限 (您的 IP 已被封锁) | Access restricted (IP blocked) |
| `101402` | `CodeIPInvalid` | `ipInvalid` | 非法 IP/CIDR 格式 | Invalid IP/CIDR format |
| `101403` | `CodeWhitelistMode` | `whitelistMode` | 系统处于白名单模式，您的 IP 未被授权 | Whitelist mode: IP not authorized |

### 4.11 开放平台 (1013xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `101301` | `CodeAppKeyInvalid` | `appKeyInvalid` | AppKey无效 | Invalid AppKey |
| `101302` | `CodeSignatureFailed` | `signatureFailed` | 签名验证失败 | Signature verification failed |
| `101303` | `CodeRequestExpired` | `requestExpired` | 请求已过期 | Request expired |
| `101304` | `CodeScopeMismatch` | `scopeMismatch` | 权限不足 (Scope 不匹配) | Insufficient permissions (Scope mismatch) |
| `101305` | `CodeRateLimited` | `rateLimited` | 已触发流量限制 | Rate limit exceeded |
| `101306` | `CodeAppDisabled` | `appDisabled` | 应用已被禁用 | Application has been disabled |

---

## 五、Client 端状态码 (20xxxx)

> 当前 Client 端仅验证码号段有已定义码位（下表）；其余号段为预留，尚未定义任何码位。
> 预留码位：`200602`、`200603`。

### 5.1 验证码 (2006xx)

| code | 后端常量 | 前端 key | 中文 | English |
|---|---|---|---|---|
| `200601` | `CodeCaptchaExpired` | `captchaExpired` | 验证码已过期 | Captcha expired |
| `200604` | `CodeCaptchaSendTooFrequent` | `captchaSendTooFrequent` | 发送过于频繁，请稍后再试 | Captcha sent too frequently |
| `200605` | `CodeVerifyTypeNotConfigured` | `verifyTypeNotConfigured` | 未配置验证方式，请联系管理员 | Verification method not configured |

---

## 六、关键文件索引

| 用途 | 路径 |
|---|---|
| 后端错误码定义 | `server/internal/pkg/errorx/errorx.go` |
| 前端错误码常量与映射 | `admin-web/src/service/request/backend-error.ts` |
| 中文语言包 | `admin-web/src/locales/langs/zh-cn/request.ts` |
| 英文语言包 | `admin-web/src/locales/langs/en-us/request.ts` |

---

## 七、新增状态码示例

以新增"订单模块"状态码为例（取下一个空闲业务域 `17`）：

### 7.1 后端定义

```go
// server/internal/pkg/errorx/errorx.go

const (
    // 订单模块 (1017xx)
    CodeOrderNotFound     Code = 101701
    CodeOrderAlreadyPaid  Code = 101702
    CodeOrderCancelled    Code = 101703
)

var codeMessages = map[int]string{
    CodeOrderNotFound:    "订单不存在",
    CodeOrderAlreadyPaid: "订单已支付",
    CodeOrderCancelled:   "订单已取消",
}
```

### 7.2 前端定义

```typescript
// admin-web/src/service/request/backend-error.ts

export enum BackendErrorCode {
  // ... 现有状态码
  
  // 订单模块
  ORDER_NOT_FOUND = '101701',
  ORDER_ALREADY_PAID = '101702',
  ORDER_CANCELLED = '101703',
}

export const backendErrorI18nKeyMap: Record<string, string> = {
  // ... 现有映射
  
  // 订单模块
  [BackendErrorCode.ORDER_NOT_FOUND]: 'orderNotFound',
  [BackendErrorCode.ORDER_ALREADY_PAID]: 'orderAlreadyPaid',
  [BackendErrorCode.ORDER_CANCELLED]: 'orderCancelled',
}
```

### 7.3 语言包

```typescript
// admin-web/src/locales/langs/zh-cn/request.ts

export default {
  backend: {
    // ... 现有翻译
    
    // 订单模块
    orderNotFound: '订单不存在',
    orderAlreadyPaid: '订单已支付',
    orderCancelled: '订单已取消',
  }
}

// admin-web/src/locales/langs/en-us/request.ts

export default {
  backend: {
    // ... 现有翻译
    
    // 订单模块
    orderNotFound: 'Order not found',
    orderAlreadyPaid: 'Order already paid',
    orderCancelled: 'Order cancelled',
  }
}
```

---

## 八、相关文档

- [Server架构设计](./server-architecture.md)
- [Admin-Web架构设计](./admin-web-architecture.md)
