# API 风格规范

> 适用于 adminapi / clientapi / gateway 三个 HTTP 接入面与内部 RPC 的对外契约。
> 决策依据：（注解优先 + oasdiff 破坏性门禁）；错误模型：kernel/errorx；状态码：docs/rules/status-codes.md。

## 1. 版本与挂载

| 接入面 | 前缀 | 说明 |
|--------|------|------|
| adminapi | `/admin/v1` | 管理端 HTTP + RBAC |
| clientapi | `/client/v1` | C 端 HTTP |
| gateway | `/gateway/v1` | 外部 App HTTP（另含 WS） |
| internal RPC | `/internal/v1/rpc/{module}/{port}/{method}` | 服务间；N-1 兼容 |
| ops | `/health/live`、`/health/ready`、`/metrics` | 无版本 |

- 破坏式变更升路径版本（`/v1` → `/v2`）或走 major，旧版本保留 N-1 窗口；禁止同路径内破坏字段语义。
- 弃用三步：公告 → 双发窗口 → major 删除；被弃用端点在 OpenAPI 标 `deprecated: true`。

## 2. 路由约定

- 资源用**名词复数 + kebab-case**：`/admin/v1/admin-users`、`/admin/v1/upload-records`。
- 动作语义不放进 HTTP method 的，用**动作子路径**：`POST /articles/{id}/publish`、`POST /articles/{id}/unpublish`。
- 批量读取走**批量端口**（禁止循环调用）：`POST /admin-users/batch-get`，body `{ "ids": [...] }`。
- 列表查询一律 GET + query；创建 POST；全量替换 PUT；部分更新 PATCH；删除 DELETE。
- 禁止路由里出现模块前缀（模块归属由代码组织与 RBAC 表达，不进 URL）。

## 3. 请求约定

- Body/响应的**业务字段一律 camelCase**；协议元字段（`message_key`/`request_id`/`trace_id`/`page_size`）用 snake_case。
- 时间 **RFC3339 UTC** 字符串；ID 一律**字符串**（ULID）。
- 分页：`page`（1 起）+ `page_size`（默认 20，上限 100）；响应 data 内 `total` 必返。
- 排序：`sort=<field>:asc|desc`，字段白名单在 application 层校验（kernel/security/primitives 排序助手， 落地）。
- 过滤参数与字段同名（`status`、`keyword`），禁止 `filter_xxx` 前缀。
- 校验前置：DTO 用 validator 标签，错误码 `10001002`（见 status-codes.yaml）。

## 4. 响应信封（统一，无例外；权威定义见 docs/rules/status-codes.md §8）

```jsonc
// 成功
{ "code": "10000", "message_key": "common.success", "data": { "...": "..." } }
// 列表
{ "code": "10000", "message_key": "common.success", "data": { "list": [], "total": 0, "page": 1, "page_size": 20 } }
// 失败
{ "code": "10101", "message_key": "auth.token_missing", "details": { "reason": "token missing" },
  "request_id": "01J...", "trace_id": "..." }
```

- `code` 为 5 位**字符串**状态码（contracts/status-codes.yaml 生成常量，禁止手写数字）。
- **禁止硬编码文案**：文案由前端 i18n 目录按 `message_key` 渲染；`details` 只放结构化上下文。
- HTTP status 只表达传输语义（由 errorx 映射），业务语义看 `code`。
- 分页/空集合返回 `10000` + 空 `list`，不用错误码表达"没数据"。

## 5. 幂等与并发

- 非幂等写操作（创建/提交类 POST）支持 `Idempotency-Key` 头；网关配额与幂等键 fail-closed（contracts/state-policies.yaml）。
- 更新冲突走乐观锁：资源带 `version` 字段，冲突返回 `10203`。
- 重试语义由状态码 `retryable` 字段表达；客户端只按它重试。

## 6. OpenAPI 注解与生成

- 契约源 = Go 代码里的 **swag 注解**；`swag init --v3.1` 生成 OpenAPI 3.1，产物入库 `contracts/openapi/{surface}.v1.yaml`（单一事实源，禁止手改生成物）。
- 注解最小集：
  - 路由组头部：`@Surface admin`（admin/client/gateway）+ `@Tag <module>`；
  - 每个端点：`@ID admin.articles.list`（`{surface}.{module}.{resource}.{action}`，全局唯一）、`@Success 200 {object} Envelope{data=...}`、`@Failure {int} {object} Envelope`；
  - 错误响应必须列出该端点会返回的全部状态码常量。
- 生成命令（起接入 Make/脚本）：`swag init --v3.1 --parseDependency -g <surface 入口> -o contracts/openapi -o <surface>.v1.yaml`（按接入面独立生成）。

## 7. oasdiff 破坏性变更门禁

- CI（及本地装有 oasdiff 时）：`oasdiff breaking <origin/main 的 spec> <当前 spec>`，发现破坏性变更即失败。
- 破坏性变更的唯一出路 = 升路径版本或 major（§1，对齐）；字段新增/可选化视为兼容。
- 本地无 oasdiff 或 spec 还是占位时门禁显式 SKIP，不做假绿；占位 spec 以 `x-placeholder: true` 标记，替换生成物时必须删除该标记。
