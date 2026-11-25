# 客户端接入指南：HTTP API 与 WebSocket

> 读者：要写一个客户端（移动 App / H5 / 小程序 / 第三方服务端 SDK）对接 NetyAdmin 的开发者。
> 本文给出三个接入面的选择、完整调用序列与可运行示例。协议细节以 [WS 协议规范](../rules/ws-protocol.md) 与 [API 契约](../../contracts/openapi/README.md) 为准。

## 1. 三个接入面，先选对门

| 面 | 路径前缀 | 谁用 | 鉴权 | 说明 |
|----|---------|------|------|------|
| **admin** | `/admin/v1` | 管理后台 | Admin JWT + RBAC 权限码 | 仓库自带 `admin/` 前端即完整参考实现 |
| **client** | `/client/v1` | C 端业务客户端（App / H5 / 小程序） | User JWT | 基座用 `users` 示例模块演示；下游替换为自己的业务模块 |
| **gateway** | `/gateway/v1` | 第三方系统（服务端对服务端） | AppKey + HMAC-SHA256 签名 | 治理面：应用、scope、配额、IP 管控、调用统计 |

三个面挂在**同一个进程**上，共用同一套错误信封、分页与限流设施；区别只在身份模型与权限边界。

- 做管理后台 → 直接改 `admin/` 前端，或按 §6 对接 admin 面。
- 做自己的 App / H5 → **client 面**（§3、§4）。
- 把自己的 API 开放给第三方公司 → **gateway 面**（§5）。

## 2. HTTP 通用约定（三个面一致）

### 2.1 响应信封

```json
// 成功
{"code": "10000", "message_key": "ok", "data": {}, "request_id": "..."}
// 失败
{"code": "40101", "message_key": "auth.unauthorized", "details": {"reason": "..."}, "request_id": "..."}
```

- `code` 是 5 位字符串业务码，`10000` = 成功；非 10000 一律按失败处理（HTTP 状态码同步表达类别：401349/5xx）。
- `message_key` 是 i18n 键，**不要按它做逻辑分支**——用 `code` 分支。
- 5 位码分段见 [状态码规范](../rules/status-codes.md)：`1xxxx` 核心 / `21-23xxx` admin / `31-32xxx` client / `18xxx` gateway。
- `request_id` 全链唯一：报障时提供它可在日志中定位整条调用链。

### 2.2 分页

列表接口统一 query 参数 `page`（从 1 起）/ `page_size`，响应 `data`：

```json
{"list": [], "total": 0, "page": 1, "page_size": 20}
```

### 2.3 认证头

| 面 | 头 |
|----|-----|
| admin / client | `Authorization: Bearer <access_token>` |
| gateway | `X-NA-App-Id` / `X-NA-Timestamp` / `X-NA-Nonce` / `X-NA-Signature`（见 §5.3） |

### 2.4 连通性自检

```bash
curl -s http:/7.0.0.1:8080/health/live                 # 进程存活
curl -s http:/7.0.0.1:8080/client/v1/echo              # client 面可达（返回方法/路径/request_id）
```

## 3. client 面：HTTP 接入

### 3.1 前提：启用示例用户模块

`users` 是 **C 端用户示例模块**，默认关闭（`users.enabled=false`，公网注册端点不默认开放）。启用：

```yaml
# config.yaml
users:
  enabled: true
```

重启后 `/client/v1/users/*` 生效（数据表在迁移时已建好，无需重新迁移）。下游通常按 [新手教程](tutorial-first-module.md) 写自己的用户/业务模块替换它——client 面的路由扩展点相同。

### 3.2 端点清单

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|:----:|------|
| GET | `/client/v1/echo` | — | 连通性回显 |
| POST | `/client/v1/users/register` | — | 注册（`email` + `password`，用户名随机生成） |
| POST | `/client/v1/users/login` | — | 登录，返回 User JWT |
| GET | `/client/v1/users/me` | Bearer | 当前用户资料（email/phone 掩码返回） |
| PUT | `/client/v1/users/username` | Bearer | 修改用户名 |
| PUT | `/client/v1/users/phone` | Bearer | 修改手机号 |
| POST | `/client/v1/ws/tickets` | Bearer | 换取 WS ticket（§4） |
| GET | `/client/v1/ws?ticket=` | ticket | WebSocket 握手（§4） |
| GET | `/client/v1/messages/inbox` | Bearer | 站内信列表（分页） |
| GET | `/client/v1/messages/inbox/unread-count` | Bearer | 未读数 |
| POST | `/client/v1/messages/inbox/:id/read` | Bearer | 标记已读 |
| POST | `/client/v1/messages/inbox/read-all` | Bearer | 全部已读 |
| POST | `/client/v1/storage/credentials` | Bearer | 申请直传凭证（presigned PUT） |
| POST | `/client/v1/storage/complete` | Bearer | 上传完成确认（HMAC 防伪） |

> 站内信与直传端点随 `users.enabled=true` 一并挂载（它们以 C 端用户身份消费）。

### 3.3 完整流程（curl）

```bash
BASE=http:/7.0.0.1:8080

# ① 注册
curl -s -X POST $BASE/client/v1/users/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"S3cret-pass"}'
# → {"code":"10000","data":{"id":"...","email":"a***@example.com","username":"u_xxxx",...}}

# ② 登录（拿 access_token，默认 2 小时有效）
TOKEN=$(curl -s -X POST $BASE/client/v1/users/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"S3cret-pass"}' | jq -r .data.access_token)
# → data: {"access_token":"eyJ...","token_type":"Bearer","expires_in":7200,"username":"u_xxxx"}

# ③ 带 token 调业务接口
curl -s $BASE/client/v1/users/me -H "Authorization: Bearer $TOKEN"
curl -s "$BASE/client/v1/messages/inbox?page=1&page_size=20" -H "Authorization: Bearer $TOKEN"

# ④ 修改资料
curl -s -X PUT $BASE/client/v1/users/username \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"username":"alice"}'
```

### 3.4 Token 生命周期

- 示例模块只签发 `access_token`（2 小时），**无 refresh token**：过期（401）后重新登录。
- 下游业务若需要 refresh 轮换，参照 admin 面身份模块的 refresh + 吊销模型实现自己的登录端点（机制在 `internal/kernel/security/authn`）。
- 401 = token 过期/无效；403 = 无权限；429 = 触发限流（登录限流按 IP 每分钟 10 次）。

## 4. client 面：WebSocket 接入

### 4.1 连接时序（五步）

```
① POST /client/v1/users/login                → access_token
② POST /client/v1/ws/tickets  (Bearer)       → {"ticket":"...","expires_in":60}
③ GET  /client/v1/ws?ticket=...  (Upgrade)   → 101，随后收到 hello 帧
④ 发送 {"type":"subscribe","channel":"user:<你的用户ID>"}
⑤ 收到 {"type":"subscribed","channel":"...","seq":N} → 开始收推送
```

规则（完整版见 [WS 协议 §1](../rules/ws-protocol.md)）：

- ticket **单次使用、60 秒过期**，且绑定签发面与身份：client 的 ticket 不能用于 admin/gateway 面。
- 签发时可带允许频道集合：`{"channels":["user:01J..."]}`（绑定后只能订阅集合内频道；不带则在连接后按 ACL 判定）。
- **禁止把 JWT 放进 URL**——URL 只允许一次性 ticket。
- 握手后服务端第一帧是 `hello`：`{"type":"hello","payload":{"conn_id":"...","protocol_version":1,"heartbeat_interval":30000}}`。
- **客户端不要发 hello**：客户端帧白名单只有 `subscribe/unsubscribe/message/ping/ack/resume`，发 hello 会被按协议违规关闭（4006）。

### 4.2 帧格式（JSON 信封）

```json
{
  "id": "01J9Z4A1B2C3D4E5",
  "type": "message",
  "channel": "user:01J9Z3K4Y7Q2M8N1",
  "payload": {},
  "ts": 1730000000000,
  "schema_version": 1,
  "seq": 42
}
```

| type | 方向 | 用途 |
|------|------|------|
| `hello` | S→C | 握手成功（conn_id / protocol_version / heartbeat_interval） |
| `subscribe` / `unsubscribe` | C→S | 订阅 / 退订频道 |
| `subscribed` / `unsubscribed` | S→C | 订阅回执（带回当前 seq） |
| `message` | 双向 | 业务消息（下行主要形态） |
| `ping` / `pong` | 双向 | 心跳（见 §4.4） |
| `error` | S→C | 错误帧（`code`=5 位业务码，如 32002 频道非法） |
| `close` | S→C | 服务端关闭通知（`close_code`/`reason`），随后发 WS Close 帧 |

未知字段必须忽略（向前兼容）；未知 `type` 会被视为协议违规。

### 4.3 频道与权限（client 面）

| 频道 | 当前规则 |
|------|---------|
| `user:<你的用户ID>` | **唯一可订阅**：私有频道，仅本人；服务端向该频道推送即推到你的所有在线连接 |
| `topic:*` / `admin:*` / `app:*` | client 面拒绝（4002 / error 帧 32001） |

> client 面 ACL 是"可替换示例"定位：下游业务需要公共频道时，在自己的 client 路由/`ChannelAuthorizer` 中放开并登记。频道名必须用常量构造（`channels.User(id)` 等），不要手拼字符串。

订阅/消息两级校验：subscribe 时校验一次，每条上行 message 发送前再校验。

### 4.4 心跳与重连

| 参数 | 默认 | 行为 |
|------|------|------|
| 服务端 ping 周期 | 30s | 服务端每 30s 发 `{"type":"ping"}` |
| 客户端回 pong | — | 收到 ping 立即回 `{"type":"pong","ts":<原 ts>}`；客户端也可主动 ping，服务端回 pong |
| 空闲超时 | 90s | 无任何 ping/pong 证据 → 服务端以 1001 关闭 |
| 重连退避 | 1s→2s→4s…30s | 带 0~30% jitter；**每次重连必须重新签 ticket**（ticket 单次） |

重连后需要恢复订阅（`subscribed` 回执里的 `seq` 可用于判断是否有消息缺口；本版本不承诺断线续传，需要强一致的业务走 HTTP 拉取）。

### 4.5 关闭码

| 码 | 含义 | 客户端处理 |
|----|------|-----------|
| 1000 | 正常关闭 | 不重连 |
| 1001 | 服务端优雅关闭/发布 | 退避重连 |
| 1008 | 策略违规 | 不重连，检查权限 |
| 1013 | 过载/背压/连接数超限 | 退避重连（加长退避） |
| 4001 | ticket/JWT 过期 | **重新签发 ticket** 后重连 |
| 4002 | 频道 ACL 拒绝 | 不重连该频道，检查订阅名 |
| 4003 | 消息速率超限（默认 10/s/连接） | 降速后重连 |
| 4004 | 单帧超 64KB | 拆小 payload |
| 4005 | 同身份连接数超限（默认 5） | 关掉多余连接 |
| 4006 | 协议违规（非法帧/未知类型） | 检查帧格式，不盲目重连 |
| 4007 | 令牌吊销/账号禁用 | 重新登录 |
| 4008 | 服务端错误 | 退避重连 |

### 4.6 浏览器客户端（H5 / 小程序 webview）

WS 握手会校验 `Origin`：

- **非浏览器客户端**（App、Node、Go 等）不发 `Origin`，直接放行。
- **浏览器**必须命中白名单，否则 403：

```yaml
# config.yaml
realtime:
  client_origins:
    - https://app.example.com      # 你的 H5 部署域名
    - http://localhost:5173        # 本地开发
```

### 4.7 可运行示例（Node ≥ 22，零依赖）

```js
// client-ws-demo.mjs —— node client-ws-demo.mjs <baseUrl> <email> <password>
const [base, email, password] = process.argv.slice(2);

// ① 登录
const login = await fetch(`${base}/client/v1/users/login`, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ email, password })
}).then((r) => r.json());
if (login.code !== "10000") throw new Error(`login: ${login.code} ${login.message_key}`);
const token = login.data.access_token;

// ② 取用户 ID（订阅 user:<id> 用）与 ticket
const me = await fetch(`${base}/client/v1/users/me`, {
  headers: { Authorization: `Bearer ${token}` }
}).then((r) => r.json());
const userID = me.data.id;

const t = await fetch(`${base}/client/v1/ws/tickets`, {
  method: "POST",
  headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
  body: JSON.stringify({ channels: [`user:${userID}`] })
}).then((r) => r.json());
if (t.code !== "10000") throw new Error(`ticket: ${t.code} ${t.message_key}`);

// ③ 建立 WS（Node ≥ 22 内置 WebSocket；ticket 只放 query）
const ws = new WebSocket(`${base.replace(/^http/, "ws")}/client/v1/ws?ticket=${encodeURIComponent(t.data.ticket)}`);

let heartbeatMs = 30000;
const send = (obj) => ws.send(JSON.stringify(obj));

ws.onmessage = (ev) => {
  const frame = JSON.parse(ev.data);
  switch (frame.type) {
    case "hello":
      heartbeatMs = frame.payload.heartbeat_interval;
      console.log("hello:", frame.payload.conn_id, "heartbeat:", heartbeatMs);
      send({ type: "subscribe", channel: `user:${userID}` });
      break;
    case "subscribed":
      console.log("subscribed:", frame.channel, "seq:", frame.seq);
      break;
    case "message":
      console.log("message:", frame.channel, frame.payload);
      break;
    case "ping":                       // 服务端心跳：必须回 pong
      send({ type: "pong", ts: frame.ts });
      break;
    case "pong":
      break;
    case "error":
      console.error("error frame:", frame.code, frame.message_key, frame.channel);
      break;
    case "close":
      console.warn("server closing:", frame.close_code, frame.reason);
      break;
  }
};

ws.onclose = (ev) => {
  console.log("closed:", ev.code, ev.reason);
  if (ev.code === 1000) return;              // 主动/正常关闭不重连
  const backoff = Math.min(30000, 1000 * 2 ** retries++) * (1 + Math.random() * 0.3);
  setTimeout(connect, backoff);              // 重连 = 重新签 ticket + 重连（示例略）
};
```

浏览器端唯一差异：`new WebSocket(url)` 由浏览器自动带 `Origin`（需在 `realtime.client_origins` 登记），其余逻辑相同。

## 5. gateway 面：第三方系统接入

第三方**不做登录**——用管理端下发的 AppKey/AppSecret 对每个请求签名，服务端验证后转发到内部 client API。

### 5.1 创建应用（管理端）

管理端「开放平台 → 应用管理」创建应用，或调管理端 API（AppKey/AppSecret 由你生成并提交，服务端加密存储、不回显）：

```
POST /admin/v1/open/apps
Authorization: Bearer <admin token>
{
  "name": "合作方A",
  "appKey": "<16-64 位，全局唯一>",
  "secret": "<16-128 位，自己生成并妥善保存>",
  "scopes": ["user_base", "user_profile"]
}
```

泄露或轮换时调 `PUT /admin/v1/open/apps/:id/reset-secret`——服务端生成新密钥并**明文返回一次**。内置 `frontend-app` 应用（迁移自举）承载自有 C 端前端的 IP/配额/scope 面。

### 5.2 可用能力（scope 组）

| scope 组 | 覆盖内部 API |
|----------|-------------|
| `user_base` | `/client/v1/users/register`、`/client/v1/users/login` |
| `user_profile` | `/client/v1/users/me`、`username`、`phone` |
| `msg_inbox` | `/client/v1/messages/inbox*` |
| `storage_rw` | `/client/v1/storage/credentials`、`/complete` |

应用被授权某个 scope 组，即获得组内全部 API 的访问权。调用未登记 API → 18401；scope 不符 → 18201。

### 5.3 路径映射与签名

- 调用路径 = `/gateway/v1` + 内部 path：`POST /gateway/v1/client/v1/users/login` 会被转发到内部 `/client/v1/users/login`。
- 签名串（详见 [网关 HMAC 签名契约](gateway-canonical.md)）：

```
stringToSign = METHOD + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + payloadHash
```

`path` 含 `/gateway/v1` 前缀不含 query；GET 的 `payloadHash` = query 按 key 排序拼接原串；非 GET = body 的 SHA256 hex。

```js
// gateway-sign-demo.mjs —— Node ≥ 22 零依赖
import crypto from "node:crypto";
const gatewaySign = (appSecret, method, path, query = {}, body = "") => {
  const ts = Math.floor(Date.now() / 1000).toString();
  const nonce = crypto.randomBytes(16).toString("hex");
  const payloadHash =
    method === "GET"
      ? Object.keys(query).sort().map((k) => `${k}=${query[k]}`).join("&")
      : crypto.createHash("sha256").update(body).digest("hex");
  const sig = crypto
    .createHmac("sha256", appSecret)
    .update([method, path, ts, nonce, payloadHash].join("\n"))
    .digest("hex");
  return { "X-NA-App-Id": appKey, "X-NA-Timestamp": ts, "X-NA-Nonce": nonce, "X-NA-Signature": sig };
};

const body = JSON.stringify({ email: "alice@example.com", password: "S3cret-pass" });
const res = await fetch("http:/7.0.0.1:8080/gateway/v1/client/v1/users/login", {
  method: "POST",
  headers: { "Content-Type": "application/json", ...gatewaySign(appSecret, "POST", "/gateway/v1/client/v1/users/login", {}, body) },
  body
});
console.log(await res.json());   // 内部 client API 的正常响应
```

### 5.4 gateway 面 WebSocket

第三方系统需要实时消息时：**服务端**签名换 ticket，再连 WS。

```bash
# ① 换 ticket（HMAC 签名，path 含 /gateway/v1/ws/tickets）
POST /gateway/v1/ws/tickets        → {"ticket":"...","expires_in":60}
# ② 连接（ticket 放 query）
GET  /gateway/v1/ws?ticket=...
```

- 频道只有 `app:<你的AppID>:<频道名>`，`<频道名>` 需匹配应用 scope 中的 `ws.channel.<pattern>`（glob 通配）。
- **AppSecret 绝不能进浏览器**——所以 gateway 面不设浏览器 Origin 白名单，只接受服务端接入。
- 同一套关闭码/心跳/重连规则（§4.4、§4.5）。

### 5.5 配额与 IP 管控

管理端可对应用配置：调用速率（`Rate=0` = 显式不限）、IP 组白名单/黑名单。超配额 → 18106（HTTP 429）；IP 拒绝 → fail-closed 403。

## 6. admin 面（管理后台）

仓库自带 `admin/` 前端就是 admin 面的完整参考实现，通常无需从零对接：

- 登录：`GET /admin/v1/auth/captcha` 取验证码 → `POST /admin/v1/auth/login`（验证码 + 账号密码）→ access/refresh token；首登强制改密。
- 刷新：`POST /admin/v1/auth/refresh`（refresh_token 一次性轮换）。
- WS 客户端参考：`admin/src/core/ws/client.ts`（ticket 签发 → upgrade → hello → 订阅 → 心跳 → 指数退避重连的完整实现）。
- 接口全部权限码保护；接口清单见 [OpenAPI 契约](../../contracts/openapi/admin.v1.yaml)。

## 7. 服务端主动推送（给自己的客户端发消息）

下游业务模块向客户端推送，不改客户端协议——服务端调用 realtime 广播端口，把消息发到 `user:<id>` 频道即可（在线连接实时收到，离线不补发）。机制与代码示例见 [下游开发指南 · 五 WS 开发](downstream-development.md#五-ws-开发)。

## 8. 排错速查

| 现象 | 原因与处理 |
|------|-----------|
| `/client/v1/users/*` 全 404 | `users.enabled=false`（默认）——开启后重启 |
| 登录 400 | email/password 校验失败或密码强度不足（8–72 位） |
| 401（HTTP） | token 缺失/过期/被吊销——重新登录 |
| 429 | 限流：登录 10 次/分钟/IP；网关调用看应用配额（18106） |
| ticket 401 + 18501 | ticket 无效/已用/跨面——每连接新签一张，60s 内用完 |
| ticket 401 + 18502 | ticket 逻辑过期——重新签发 |
| WS 握手 403（纯文本） | Origin 不在白名单（浏览器）或 `X-Forwarded-Proto` 非 https（prod） |
| 连上立刻 4006 | 发了协议外帧（最常见：客户端 hello）——按 §4.2 白名单发帧 |
| 连上立刻 4001 | ticket 超 60s 或已用过 |
| 4005 | 同身份第 6 条连接——同一 token 最多 5 条 |
| 收不到推送 | 未订阅 `/client/v1/ws` 的 `user:<id>`；或推送发到了 topic/别的频道 |
| 网关 18401 | 调用的内部 path 未在 `gw_apis` 登记 |
| 网关 18201 | 应用的 scope 组不覆盖该 API——管理端授权 |
| 网关 18103 | 签名不符：检查 path 是否含 `/gateway/v1` 前缀、GET 的 query 拼接、body 哈希 |

---

适用版本 v2.0.0
