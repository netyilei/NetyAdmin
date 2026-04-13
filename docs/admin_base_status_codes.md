# Admin 基座：状态码与前端 i18n 映射（HTTP）

本文件以当前 **server/admin-web** 实际实现为准，定义 Admin 基座的状态码规范与前端多语言映射方式。

## 1) 统一原则

- HTTP 统一返回结构：`{ code, msg, data, request_id }`
- `msg`：强制返回空字符串（不承载用户可读文本）
- 前端根据 `code` 在请求拦截层映射为当前语言的提示文本（`vue-i18n`）
- `request_id`：用于链路追踪，配合响应 Header `X-Request-ID`

## 2) 成功码

- `100000`：成功

## 3) 错误码来源（后端真源）

后端错误码定义位于：[errorx.go](../../server/internal/pkg/errorx/errorx.go)

提示：后端对外传输时将 `Code` 转为字符串（例如 `100002`）。

## 4) 前端映射（i18n）

前端映射逻辑位于：

- [backend-error.ts](../../admin-web/src/service/request/backend-error.ts)
- 语言包：`admin-web/src/locales/langs/*/request.ts` 的 `request.backend.*`

## 5) Admin 基座已落地错误码表

| code | 含义（中文语义） | i18n key |
|---|---|---|
| 100001 | 参数错误 | `request.backend.invalidParams` |
| 100002 | 未授权 | `request.backend.unauthorized` |
| 100003 | 无权限 | `request.backend.forbidden` |
| 100004 | 资源不存在 | `request.backend.notFound` |
| 100005 | 服务器内部错误 | `request.backend.internalError` |
| 100006 | 请求过于频繁 | `request.backend.tooManyRequest` |
| 100007 | 请求错误 | `request.backend.badRequest` |
| 100008 | 资源已存在 | `request.backend.alreadyExists` |
| 101001 | 用户不存在 | `request.backend.userNotFound` |
| 101002 | 用户已禁用 | `request.backend.userDisabled` |
| 101003 | 密码错误 | `request.backend.passwordWrong` |
| 101004 | 用户名已存在 | `request.backend.userAlreadyExists` |
| 101005 | 令牌已过期 | `request.backend.tokenExpired` |
| 101006 | 令牌无效 | `request.backend.tokenInvalid` |
| 101007 | 原密码错误 | `request.backend.oldPasswordWrong` |
| 102001 | 角色不存在 | `request.backend.roleNotFound` |
| 102002 | 角色正在使用中 | `request.backend.roleInUse` |
| 102003 | 角色已存在 | `request.backend.roleAlreadyExists` |
| 102004 | 角色编码重复 | `request.backend.roleCodeDuplicate` |
| 102005 | 不能删除超级管理员 | `request.backend.cannotDeleteSuper` |
| 102006 | 不能修改超级管理员 | `request.backend.cannotModifySuper` |
| 103001 | 菜单不存在 | `request.backend.menuNotFound` |
| 103002 | 菜单存在子菜单 | `request.backend.menuHasChildren` |
| 103003 | 菜单已存在 | `request.backend.menuAlreadyExists` |
| 103004 | 菜单路由重复 | `request.backend.menuRouteDuplicate` |
| 104001 | 按钮不存在 | `request.backend.buttonNotFound` |
| 104002 | 按钮已存在 | `request.backend.buttonAlreadyExists` |
| 104003 | 按钮编码重复 | `request.backend.buttonCodeDuplicate` |
| 105001 | API不存在 | `request.backend.apiNotFound` |
| 105002 | API已存在 | `request.backend.apiAlreadyExists` |
| 105003 | API路径重复 | `request.backend.apiPathDuplicate` |
