# 状态码规范（V2）

> 版本：v1.0 · 2026-11-11
> 单一事实源：`contracts/status-codes.yaml`（生成 Go/TS 常量与文档，禁止手写数字）
> 适用范围：HTTP API、WebSocket、内部 RPC、后台任务、审计与日志

---

## 1. 总则

1. **5 位字符串**：状态码是 `"10000"` 这样的 5 位字符串，不是数字；JSON 中保持字符串，避免前导零与数值精度问题。
2. **成功码统一 `10000`**：跨所有接入面。客户端只需判断 `code === "10000"`；错误码才按接入面 + 领域分配。若未来坚持每面独立成功码，将 `10000` 替换为各面的 `x000` 即可，不影响其余规则。
3. **错误码按面分配**，互不重叠：核心 `1xxxx`、Admin `2xxxx`、Client `3xxxx`、下游 `4xxxx-99999`。
4. **HTTP 状态码只表达传输语义**；业务结果以 `code` 为准，映射见 §6。
5. **码一旦发布永不复用**；废弃只能标记 `deprecated`，保留一个 major。
6. **禁止手写数字**：代码中只能使用 `contracts/status-codes.yaml` 生成的常量。
7. **消息不硬编码文案**：状态码登记 `message_key`，由后端 i18n 目录与前端 i18n 渲染。

---

## 2. 分配总表

| 范围 | 归属 | 用途 |
|------|------|------|
| `10000` | Kernel | 成功（全局） |
| `10001-10099` | Kernel | 通用错误：参数/校验/不存在/冲突/超时/内部/限流 |
| `10100-10199` | Kernel | 认证/会话：token/登录/账号状态 |
| `10200-10299` | Kernel | 数据/事务/缓存 |
| `10300-10399` | Kernel | Bus / Scheduler / Hooks |
| `10400-10499` | Kernel | 模块/运行时/部署/绑定 |
| `10500-10599` | Kernel | 配置/密钥 |
| `10600-10699` | Kernel | 传输/内部 RPC |
| `10700-10799` | Kernel | 安全：IPAC/签名/Nonce/限流 |
| `10800-10899` | Platform Services | 存储/消息/实时/平台服务通用 |
| `10900-17999` | Kernel | 保留 |
| `18000-18999` | Gateway / Platform Services | 网关与平台服务专用 |
| `19000-19999` | Kernel | 保留 |
| `20000-20999` | Admin | 通用/保留 |
| `21000-21999` | **Admin HTTP API** | 按领域细分（§3.1） |
| `22000-22999` | **Admin WebSocket** | 按协议细分（§3.2） |
| `23000-23999` | **Admin Ops/Jobs** | 调度/诊断/维护（§3.3） |
| `24000-29999` | Admin | 保留 |
| `30000-30999` | Client | 通用/保留 |
| `31000-31999` | **Client HTTP API** | 按领域细分（§3.4） |
| `32000-32999` | **Client WebSocket** | 按协议细分（§3.5） |
| `33000-39999` | Client | 保留 |
| `40000-99999` | **Downstream** | 使用者自行拓展（§5） |

> `18000-18999` 位于 `1xxxx` 核心段内，归网关/平台服务；不要与 `10001-17999` 的核心错误混用。

---

## 3. 领域细分

### 3.1 Admin HTTP API（21xxx）

| 子范围 | 领域 |
|--------|------|
| `21100-21199` | identity/auth：登录、令牌、账号 |
| `21200-21299` | rbac：角色、菜单、API、按钮、权限 |
| `21300-21399` | configcenter |
| `21400-21499` | content：分类、文章、Banner |
| `21500-21599` | storage |
| `21600-21699` | message |
| `21700-21799` | gateway governance：App/Scope/API |
| `21800-21899` | audit |
| `21900-21999` | taskcenter |

### 3.2 Admin WebSocket（22xxx）

| 子范围 | 领域 |
|--------|------|
| `22100-22199` | connect / ticket / auth |
| `22200-22299` | subscribe / channel |
| `22300-22399` | message / rate limit |
| `22400-22499` | close / session |
| `22500-22599` | protocol / schema |

### 3.3 Admin Ops/Jobs（23xxx）

| 子范围 | 领域 |
|--------|------|
| `23100-23199` | scheduler |
| `23200-23299` | diagnostics |
| `23300-23399` | maintenance |

### 3.4 Client HTTP API（31xxx）

| 子范围 | 领域 |
|--------|------|
| `31100-31199` | auth / register / login / oauth |
| `31200-31299` | profile / account |
| `31300-31399` | content |
| `31400-31499` | storage / upload |
| `31500-31599` | message / inbox |
| `31600-31699` | settings |

### 3.5 Client WebSocket（32xxx）

| 子范围 | 领域 |
|--------|------|
| `32100-32199` | connect / ticket / auth |
| `32200-32299` | subscribe / channel |
| `32300-32399` | message / rate limit |
| `32400-32499` | close / session |

### 3.6 Gateway / Platform Services（18xxx）

| 子范围 | 领域 |
|--------|------|
| `18100-18199` | signature / auth：AppKey/HMAC/时间戳/Nonce |
| `18200-18299` | scope / authorization |
| `18300-18399` | rate limit / quota |
| `18400-18499` | app / API |
| `18500-18599` | WS 入口 |

---

## 4. 初始核心码

> 以下为起始集合；新增码必须登记 `contracts/status-codes.yaml` 并走生成流程。

| 码 | 名称 | HTTP | 含义 |
|----|------|------|------|
| `10000` | CodeSuccess | 200 | 成功 |
| `10001` | CodeInvalidParams | 400 | 参数错误 |
| `10002` | CodeValidationFailed | 400 | 校验失败 |
| `10003` | CodeNotFound | 404 | 资源不存在 |
| `10004` | CodeConflict | 409 | 冲突/已存在 |
| `10005` | CodeUnauthorized | 401 | 未认证 |
| `10006` | CodeForbidden | 403 | 无权限 |
| `10007` | CodeTooManyRequests | 429 | 请求过于频繁 |
| `10008` | CodeTimeout | 503 | 请求超时 |
| `10009` | CodeInternal | 500 | 内部错误 |
| `10010` | CodeUnavailable | 503 | 服务不可用 |
| `10011` | CodeIdempotencyConflict | 409 | 幂等键冲突 |
| `10012` | CodeUnsupported | 400 | 不支持的操作 |
| `10101` | CodeTokenMissing | 401 | 缺少令牌 |
| `10102` | CodeTokenInvalid | 401 | 令牌无效 |
| `10103` | CodeTokenExpired | 401 | 令牌过期 |
| `10104` | CodeTokenRevoked | 401 | 令牌已吊销 |
| `10105` | CodeAccountDisabled | 403 | 账号已禁用 |
| `10106` | CodePermissionDenied | 403 | 权限不足 |
| `10201` | CodeDBError | 500 | 数据库错误 |
| `10202` | CodeCacheError | 500 | 缓存错误 |
| `10203` | CodeOptimisticLock | 409 | 乐观锁冲突 |
| `10204` | CodeUniqueViolation | 409 | 唯一约束冲突 |
| `10301` | CodeEventPublishFailed | 500 | 事件发布失败 |
| `10302` | CodeJobFailed | 500 | 任务执行失败 |
| `10303` | CodeHookAborted | 400 | 钩子中止 |
| `10401` | CodeModuleNotFound | 500 | 模块不存在 |
| `10402` | CodeDependencyUnresolved | 500 | 依赖未解析 |
| `10403` | CodeBindingFailed | 500 | 端口绑定失败 |
| `10404` | CodeModuleDisabled | 503 | 模块未启用 |
| `10501` | CodeConfigInvalid | 500 | 配置非法 |
| `10502` | CodeSecretMissing | 500 | 密钥缺失 |
| `10601` | CodeRPCTimeout | 503 | 内部 RPC 超时 |
| `10602` | CodeRPCUnavailable | 503 | 内部 RPC 不可用 |
| `10603` | CodeContractMismatch | 500 | 契约版本不匹配 |
| `10604` | CodeCircuitOpen | 503 | 熔断打开 |
| `10701` | CodeIPBlocked | 403 | IP 被拒绝 |
| `10702` | CodeNonceReplay | 401 | Nonce 重放 |
| `10703` | CodeSignatureInvalid | 401 | 签名无效 |
| `10801` | CodeStorageError | 500 | 存储错误 |
| `10802` | CodeMessageSendFailed | 500 | 消息发送失败 |
| `10803` | CodeRealtimeError | 500 | 实时通道错误 |
| `18101` | CodeGatewayAppNotFound | 401 | App 不存在 |
| `18102` | CodeGatewayAppDisabled | 403 | App 已禁用 |
| `18103` | CodeGatewaySignatureInvalid | 401 | 网关签名无效 |
| `18104` | CodeGatewayTimestampExpired | 401 | 网关时间戳过期 |
| `18105` | CodeGatewayNonceReplayed | 401 | 网关 Nonce 重放 |
| `18106` | CodeGatewayQuotaExceeded | 429 | App 配额超限 |
| `18201` | CodeGatewayScopeDenied | 403 | Scope 不足 |
| `18202` | CodeGatewayAPINotAllowed | 403 | API 不在白名单 |
| `18301` | CodeGatewayRateLimited | 429 | 网关限流 |
| `18401` | CodeGatewayAPINotRegistered | 404 | 开放 API 未注册 |
| `18501` | CodeGatewayWSTicketInvalid | 401 | WS ticket 无效 |
| `18502` | CodeGatewayWSTicketExpired | 401 | WS ticket 过期 |
| `18503` | CodeGatewayChannelDenied | 403 | 频道无权限 |

---

## 5. 下游扩展（4xxxx-99999）

- 每个下游项目注册一个首位数字：`4xxxx`、`5xxxx`、`6xxxx`、`7xxxx`、`8xxxx`、`9xxxx`（最多 6 个项目共享本基座时使用；单项目可只用 `4xxxx`）。
- 项目内建议：
  - `x0xxx`：项目通用错误；
  - `x1xxx`-`x9xxx`：按模块/领域分配（如 `41000-41999` 模块 1）。
- 下游在**自己的** `contracts/status-codes.yaml` 登记；基座只校验范围不占用 `1xxxx-3xxxx`。
- 下游不得把 `4xxxx` 用于与基座语义冲突的通用错误；通用错误继续复用 `1xxxx`。

---

## 6. HTTP 状态码映射

| 业务码 | HTTP | 说明 |
|--------|------|------|
| `10000` | 200 | 成功 |
| `10001002012` | 400 | 参数/校验/不支持 |
| `10005101102103104702703` | 401 | 未认证 |
| `10006105106701` | 403 | 无权限 |
| `10003` | 404 | 不存在 |
| `10004011203204` | 409 | 冲突 |
| `10007106301` | 429 | 限流 |
| `10008010404601602604` | 503 | 超时/不可用 |
| `100092012023013024014024035015026038xx` | 500 | 服务端错误 |

> 具体映射以 `contracts/status-codes.yaml` 的 `http` 字段为准；本表是默认约定。

---

## 7. 命名与登记

- Go 常量名：`Code<Domain><Name>`，如 `CodeAdminIdentityTokenExpired`、`CodeGatewaySignatureInvalid`。
- 消息 key：`<surface>.<domain>.<error>`，如 `admin.identity.token_expired`。
- 登记字段：

```yaml
- code: "21103"
  name: CodeAdminIdentityTokenExpired
  surface: admin.http
  domain: identity
  http: 401
  retryable: false
  message_key: auth.token_expired
  owner: platform-team
  since: 2.0.0
  deprecated: false
```

- 生成物：
  - `internal/kernel/errorx/codes_gen.go`
  - `admin/src/constants/status-codes.ts`
  - `docs/rules/status-codes-index.md`
- CI 校验：唯一性、范围归属、无越界、无未登记、无代码字面量。

---

## 8. 使用规则

### 后端

```go
return errorx.New(errorx.CodeAdminIdentityTokenExpired)
return errorx.New(errorx.CodeInvalidParams, "email", "invalid")
```

- 禁止 `errorx.New("21103")` 或裸数字。
- 响应统一：`{code, message_key, data, details, request_id, trace_id}`；文案由 i18n 目录按 `message_key` 渲染（字段名与 contracts/status-codes.yaml 登记一致）。

### 前端

```ts
import { StatusCode, isSuccess } from '@/constants/status-codes'

if (!isSuccess(res.code)) {
  // 按 code 区间/具体码处理
}
```

- 禁止硬编码字符串；禁止用 HTTP status 判断业务结果。

### WebSocket

- 错误帧携带 `code`（同上）+ `message_key`；关闭帧使用 WS close code（见 `ws-protocol.md`），不与业务状态码混用。

---

## 9. 废弃与迁移

- 标记 `deprecated: true`，保留一个 major；文档与生成常量标注。
- 禁止删除或复用；新码替代旧码时，双发窗口内同时返回（`deprecated_code` 字段可选）。
- 下游升级指南列出废弃码与替代码。

> 网关 quota：`RateLimitEnabled=true 且 Rate=0` = 显式不限流（免 Redis 往返）——契约定稿见 docs/guide/gateway-canonical.md。
