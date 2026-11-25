# 命名与命名空间规范（V2）

> 版本：v1.0 · 2026-11-11
> 核心原则：**每个命名空间只有一个 owner、一份定义文件；所有使用方（含 owner 自己与其他模块）必须引用这份封装，禁止第二份定义。**
> 单一事实源：`contracts/namespaces.yaml`（登记前缀与 owner）+ 各模块 `api/keys`、`api/rpc`、`api/events`、`api/channels` 包。

---

## 1. 六条铁律

1. **单一事实源**：Redis key、RPC 路由、WS 频道、事件 topic、配置 key、指标名、存储路径、任务名、Hook 名，每类都只有一个定义文件，位于 owner 模块的 `api/` 下。
2. **禁止字面量**：业务代码不得出现原始字符串 key / 路由 / 频道 / topic；只能调用 owner 模块导出的 builder 或常量。
3. **禁止二次封装**：使用方不得在自己模块里再包一层 key builder；必须直接 import owner 的 `api/keys`（或 `api/rpc`、`api/channels`）。
4. **跨模块读取是耦合信号**：优先通过 Port 获取数据；确需直接读另一个模块的 key 时，必须 import owner 的 keys 并在注释说明原因，禁止复制格式。
5. **版本化与弃用**：破坏性变更用新 key + 双读窗口；旧 key 只弃用、不复用；禁止原地改格式。
6. **环境隔离**：key 必须包含环境段；禁止跨环境共享 Redis/DB；禁止把 PII/密钥/用户输入原文拼进 key（必要时 hash）。

---

## 2. 全局格式

```
<root>:<env>:[<tenant>:]<module>:<feature>:<id>[:<sub>]
```

| 段 | 说明 |
|----|------|
| `root` | 默认 `na`，可由配置覆盖；用于多套系统共享 Redis 时隔离 |
| `env` | `dev` / `staging` / `prod`，来自配置；禁止省略 |
| `tenant` | 可选；仅多租户启用时出现 |
| `module` | Manifest 中的模块名，全局唯一 |
| `feature` | 该模块内的功能段，常量定义 |
| `id` | URL-safe；敏感值先 hash；不含空格/中文/控制字符 |
| `sub` | 可选子段，如版本、分片号 |

格式约束：

- 全小写；冒号分隔；长度 ≤ 128；禁止空格与 Unicode。
- Redis Cluster：需要多 key 原子操作时使用 hash tag `{<module>}` 或 `{<module>:<entity>}`，确保同 slot；同一 feature 的 key 保持同一 tag。
- 禁止把时间戳、随机数直接拼进 key 作为唯一性来源（用 ID 生成器）。

示例：

```
na:prod:identity:token:01J9Z3K4Y7Q2M8N1
na:prod:identity:lock:login:01J9Z3K4Y7Q2M8N1
na:prod:identity:rl:login:203.0.113.10
na:prod:realtime:ticket:01J9Z4A1B2C3D4E5
na:prod:realtime:conn:01J9Z4A1B2C3D4E5
na:prod:realtime:ch:user:01J9Z3K4Y7Q2M8N1
na:prod:gateway:nonce:app_01J9Z:9f2c...
na:prod:storage:upload:01J9Z5F6G7H8J9K0
```

---

## 3. 模块 keys 包（Redis / 缓存 / 锁 / 限流 / 幂等）

每个模块在 `internal/modules/<module>/api/keys/` 下提供唯一封装：

```
internal/modules/<module>/api/keys/
├── keys.go        # 导出 builder、常量、TTL
└── keys_test.go   # 格式快照与边界测试
```

```go
package keys

const (
    Prefix = "identity"        // 模块段
    TokenTTL = 30 * time.Minute
)

func Token(userID string) string {
    return fmt.Sprintf("%s:%s:%s:token:%s", Root(), Env(), Prefix, userID)
}
func LoginLock(ip string) string { ... }
```

规则：

- 只导出 builder/常量；不导出可拼接待替换的格式字符串。
- TTL 与 key 定义放在同一文件，避免"key 在这里、TTL 在那里"。
- owner 自己也只能通过该包生成 key，禁止业务代码手写。
- 其他模块使用时：`import "…/internal/modules/identity/api/keys"`，直接调用；**禁止**复制格式或再包一层。
- 若跨模块读取频繁，改为 Port（例如 `identity.SessionPort`），不要长期直接读 key。

---

## 4. Redis key 分类与格式

| 类别 | 格式 | TTL | 说明 |
|------|------|-----|------|
| 缓存 | `na:{env}:{module}:cache:{entity}:{id}` | 按实体 | 缓存必须带 TTL；tag 失效由 cache 子系统负责 |
| 锁 | `na:{env}:{module}:lock:{resource}:{id}` | 短 TTL + 续租 | 必须带 owner token；禁止长锁 |
| 限流 | `na:{env}:{module}:rl:{policy}:{dimension}:{value}` | 窗口 | 与 RateLimit 策略名一致 |
| 幂等 | `na:{env}:{module}:idem:{operation}:{key}` | 业务定义 | key 为客户端幂等键或请求体 hash |
| Nonce | `na:{env}:{module}:nonce:{scope}:{nonce}` | 2m（网关） | SetNX；禁止复用 |
| 会话/令牌 | `na:{env}:identity:token:{userID}` | 令牌 TTL | 仅 identity 定义 |
| 队列/Stream | `na:{env}:{module}:q:{name}` | 持久 | 消费组命名 `<module>:<name>:cg` |
| Pub/Sub | `na:{env}:{module}:ch:{name}` | 无 | 频道名与事件/WS 规则一致 |
| WS ticket | `na:{env}:realtime:ticket:{id}` | 60s | 单次使用 |
| WS 连接 | `na:{env}:realtime:conn:{connID}` | 心跳续期 | 记录 node_id/surface/principal |
| WS 在线 | `na:{env}:realtime:presence:{surface}:{id}` | 心跳 TTL | 在线状态 |
| WS 序号 | `na:{env}:realtime:seq:{channel}` | 10m 滑动续期 | 频道序号（`coordination.IncrSequence`；空闲到期重置——`realtime/api/keys.Seq` 唯一封装） |
| 去重/Inbox | `na:{env}:{module}:inbox:{messageID}` | 事件窗口 | 消费幂等 |
| 任务锁 | `na:{env}:{module}:joblock:{job}` | 运行超时 | 防重入 |
| 选主租约 | `na:{env}:scheduler:leader:{group}` | 租约 TTL | 仅 scheduler 定义 |
| 配置缓存 | `na:{env}:configcenter:cfg:{group}:{key}` | 短 TTL | 仅 configcenter 定义 |
| 特性开关 | `na:{env}:configcenter:flag:{name}` | 短 TTL | 仅 configcenter 定义 |

补充规则：

- 所有 key 默认必须有 TTL；确需持久化的，必须在 keys 包中显式命名 `Persistent` 并说明理由。
- 锁必须携带 owner token（随机 ULID）与 TTL，释放时校验 token；禁止 `DEL` 他人锁。
- 幂等 key 必须包含操作名，避免不同操作互相覆盖。
- 限流 key 的维度值需归一化（IP、用户 ID、AppKey），禁止拼接原始 Header。

---

## 5. RPC 命名与 key

| 项 | 规则 | 示例 |
|----|------|------|
| 服务名 | 模块名 | `identity` |
| 实例名 | `<module>-<role>-<n>` | `identity-core-1` |
| 端口名 | `<module>.<Port>` | `identity.UserQuery` |
| 方法 ID | `<module>.<Port>.<Method>` | `identity.UserQuery.GetUser` |
| HTTP 路由 | `/internal/v1/rpc/<module>/<port>/<method>` | `/internal/v1/rpc/identity/user_query/get_user` |
| 幂等键 | `na:{env}:{module}:idem:rpc:{method}:{key}` | — |
| 契约版本 | Manifest `contract_version` / `api_version` | — |

规则：

- 路由与方法常量定义在 `internal/modules/<module>/api/rpc/`，禁止手写字符串。
- 内部 RPC 错误码使用核心 `1xxxx`（`106xx` 传输、`104xx` 运行时、`101xx` 认证）；端口业务错误使用该平台服务的 `18xxx` 或对应领域范围。
- 服务 JWT 的 `scp` 使用方法 ID（`<module>.<Port>.<Method>`），不要用通配符；批量端口单独声明。
- 路由版本 `/v1` 与契约版本解耦：路径版本用于破坏式协议变更，`contract_version` 用于滚动升级兼容。

---

## 6. WebSocket 命名（详见 `ws-protocol.md`）

| 项 | 规则 | 示例 |
|----|------|------|
| 频道 | `user:<id>` / `admin:<id>` / `app:<appID>:<channel>` / `topic:<name>` | `user:01J9Z...` |
| 频道定义 | 唯一在 `internal/modules/realtime/api/channels/` | — |
| ticket key | `na:{env}:realtime:ticket:{id}` | TTL 60s，单次 |
| 连接 key | `na:{env}:realtime:conn:{connID}` | 含 node_id/surface |
| 在线 key | `na:{env}:realtime:presence:{surface}:{id}` | 心跳续期 |
| 广播频道 | `na:{env}:realtime:ch:{channel}` | Redis Pub/Sub |
| 消息类型 | 常量在 `internal/modules/realtime/api/protocol/` | `subscribe/message/ping` |
| 关闭码 | 见 ws-protocol.md | 100001081301… |

规则：

- 禁止在业务代码写 `"user:" + id`；必须调用 `channels.User(id)`。
- 频道名不含 PII；用户频道用 ID，不用手机号/邮箱。
- 订阅前必须过 `ChannelAuthorizer`；频道归属（user/admin/app/topic）决定 ACL 策略。

---

## 7. 事件 topic

```
domain.<context>.<entity>.<action>       domain.identity.user.registered
platform.<subsystem>.<event>             platform.config.changed
integration.<system>.<event>             integration.email.requested
```

- 定义在 `internal/modules/<module>/api/events/`；常量名 `EventUserRegistered`，方法 `Topic()` 返回常量。
- 登记在 Manifest `publishes/subscribes`；CI 生成事件目录。
- 禁止在业务代码手写 topic 字符串；Bus 的 `Publish/Subscribe` 接收事件类型而非字符串。
- 事件版本用 `SchemaVer`，不把版本拼进 topic；破坏式变更升版本并双发。

---

## 8. 配置 key

```
<module>.<section>.<key>
```

- 示例：`gateway.signature.clock_skew`、`realtime.ws.max_connections`。
- 常量定义在模块 `api/config/`；业务代码不得手写配置路径。
- 环境变量：`NETYADMIN_<MODULE>_<SECTION>_<KEY>`（大写、下划线）。
- 动态配置 key 与静态 key 分开：`configcenter` 管动态，模块配置管启动期。

---

## 9. 指标、日志、追踪

| 类型 | 规则 | 示例 |
|------|------|------|
| 指标 | `<module>_<subsystem>_<name>` | `gateway_requests_total` |
| 标签 | 低基数；禁止 userID/requestID 作为标签 | `app,api,code` |
| 日志字段 | 统一：`module/node_id/surface/request_id/trace_id/principal` | — |
| Span 名 | `<module>.<operation>` | `identity.login` |
| 事件指标 | `na_bus_published_total{topic,domain}` | — |

- 指标名与标签 key 常量定义在模块 `api/metrics/`；禁止散落字符串。

---

## 10. 存储与数据库

| 类型 | 规则 |
|------|------|
| 对象存储 key | `{env}/{module}/{yyyy}/{mm}/{uuid}`；模块独占前缀 |
| 表名 | `<module>_<entity>`；模块独占；禁止跨模块前缀 |
| 迁移文件名 | `NNNN_<module>_<desc>.up.sql` |
| 跨模块外键 | 禁止；只存 ID |
| 跨模块 JOIN | 禁止 |

---

## 11. Job / Hook / Feature / Build tag / API 路径

| 类型 | 规则 | 示例 |
|------|------|------|
| Job | `<module>.<job>` | `storage.upload_record_cleanup` |
| Hook | `<module>.<stage>.<name>` | `gateway.request.tenant` |
| Feature | `<module>.<feature>` | `gateway.ws` |
| Build tag | `<module>_<adapter>` | `cdn_cloudflare` |
| Admin API | `/admin/v1/<module>/...` | `/admin/v1/gateway/apps` |
| Client API | `/client/v1/<module>/...` | `/client/v1/content/articles` |
| Gateway API | `/gateway/v1/...` | `/gateway/v1/ws` |
| Internal RPC | `/internal/v1/rpc/...` | `/internal/v1/rpc/identity/...` |

---

## 12. 治理与 CI

### 12.1 命名空间登记

`contracts/namespaces.yaml` 登记：

```yaml
version: 1
root: na
modules:
  identity:
    prefix: identity
    owner: platform-team
    keys: internal/modules/identity/api/keys
  realtime:
    prefix: realtime
    owner: platform-team
    keys: internal/modules/realtime/api/keys
reserved_prefixes: [kernel, platform, system]
```

### 12.2 CI 检查

1. **字面量扫描**：禁止 `redis.*("...")`、`Publish("...")`、`Subscribe("...")`、`"user:" +` 等模式；必须通过 keys/topic/channel 包。
2. **重复定义**：同一前缀/格式在多个包出现 → 失败。
3. **未登记**：模块 keys 包未在 `namespaces.yaml` 登记 → 失败。
4. **格式断言**：keys 包单测快照（root/env/module/feature 段完整、长度、字符集）。
5. **TTL 检查**：Redis 写入点必须带 TTL 或显式 `Persistent` 标记。
6. **跨模块引用**：使用方 import owner keys 包；若直接 import 其他模块 `internal` → 失败。

### 12.3 Code Review 清单

- [ ] 新增 key/频道/topic/路由是否有 owner 模块的 keys/api 包？
- [ ] 是否出现第二份格式字符串？
- [ ] 是否带 TTL/版本/环境段？
- [ ] 是否含 PII/密钥？
- [ ] 跨模块读取是否优先用 Port？
- [ ] 是否登记 `contracts/namespaces.yaml`？

### 12.4 弃用与迁移

- 新 key 与旧 key 双读窗口；写新 key；TTL 自然淘汰；删除旧定义。
- 禁止原地改格式；禁止复用旧 key 表达新语义。

---

## 13. 下游扩展

- 下游项目在自己的 `contracts/namespaces.yaml` 登记模块前缀；前缀不得与基座冲突。
- 下游模块遵循同样的 `api/keys` 封装规则；基座 CI 可对合并后的仓库跑前缀冲突检查。
- 下游不得修改基座模块的 keys 包；需要扩展时在自己的模块内定义新 feature 段。
