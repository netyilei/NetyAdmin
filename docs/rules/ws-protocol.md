# WebSocket 协议与命名规范（V2）

> 版本：v1.0 · 2026-11-11
> 适用范围：gateway / admin / client 三端 WS
> 单一事实源：`modules/realtime/api/protocol/`（消息类型、关闭码、信封）与 `modules/realtime/api/channels/`（频道命名与 ACL）

---

## 1. 入口与握手

| Surface | Ticket 端点 | 连接端点 | 鉴权 |
|---------|------------|---------|------|
| gateway | `POST /gateway/v1/ws/tickets`（AppKey + HMAC） | `GET /gateway/v1/ws?ticket=…` | App 签名 |
| admin | `POST /admin/v1/ws/tickets`（Admin JWT） | `GET /admin/v1/ws?ticket=…` | Admin JWT + RBAC |
| client | `POST /client/v1/ws/tickets`（User JWT） | `GET /client/v1/ws?ticket=…` | User JWT |

规则：

- **禁止把 JWT/AppSecret 放在 URL**；URL 只允许一次性 ticket。
- ticket：单次使用、60s 过期、绑定 `surface + 身份 + 允许频道集合`，Redis 存储（Redis 关闭时单机内存，文档标注不支持多节点）。
- ticket 过期细分：Redis 键保留期 = 2×逻辑 TTL（过期桶）；握手被拒的票在桶内按 payload 过期时间返回 `18502`，超桶/已用/未发/跨面统一 `18501`（存储层不可辨）。
- 握手必须校验 `Origin`（浏览器）与 `X-Forwarded-Proto`（代理场景）；不通过返回 403。
- 浏览器 Origin 白名单按面配置：admin=`realtime.admin_origins`、client=`realtime.client_origins`（H5/webview 等 C 端浏览器形态）；空=拒所有带 Origin 请求，非浏览器客户端（App/SDK，无 Origin）不受影响。gateway 面不设 Origin 白名单——AppSecret 不进浏览器，第三方 WS 只能由服务端签名接入。
- `Upgrade` 路径必须绕过 `http.TimeoutHandler`（V1 只豁免 SSE，V2 为 Upgrade 增加豁免）。
- 握手成功后返回 `hello` 帧（含 `protocol_version`、`conn_id`、`heartbeat_interval`）。

---

## 2. 频道命名与 ACL

| 频道格式 | 归属 | ACL |
|---------|------|-----|
| `user:<userID>` | C 端用户私有 | 仅本人 |
| `admin:<adminID>` | 管理员私有 | 仅本人 |
| `app:<appID>:<channel>` | 外部 App | Scope：`ws.channel.<pattern>` |
| `topic:<name>` | 公共/业务主题 | surface 白名单 + 可选权限码 |

规则：

- 频道名由 `channels.User(id)`、`channels.Admin(id)`、`channels.App(appID, name)`、`channels.Topic(name)` 生成；**禁止字符串拼接**。
- 频道名不含 PII（禁止手机号/邮箱/姓名）；只用不可变 ID。
- 频道长度 ≤ 128；`<name>` 只允许小写字母、数字、`-`、`_`、`.`。
- 订阅与消息两级 ACL：`subscribe` 时校验一次，每条消息发送前再校验（防止权限变更后仍可发）。
- `ChannelAuthorizer` 由各 surface 提供：
  - gateway：Scope → 频道模式匹配；
  - admin：权限码 `ws:channel:<name>`；
  - client：私有频道 + 公开频道白名单。

---

## 3. 消息信封

```json
{
  "id": "01J9Z4A1B2C3D4E5",
  "type": "message",
  "channel": "user:01J9Z3K4Y7Q2M8N1",
  "payload": {},
  "ts": 1730000000000,
  "request_id": "…",
  "trace_id": "…",
  "schema_version": 1,
  "seq": 42
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `id` | 是 | ULID；消息唯一 ID，用于幂等/去重 |
| `type` | 是 | 消息类型常量（见 §4） |
| `channel` | 视类型 | 订阅/消息类必填 |
| `payload` | 否 | 业务负载；JSON；≤ 64KB |
| `ts` | 是 | 服务端毫秒时间戳 |
| `request_id` | 否 | 关联 HTTP 请求 |
| `trace_id` | 否 | 链路追踪 |
| `schema_version` | 是 | 负载 schema 版本；只加字段 |
| `seq` | 否 | 频道内序号，用于排序/断线续传 |

约束：

- 单帧 ≤ 64KB；超限返回 `4004 payload_too_large` 并保持连接（或按策略关闭）。
- 禁止在 payload 中放密钥、完整 PII；遵循模块数据分级。
- 未知字段必须忽略（向前兼容）。

---

## 4. 消息类型

### 4.1 客户端 → 服务端

| 类型 | 说明 | 关键字段 |
|------|------|---------|
| `subscribe` | 订阅频道 | `channel`, `since_seq?` |
| `unsubscribe` | 取消订阅 | `channel` |
| `message` | 上行业务消息 | `channel`, `payload` |
| `ping` | 心跳 | `ts` |
| `ack` | 确认收到（可选） | `id` |
| `resume` | 断线续传（可选） | `channel`, `last_seq` |

### 4.2 服务端 → 客户端

| 类型 | 说明 | 关键字段 |
|------|------|---------|
| `hello` | 握手成功 | `conn_id`, `protocol_version`, `heartbeat_interval` |
| `subscribed` | 订阅成功 | `channel`, `seq` |
| `unsubscribed` | 取消成功 | `channel` |
| `message` | 下行消息 | `id`, `channel`, `payload`, `seq` |
| `pong` | 心跳响应 | `ts` |
| `ack` | 上行确认（可选） | `id` |
| `error` | 错误帧 | `code`, `message_key`, `channel?` |
| `close` | 服务端关闭通知 | `close_code`, `reason` |

规则：

- 类型常量定义在 `modules/realtime/api/protocol/types.go`；禁止手写字符串。
- 新增类型必须向后兼容：旧客户端忽略未知类型。
- `error` 帧的 `code` 使用 `22xxx`（admin）/`32xxx`（client）/`18xxx`（gateway）WS 状态码，不与 HTTP 状态码混用。

---

## 5. 关闭码（Close Code）

| 码 | 含义 | 触发 |
|----|------|------|
| `1000` | Normal Closure | 客户端主动正常关闭 |
| `1001` | Going Away | 服务端优雅关闭/滚动升级 |
| `1008` | Policy Violation | 鉴权失败/频道越权 |
| `1013` | Try Again Later | 过载/背压/限流 |
| `4001` | Auth Expired | ticket/JWT 过期（应用码） |
| `4002` | Channel Denied | 频道 ACL 拒绝 |
| `4003` | Rate Limited | 消息速率超限 |
| `4004` | Payload Too Large | 单帧超限 |
| `4005` | Duplicate Connection | 同身份连接数超限 |
| `4006` | Protocol Violation | 非法帧/未知类型/格式错误 |
| `4007` | Session Revoked | 令牌吊销/账号禁用 |
| `4008` | Server Error | 服务端内部错误 |

规则：

- 应用码 `4000-4999` 由本规范独占；下游不得占用；需要扩展时用 `5000-5999` 并向基座登记。
- 关闭前服务端应发送 `close` 帧（含 `close_code`/`reason`），再发 WebSocket Close 帧。
- 关闭码常量定义在 `modules/realtime/api/protocol/close.go`。

---

## 6. 连接与消息限制

| 项 | 默认 | 可配置 | 说明 |
|----|------|--------|------|
| 单连接频道数 | 100 | 是 | 超限 `error` + 拒绝订阅 |
| 单用户连接数 | 5 | 是 | 超限关闭最旧连接（`4005`）；按 `principal.Type` 分型计数 |
| 单 App 连接数 | 10,000 | 是 | 超限关闭最旧同型连接（`1013`）；app principal 独立桶 |
| 节点全局连接总数 | 100000（`realtime.max_conns_total`；0=显式不限） | 拒绝新连接（资源上限红线） |
| 单帧大小 | 64KB | 是 | 超限 `4004` |
| 消息速率（每连接） | 10/s | 是 | 超限 `4003` |
| 消息速率（每 App） | 1000/s | 是 | gateway app principal 应用维度滑窗（`gateway_app` 策略，与 HTTP 面同名）；超限 `4003` |
| 广播速率（每频道） | 按策略 | 是 | 超限按策略丢弃并计数 |
| 发送队列 | 64 | 是 | 满则慢消费者断开（`1013`） |
| 心跳间隔 | 30s | 是 | `ping`/`pong` |
| 空闲超时 | 90s | 是 | 无 pong 关闭（`1001`） |

规则：

- 所有限制必须通过配置，不允许硬编码；默认值写在 `kernel/config.Realtime`（唯一源——realtime 模块不声明私有配置结构，运行面注入消费；`config.validate` 对 9 个数值项启动期 >0 fail-fast，显式非正值=拒启）。
- 慢消费者策略：队列满 → 记录指标 → 发 `1013` 关闭；禁止无限缓冲。
- 计数分层：连接数/连接级速率窗口=单节点内存（§9 连接归属节点）；频道 `seq` 与 App 维度限流=Redis（`coordination.IncrSequence` / `AllowByPolicy("gateway_app")`——multi 一致，single 无 Redis 回退进程内）。

---

## 7. 心跳、重连与续传

- 服务端每 30s 发 `ping`；客户端回 `pong`；90s 无响应关闭。
- 客户端重连使用指数退避（1s→2s→4s…上限 30s，带 jitter）。
- 续传（可选协议扩展，**本版本不提供消息窗口**）：
  - 客户端 `resume` 携带 `channel + last_seq` 时返回 `error` 帧（`resume_unavailable`）——
    客户端重新订阅，需要补齐的历史走 HTTP 拉取；
  - 保留再投（重放）能力，是后续可选的窗口实现；启用时的语义：服务端按频道保留
    最近窗口内消息，超窗/未启用统一按上述错误帧处理。
- 续传不是默认承诺；需要强一致的业务应走 HTTP 拉取。

---

## 8. 投递、顺序与幂等

| 维度 | 约定 |
|------|------|
| 投递语义 | 默认 at-most-once；需要确认时客户端发 `ack`（可选） |
| 顺序 | 单频道 FIFO（`seq` 递增）；不承诺跨频道/全局顺序 |
| 去重 | 消费方按 `id` 幂等；服务端重发保留同一 `id` |
| 重放 | 仅续传窗口内；超出窗口不重放 |
| 背压 | 有界队列 + 慢消费者断开；禁止阻塞 Hub |

---

## 9. 多节点

- 每个 edge 节点运行本地 Hub；连接归属节点。
- 频道广播：发布到 Redis Pub/Sub `na:{env}:realtime:ch:{channel}`；节点只订阅本地有订阅者的频道。
- 定向消息：连接注册表记录 `node_id`；发到节点专属频道或广播后按 `conn_id` 过滤。
- 不需要粘性会话；LB 使用 least-connections 即可。
- 在线状态：`na:{env}:realtime:presence:{surface}:{id}` + 心跳 TTL。
- 节点下线：连接自然断开；客户端重连到其他节点；在线状态 TTL 过期。

---

## 10. 安全

- ticket 单次、短 TTL、绑定身份与频道集合；禁止复用。
- 订阅/消息两级 ACL；权限变更通过事件使本地 ACL 缓存失效。
- 握手校验 Origin；生产限制允许的来源。
- 输入校验：帧结构、类型白名单、payload 大小、频道格式。
- 禁止在频道名/payload 中放密钥；PII 按模块数据分级脱敏。
- 连接级审计：surface/identity/channel/close_code/peer IP；异常关闭计入安全记录。

---

## 11. 观测

| 指标 | 说明 |
|------|------|
| `realtime_connections{surface,node}` | 当前连接数 |
| `realtime_messages_total{surface,direction,type}` | 消息量 |
| `realtime_dropped_total{surface,reason}` | 丢弃/慢消费者 |
| `realtime_close_total{surface,code}` | 关闭原因分布 |
| `realtime_subscribe_denied_total{surface}` | ACL 拒绝 |
| `realtime_ticket_issued_total{surface}` | ticket 签发 |

- 日志：`conn_id/surface/principal/channel/close_code`；禁止记录完整 payload。
- 追踪：握手与消息处理带 `trace_id`；跨节点广播保留 trace。

---

## 12. 治理与 CI

- 协议常量唯一来源：`modules/realtime/api/protocol/`；业务代码禁止手写类型/关闭码。
- 频道唯一来源：`modules/realtime/api/channels/`；禁止字符串拼接。
- 协议版本：`hello.protocol_version`；破坏式变更升版本并保留旧版本一个窗口。
- 契约测试：握手/订阅 ACL/背压/重连/续传/多节点广播/关闭排空。
- CI 扫描：`"user:" +`、`"topic:" +`、裸 `4001` 等模式 → 失败。
- 文档：协议变更必须更新本文件与事件/状态码登记。
