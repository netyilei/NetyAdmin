# 下游开发指南（基于 NetyAdmin 基座二次开发）

> 读者：拿本项目做基座的下游开发者。目标：从把项目跑起来，到建业务模块、把自己的 API/WS 开放给第三方，全程走基座既有契约。
> 配套：[下游文档入口](README.md) · [下游开发规则（AI 用）](downstream-development-rules.md) · [规则总入口](../rules/README.md) · [网关 HMAC 契约](gateway-canonical.md)。
> 约定：代码示例与 v2.0.0 代码签名一致；引用的函数/结构以仓库内文件为准。

---

## 快速上手（5 分钟）

依赖：**Go ≥ 1.26**、**PostgreSQL 18**（库 `netyadmin_v2`）、**Redis 8**（single 可不装）。管理端源码开发另需 Node ≥ 20 + pnpm。

```bash
# 1. 建库（用你的 PG 账号；并把 config.yaml 的 database.user 改成同一账号）
psql -h 127.0.0.1 -U <你的PG用户> -c "CREATE DATABASE netyadmin_v2"

# 2. 配置（config.yaml 不入库；密钥只走环境变量）
cp deploy/config.example.yaml config.yaml
export NETYADMIN_DATABASE_PASSWORD=<你的PG密码>
export NETYADMIN_GW_SECRET_KEY=$(openssl rand -base64 32)

# 3. 迁移 + 启动（默认 8080）
go run ./cmd/netyadmin -mode=migrate -migrate-action=up
go run ./cmd/netyadmin

# 4. 验证
curl http:/7.0.0.1:8080/health/ready
# 迁移命令会输出一次 bootstrap admin 密码；浏览器打开 http:/7.0.0.1:8080 登录（首次登录需改密）
```

管理端前端（可选）：`cd admin && pnpm install && pnpm dev` → http://localhost:5173（代理本机 8080）；生产 `pnpm build:pro` 后由后端同源挂载。
常见启动/登录问题见 [下游文档入口 · 排障速查](README.md#排障速查)。

接下来按顺序：**一创建模块**（你的业务落点）→ **二使用基座功能** → **三 HTTP API** → **四网关** / **五 WS**。

---

## 〇 前置认知：红线速查

写任何代码前先记住这几条（基座硬性架构边界）：

| 红线 | 内容 |
|------|------|
| 依赖向内 | `kernel ← platform/business ← edges`；禁止反向 |
| 模块边界 | 模块间只走 `api/` Port（Registry Expose/Resolve）或领域事件；禁止 import 其他模块 internal/repository/entity |
| 数据私有 | 禁止跨模块 JOIN / 事务 / 外键；数据 key 不跨模块读 |
| 单一事实源 | 状态码/keys/rpc/channels/events 用生成常量或 owner 包；禁止字面量 |
| 分层禁令 | `domain` 不 import 框架/`errorx`/`keys`；`application` 不 import `adapters` |
| DB 唯一真相源 | 缓存不是；错误日志不写 DB |
| 契约先行 | 新增码/前缀/频道/事件：先登记 `contracts/`（下游用 `*.local.yaml`），再生成常量，再写业务 |
| 基座不动 | Kernel/平台服务/接入面是基座代码；缺扩展点先提 issue/ADR，不直接改 |

下游扩展只落在三处：`internal/modules/<你的模块>/` + `contracts/*.local.yaml` + `cmd/netyadmin` 装配登记（assembly/migrate 两处各一行）。

---

## 一创建模块

### 1.1 scaffold 生成骨架

```bash
node scripts/scaffold/new-module.mjs <name> [--kind=platform-service|business|edge] [--owner=platform-team]
```

生成物（对照 [scaffold README](../../scripts/scaffold/README.md)）：`module.yaml`（过 check-manifests 的 Manifest）、`module.go`（embed 迁移/Init 注册/AttachDB 惯例的装配骨架）、`api/keys|rpc|events`（唯一封装 + 快照测试骨架）、`domain/application/adapters/migrations`（分层空目录）。

### 1.2 Manifest（module.yaml）

参照真实样例 `internal/modules/configcenter/module.yaml`：

```yaml
name: mymod
kind: business
version: 1.0.0
api_version: 1
owner: <你>
slo: { latency_p99_ms: 50, availability: 0.99 }
depends_on: []                # 只写模块名；DB 是 kernel 设施走 AttachDB，不算依赖
ports:
  - name: mymod.Service
    methods:
      - { name: DoThing, idempotent: true, timeout: 2s, retryable: false }
events:
  publishes: []               # 与 Init 里 r.Events.Register 声明一致（§13.1 双源校验）
  subscribes: []
jobs: []                      # 与 r.Jobs.Register 的 job 名双向对偶校验
tables: [mymod_things]        # 表归属唯一；跨模块表名冲突 Build 直接 fail
config_section: mymod         # 配置文件必须存在该段（缺失=Build fail）
scaling:
  mode: stateless             # stateless | leader | sharded | stateful
deployment:
  colocate_with: any
  expose_internal_rpc: false
```

### 1.3 迁移（号段 + down 对称）

规则全文见 [migrations.md](../rules/migrations.md)。下游只占 `2xxxx-9xxxx`，在 `contracts/migrations.local.yaml` 登记段（活样例：note 模块，项目 2/模块 1）：

| 项目内段 | 用途 |
|---------|------|
| `P0-P9` | schema（每表一个文件，`NNNNN_<module>_<table>_schema.up.sql`） |
| `P0-P9` | indexes/constraints（二级索引独立文件） |
| `P0-P9` | seed（只 INSERT，幂等） |
| `P0-P9` | test_seed（仅 `--with-test-seed`） |

- 文件放 `internal/modules/<mymod>/migrations/`；**每个 up 必须有对称 down**（schema=DROP TABLE、seed=按稳定 ID 删行，禁 TRUNCATE；不可逆变更 down 显式 RAISE）。
- 禁 `_fix/_alter/_v2/_patch` 后缀；开发期（首次发布前）改原编号文件，发布后只新增 + expand-contract（checksum 门禁拦截改历史）。

### 1.4 api/keys 唯一封装

key/事件 topic/权限码全部进 `api/` 包，业务代码零裸字符串（namespaces.md 铁律，CI `check-literals`/`check-redis-keys` 扫描）：

```go
// api/keys/keys.go —— Redis key builder（ns 由装配注入，键内不含 root 段）
package keys

func ThingKey(id string) string { return "thing:" + id }

// api/keys/perm.go —— 权限码（与 rbac_apis 种子同值，check-rbac-perms 对偶校验）
const (
    PermThingsList  = "mymod.things.list"
    PermThingsWrite = "mymod.things.write"
)
```

### 1.5 装配登记

```go
// module.go 骨架（scaffold 已生成，此处示意全貌）
//go:embed all:migrations
var migrationsFS embed.FS

func Migrations() fs.FS { return database.MigrationsOf(migrationsFS) }

func (m *Module) Init(_ context.Context, r *module.Registry) error {
    if err := r.Migrations.Register(m.Name(), Migrations()); err != nil { return err }
    // 对外端口：Expose 的必须是实现了 api/ 接口的实例——消费方（其他模块）
    // 只能 import 你的 api/ 子树，拿不到 application/domain 类型。
    if err := r.Services.Expose("mymod.Service", port{m.svc}); err != nil { return err }
    r.Events.Register(m.Name(), module.EventPublish, []string{events.TopicThingDone})
    return nil
}

// port 实现 api 包声明的接口（出参只用基本类型/DTO，不暴露 domain 实体）。
type port struct{ svc *application.Service }

var _ api.Service = port{}
```

> 正确模式 = `api/` 声明接口 + 模块根 `port.go` 做适配 + `Expose(适配器)`；**不要**把 `*application.Service` 直接 Expose 给下游（跨模块规则只允许 import `api/` 子树，下游无法断言 application 类型）。现成范例：`message`、`storage`、`audit`、`configcenter` 四个平台服务的 api 端口（见 §2.6）。

func (m *Module) AttachDB(pool *database.Pool, startCtx context.Context) error { /* repo 注入 */ }
```

最后两步：`cmd/netyadmin/assembly.go` registerModules 加模块实例、`cmd/netyadmin/migrate.go` moduleMigrations 登记迁移源；然后 `node scripts/ci/run.mjs` 全绿 + contracts 变更后重新规则 ACK。

### 1.6 CI 门禁自查清单

门禁项逐条（下游按清单自建 CI）：gofmt / go build / go vet / 状态码生成物一致性 / check-status-codes / check-migrations（5 位号段/段归属/schema-seed 分离/down 存在）/ check-rbac-perms（路由权限码 ↔ rbac_apis 种子对偶）/ check-ts-literals / check-profile-matrix / check-bundle-budget / check-manifests / check-namespaces / check-openapi / check-imports（分层依赖图）/ check-literals（裸码值/key）/ check-contracts-mirror。

---

## 二使用基座功能

基座能力一律经 Registry 端口消费；kernel 实例在 `cmd/netyadmin/main.go` 装配段 Expose（面名见下）。示例均摘自真实消费面。

### 2.1 Cache（面名 `kernel.Cache`）

声明 entity（class 只能声明，禁手工选层）→ `GetOrLoad` → 写路径 `Invalidate`（generation INCR 广播失效）。键契约：`na:{env}:cache:<entity>:<key>`（ns 由装配注入，`cache.New` 唯一构造点——下游不拼键）。

```go
import "github.com/netyilei/netyadmin/internal/kernel/state/cache"

// Init 期：ResolveAs 消费 + 注册实体（签名：cache.Register[T](s, EntityConfig[T])）
store, err := module.ResolveAs[*cache.Store](r.Services, "kernel.Cache")
if err != nil { return err }

type Thing struct{ /* ... */ }
things, err := cache.Register(store, cache.EntityConfig[Thing]{
    Name:        "mymod.things",        // hot_local 必须在白名单（contracts cache.hot_local_whitelist）——默认 shared 免登记
    Class:       cache.ClassShared,     // shared(L2+L3) | hot_local(L1+L2, ≤64KB) ；durable 不经过本平面
    Consistency: cache.ConsistencyEvent,
    TTL:         10 * time.Minute,
    Loader: func(ctx context.Context, key string) (Thing, bool, error) {
        return m.repo.Find(ctx, key)    // L3 直读；bool=false 缓存负结果（默认 30s）
    },
})
if err != nil { return err }

// 读：L1→L2→Loader，singleflight 合并并发回源
v, found, err := things.GetOrLoad(ctx, id)
if errors.Is(err, cache.ErrNotFound) { /* 业务 404 映射 */ }

// 写后失效：generation 自增（有 L2 时 Redis INCR 广播，各节点 gen 不匹配自动 miss）
_ = store.Invalidate(ctx, "mymod.things")
// strong 一致性配合精确删除：store.DelKeys(ctx, "mymod.things", []string{keys.ThingKey(id)})
```

边界：安全/会话/协调/PII 永不 L1；命中率 <70% 自动降级 shared（`EffectiveClass` 可观测）；`encrypt_l2`（PII L2 加密）已实装：classification=pii 实体写 L2 前 AES-256-GCM 加密（密钥 env-only `NETYADMIN_CACHE_L2_KEY`，缺失=注册失败 fail-closed）；hot_local 禁 pii。

### 2.2 Bus（事件）

三域语义（发布方选择）：`Local` 进程内同步、`Broadcast` Redis Pub/Sub 全节点扇出（**at-most-once**）、`Cluster` Streams 竞争消费（**at-least-once** + 重投 + DLQ）。Handler 必须幂等。

```go
// 发布（topic 常量在 api/events；ID/OccurredAt/Kind 空缺时总线补齐）
_ = m.bus.Publish(ctx, bus.Message{
    Topic:   events.TopicThingDone,
    Kind:    bus.KindDomain,
    Payload: mustJSON(payload),
}, bus.WithDomain(bus.DomainBroadcast))          // 热更类广播；可靠消费用 DomainCluster

// 订阅（AttachDB 期——configcenter/module.go 同惯例）
_, err = m.bus.Subscribe(events.TopicThingDone, func(ctx context.Context, msg bus.Message) error {
    var p ThingDonePayload
    if err := json.Unmarshal(msg.Payload, &p); err != nil { return err } // 返回 err=Broadcast 计数/Cluster 重投
    return m.handle(ctx, p)
} /* 订阅选项（Local/Broadcast/Cluster 三域一致语义）：
    bus.WithFilter(func(msg bus.Message) bool { return msg.SchemaVer >= 2 }), // 不匹配不投递
    bus.WithPriority(10),           // 同 topic 多订阅升序投递（相同值保持注册序）
    bus.WithConcurrency(4),         // 单订阅跨消息并发上限（默认 1=串行保序）
    bus.WithBlocking(),             // 仅 Local：发布方等待 handler 实际返回（不受 HandlerTimeout 弃等）
*/)

// 事务内发布：与业务同事务提交（database.WithinTx；事务回滚即丢弃），
// relay 以 Cluster 域异步发布（outbox 语义固定不可覆盖，显式传其他域会被拒）
_ = database.WithinTx(ctx, m.pool, func(ctx context.Context) error {
    /* ...业务写（仓储经 database.TxFrom(ctx) 取事务句柄）... */
    payload, _ := json.Marshal(ThingDonePayload{ID: thingID})
    return m.bus.PublishToOutbox(ctx, bus.Message{Topic: events.TopicThingDone, Kind: bus.KindDomain,
        Payload: payload}, bus.WithAggregate(thingID))
})
```

边界：Broadcast 队列满丢弃计数（不重投）；无 Redis 的 single 拓扑 Broadcast/Cluster 降级本地投递；DLQ 依赖 DB；outbox 单行失败行级退避（达 MaxAttempts 入 DLQ，不阻塞后续行；多 relay 实例 SKIP LOCKED 并行 claim）。

### 2.3 Session / Coordination

```go
// coordination 原语（kernel/state/coordination；nil=单机无 Redis——调用前判空）
lockKey := keys.ThingLockKey(id)
holder, ok, _ := coord.LockKey(ctx, lockKey, 5*time.Second)                  // 分布式锁
if ok { defer func() { _ = coord.UnlockKey(ctx, lockKey, holder) }() }

_, isLeader, _ := coord.Campaign(ctx, "mymod.leader", 15*time.Second)        // 选主租约
_ = coord.BeginIdempotency(ctx, keys.OpKey(bizID), 10*time.Minute)           // 幂等键（SETNX，重复=ErrDenied）
cnt, _ := coord.IncrCounter(ctx, keys.ThingCounterKey(id), time.Hour)        // 计数器
```

判定策略 fail-closed/fail-open 由 `contracts/state-policies.yaml` policy 声明（安全类 fail-closed、普通读 fail-open 回源）。JWT 会话面（轮换/吊销/token_version）由 identity 模块承担，下游业务模块不直接操作 session 平面。

### 2.4 Scheduler（Job 注册）

实现 `scheduler.Job` 接口 + module.yaml `jobs` 声明，双源对偶校验（漏一边 Build fail）。范例：`audit/application/retention.go`。

```go
// domain 或 application 层
type CleanupJob struct{ repo *adapters.Repo }

func (j *CleanupJob) Name() string { return "mymod.things.cleanup" }
func (j *CleanupJob) Spec() scheduler.Spec {
    return scheduler.Spec{
        Trigger:  scheduler.TriggerCron,
        Schedule: "0 4 * * *",            // 每日 04:00（Timezone 默认 UTC）
        Timeout:  5 * time.Minute,
        Retries:  2,
        Scope:    scheduler.ScopeCluster, // 全集群单执行（选主去重）
        // 多机防重入/幂等（默认关，显式启用）：LockEnabled: true / IdempotencyTTL: 10*time.Minute
    }
}
func (j *CleanupJob) Run(ctx context.Context, _ scheduler.Execution) error { /* 必须幂等 */ }

// Init 期注册：r.Jobs.Register(m.Name(), m.job.Name(), m.job)
```

Job 必须幂等；集群互斥用 `ScopeCluster` 或 `Spec.LockEnabled`，不要自造分布式锁轮询。

### 2.5 Transport hooks（请求级六段）

六段：`RequestReceived → RequestAuthenticated → BeforeHandler → AfterHandler → ResponseSent / RequestError`。注册（组合根或模块装配）：

```go
r.Hooks.On(bootstrap.StageAfterListen, "mymod.warmup", 10, func(ctx context.Context) error {
    return nil // 进程级六段：BeforeBootstrap/AfterBootstrap/BeforeListen/AfterListen/BeforeShutdown/AfterShutdown
})
```

请求级段经 `hooks/transport` 注入 transport 管线：Received 段在鉴权前（Principal 必缺席）；Authenticated 段 principal 已注入；AfterHandler 在 handler 返回后触发（此时 gin 已写响应，需要响应写入前干预请用 BeforeHandler）；AfterHandler/ResponseSent 禁写业务 DB。

### 2.6 平台服务端口（跨模块消费面）

业务模块消费平台服务 = `ResolveAs` 到对方的 **api/ 接口**（只能 import `api/` 子树；签名全部为中性类型，不暴露 domain 实体）：

| 端口名 | 接口包 | 能力 |
|--------|--------|------|
| `message.Sender` | `internal/modules/message/api` | 按模板发通知（邮件/短信/站内信）、直投站内信 |
| `message.Templates` | `internal/modules/message/api` | 模板自举（幂等 `Ensure`：不存在即创建，已存在返回既有） |
| `storage.UploadService` | `internal/modules/storage/api` | 直传凭证签发（presigned PUT + 完成校验 secret）、上传确认 |
| `audit.Recorder` | `internal/modules/audit/api` | 写操作审计 / 安全记录 |
| `configcenter.Provider` | `internal/modules/configcenter/api` | 读动态配置（Get / 全量快照） |
| `configcenter.Watcher` | `internal/modules/configcenter/api` | 订阅配置变更（OnChange / Snapshot / Loaded） |
| `identity.Service` | `internal/modules/identity/api` | 管理员资料只读（Profile / ListAdmins） |
| `users.Users` | `internal/modules/users/api` | C 端用户只读（Profile / Get / TokenVersion） |
| `gw.Governance` | `internal/modules/gw/api` | 开放平台治理只读（App / Apps / APIs） |
| `gw.Stats` | `internal/modules/gw/api` | 网关调用统计（Overview / Trend / Top / StatusDist / Latency） |

内核类型端口（接口/类型定义在 kernel 单一源，`internal/modules/<m>/api` 提供同类型别名，下游任选其一 import）：

| 端口名 | 消费类型 | 说明 |
|--------|----------|------|
| `rbac.Authorizer` | `rbacapi.Authorizer`（= `kernel/security/authz.Authorizer`） | RBAC 授权判定 |
| `realtime.Hub` | `realtimeapi.Hub`（= `kernel/transport/ws.Hub`） | WS 频道广播/投递 |
| `realtime.TicketStore` | `realtimeapi.TicketStore`（= `ws.RedisTicketStore`） | 一次性 ticket 存储（边缘层用） |
| `kernel.Cache` | `*cache.Store`（`kernel/state/cache`） | 缓存实体注册与读写（见 §2.1） |
| `kernel.Metrics` | `*observability.Metrics`（`kernel/observability`） | 自定义业务指标注册（与 `/metrics`、admin 观测页同 registry） |

> 另有三个端口**不面向下游**：`audit.Service`（进程关停排空）、`users.Service`（管理端全量 CRUD）、`clientapi.RouteRegistrar`（组合根→模块自注册路由，`gin.IRouter` 本身可 import）。业务模块需要对应能力时用上表 api 端口。

```go
import (
    auditapi "github.com/netyilei/netyadmin/internal/modules/audit/api"
    ccapi "github.com/netyilei/netyadmin/internal/modules/configcenter/api"
    gwapi "github.com/netyilei/netyadmin/internal/modules/gw/api"
    identityapi "github.com/netyilei/netyadmin/internal/modules/identity/api"
    messageapi "github.com/netyilei/netyadmin/internal/modules/message/api"
    storageapi "github.com/netyilei/netyadmin/internal/modules/storage/api"
    usersapi "github.com/netyilei/netyadmin/internal/modules/users/api"
)

// Init 期取端口（依赖字段注入，禁止全局变量）
sender, err := module.ResolveAs[messageapi.Sender](r.Services, "message.Sender")
tplReg, err := module.ResolveAs[messageapi.Templates](r.Services, "message.Templates")
upload, err := module.ResolveAs[storageapi.UploadService](r.Services, "storage.UploadService")
rec, err := module.ResolveAs[auditapi.Recorder](r.Services, "audit.Recorder")
cfg, err := module.ResolveAs[ccapi.Provider](r.Services, "configcenter.Provider")
watch, err := module.ResolveAs[ccapi.Watcher](r.Services, "configcenter.Watcher")
admins, err := module.ResolveAs[identityapi.Identity](r.Services, "identity.Service")
udir, err := module.ResolveAs[usersapi.Users](r.Services, "users.Users")
gov, err := module.ResolveAs[gwapi.Governance](r.Services, "gw.Governance")
stats, err := module.ResolveAs[gwapi.Stats](r.Services, "gw.Stats")

// 使用（示例：邮箱验证码 + 直传 + 审计 + 动态阈值 + 配置热更回调）
id, err := sender.Send(ctx, "<验证码模板ID>", "alice@example.com", map[string]string{"code": code})
cred, err := upload.Credentials(ctx, storageapi.CredentialsRequest{FileName: "a.png", MimeType: "image/png"})
key, err := upload.Complete(ctx, cred.RecordID, cred.Secret)
err = rec.Write(ctx, auditapi.Record{Type: auditapi.TypeOperation, Action: "order.create",
    Result: "success", ActorType: "user", ActorID: uid, RequestID: rid})
entry, err := cfg.Get(ctx, "mymod", "sms.daily_limit")
watch.OnChange(func() { reload(watch.Snapshot()) })
owner, err := admins.Profile(ctx, adminID)
user, err := udir.Get(ctx, uid)
app, err := gov.App(ctx, appKey)
ov := stats.Overview(24)
```

约束：

- 通知模板：下游用自有 code 经 `message.Templates.Ensure` 自举（见下节），或由管理端「消息管理」手工建；`Send` 的 target 随渠道（邮箱/手机号/用户 ID）。
- `audit.Record.Detail` 由产生方自行脱敏（禁 PII/密钥明文）；ID/时间由服务端补齐。
- `identity.Profile`/`users.User` 的 Email/Phone 为 PII——展示层必须脱敏，禁写日志。
- `configcenter.Watcher.OnChange` 回调内禁阻塞（只做标记/刷新）。
- 以上端口缺失时 `ResolveAs` 返回错误——按可选依赖处理（模块禁用时降级 func，不要 panic）。

#### 模板自举与验证码发送（完整链路）

下游**不需要**管理员预建模板，也**不得**直写 `message_templates`（模块数据私有；**含迁移 seed**——跨模块写表违反数据私有契约）。对下游而言，`message.Templates.Ensure` 是唯一合规的代码入口：

```go
// 依赖字段（Init 期注入；两个端口都来自 message/api）
type Notifier struct {
	sender messageapi.Sender
	tpls   messageapi.Templates
}

// 首次使用前惰性确保（repo 在 AttachDB 后可用——不要在 Init 里调用）
func (n *Notifier) sendVerifyCode(ctx context.Context, to, verifyCode string) error {
	tpl, err := n.tpls.Ensure(ctx, messageapi.TemplateSpec{
		Code:    "mymod.verify_email", // ← 模板标识：常量（决定文案；同 code 只建一次）
		Channel: messageapi.ChannelEmail,
		Subject: "您的验证码",
		Body:    "您好，您的验证码是：{{verify_code}}，5 分钟内有效。", // 占位符名自定
	})
	if err != nil {
		return fmt.Errorf("ensure template: %w", err)
	}
	// 本次验证码：只作为变量参与渲染（每次都不同，但不会创建新模板）
	_, err = n.sender.Send(ctx, tpl.ID, to, map[string]string{"verify_code": verifyCode})
	return err
}
```

> **两个 "code" 不要混**：`TemplateSpec.Code` 是**模板标识**（常量，数据库唯一键，同 code 永远只有一行）；
> `{{verify_code}}` / `vars` 是**本次发送的动态值**（验证码），每次不同，只参与正文渲染。
> 把会变的值拼进 `TemplateSpec.Code`（如 `verify_email_246810`）才会每次新建模板、无限增长。
>
> **占位符与唯一性无关**：多个模板可以自由复用同名占位符（如 `{{code}}`、`{{minutes}}`）——
> "已存在"只按 Code/name 判定。同一模板内占位符名必须与 `Send` 的 `vars` 键一致，缺失的键渲染为空串。

语义与边界：

| 情形 | 行为 |
|------|------|
| Code 不存在 | 创建模板（status=enabled），返回模板投影（含 ID） |
| Code 已存在 | 返回既有行，**不改写**内容/状态——管理员在「消息管理」的编辑与禁用为最终权威（禁用后 `Send` 返回错误） |
| 管理端删除该模板 | 下游下次 `Ensure` 会重建（自愈语义）；**要永久停用请用「禁用」而不是删除**（`Ensure` 不改写状态）。若下游缓存了模板 ID：`Send` 会返回 `ErrTemplateNotFound`——清缓存重 `Ensure` 后重试一次即可自愈 |
| Code 存在但 Channel 不符 | 返回 `messageapi.ErrTemplateSpecInvalid`（fail-loud：防静默按错误通道发送） |
| 多副本同时自举 | `name` 唯一约束兜底，冲突自动重读收敛到同一行 |
| `Ensure` 调用时机 | 首次发送前（惰性）或 `after_bootstrap` 生命周期钩子（优先级 < `db-connect`=10）；`Init` 期 repo 未接、禁止调用 |
| Code 格式 | `[a-z0-9._-]` 首字符字母数字、≤128、**至少含一个字母**（纯数字串拒绝——防手机号等被误用为标识）；建议 `<module>.<purpose>` |
| 管理端改名 | **禁止**：`name` 是业务唯一键（自举 code），改名会让下次 `Ensure` 按原 code 新建第二行——改名请删除重建 |

错误判定用 `message/api` 导出的哨兵（**不要匹配错误文本**）：`ErrTemplateNotFound` /
`ErrTemplateDisabled` / `ErrTemplateSpecInvalid` / `ErrTemplateCodeInvalid` /
`ErrTemplateConflict` / `ErrNoActiveDriver` / `ErrNotReady`：

```go
if _, err := n.tpls.Ensure(ctx, spec); errors.Is(err, messageapi.ErrTemplateSpecInvalid) {
	// 代码声明与库内模板通道冲突——修声明或走管理端处理
}
```

短信通道用 `ChannelSms` 且 `Subject` 留空；正文同样支持 `{{key}}` 占位符；
模板状态判定用 `messageapi.TemplateStatusEnabled/Disabled`（禁写 1/2 字面量）。

#### 自定义指标与观测页

下游业务指标与基座共用同一 Prometheus registry（admin「运维 → 可观测」与 `/metrics` 同源）：

```go
// Init 期 Resolve 指标端口（与 kernel.Cache 同模式）
obs, err := module.ResolveAs[*observability.Metrics](r.Services, "kernel.Metrics")
if err != nil {
	return err // 基座必 Expose；错误意味着装配异常
}

// 注册自定义 collector（命名 na_<module>_<name>_<unit>；标签低基数）
if err := obs.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
	Name: "na_mymod_orders_pending",
	Help: "待处理订单数",
}, func() float64 { return m.queueLen() })); err != nil {
	return err // 重复注册=编程错误
}
```

- **不要自建 `prometheus.NewRegistry()`**——那只存在于你自己的内存里，`/metrics` 与 admin 观测页都抓不到（基座曾有模块自建第二套 registry 导致 `na_session_*` 丢失，已根修）。
- 所有指标（含你 `Register` 的 collector）自动带 `node_id` 标签（`deployment.node_id`）——多副本抓取时按节点区分；**不要**对全局 gauge（DLQ/Outbox）跨节点求和，取 max/last。
- HTTP 业务指标已由 transport 统一采集（`na_http_*`，按 method/code 维度），无需模块自埋。
- 观测页是**本节点视图**（页面标题标明节点；卡片标注「集群」的是 DB 全局值）；多节点聚合走 Prometheus（各节点 `/metrics`）。
- 观测页阈值走 `config.observability.alerts`（键见 [可观测契约 §6](../rules/observability.md#6-告警最小集与-slomd-联动)）；页面是展示型（请求时评估、不落库），外部告警仍在 Prometheus 侧用 `/metrics` 配置。
- admin 页面权限码 `admin.observability.view`；菜单随迁移种子交付（运维 → 可观测）。

**改文案走哪边（重要）**：

| 目标 | 做法 | 效果 |
|------|------|------|
| 改线上文案（如「验证码：」→「您好，您的验证码是：」） | 管理端「消息管理」编辑模板正文 | ✅ 保存即生效（`Send` 每次从库读取并渲染，无进程内模板缓存），无需发版/重启 |
| 改代码里的 `Body/Subject` 默认值 | 改代码 | ⚠️ 只影响**首次创建**（新环境/模板被删后重建）；已存在的模板保持库内文案不变 |

即：代码里的正文是**种子文案**，上线后文案的维护点是管理端。若你的团队要求「文案即代码、发布即生效」，
应在上游把 Ensure 语义改为覆盖式（当前版本不支持，属明确取舍）。

投递语义：`Send` 返回成功 = **已受理**（渲染完成 + 记录落 pending + 入发送队列），
不代表已送达；投递结果与重试在管理端「发送记录」可见。邮件通道需
`message.smtp.host/from` 已配置（未配置时 SMTP 驱动异步失败并记 failed）——
上线前先用管理端「测试邮件」验证 SMTP 联通。

---

## 三 HTTP API 开发

### 3.1 路由注册惯例

模块暴露 `RegisterAdminRoutes(public, authed gin.IRouter, ...)`，组合根挂到 `/admin/v1` 组（对照 `configcenter/adapters/adminapi.go`）：

```go
func RegisterAdminRoutes(public, authed gin.IRouter, deps Deps) {
    authzMW := transport.AuthzMiddleware(deps.Authorizer, transport.PrincipalFrom, deps.OnDeny)
    secured := func(perm string, h gin.HandlerFunc) []gin.HandlerFunc {
        return transport.Secured(authzMW, perm, h)   // RequirePerm + AuthZ 中间件链
    }
    public.GET("/things/public", publicHandler(deps))                       // 免鉴权面（记得限流策略）
    things := authed.Group("/things")
    things.GET("", secured(keys.PermThingsList, listHandler(deps))...)      // GET + query 分页
    things.POST("", secured(keys.PermThingsWrite, createHandler(deps))...)
    things.PUT("/:id", secured(keys.PermThingsWrite, updateHandler(deps))...)
    things.DELETE("/:id", secured(keys.PermThingsWrite, deleteHandler(deps))...)
}
```

路由风格（api-style §2）：名词复数 kebab-case；动作子路径 `POST /things/{id}/publish`；禁止模块前缀进 URL。

### 3.2 信封与状态码

统一信封无例外（api-style §4）：成功 `{code:"10000", message_key, data}`；失败 `{code, message_key, details, request_id, trace_id}`。`code` 是 5 位**字符串**，常量唯一源 `kernel/errorx/codes_gen.go`（`contracts/status-codes.yaml` 生成，`go generate ./...`）——下游自有码登记 `contracts/status-codes.local.yaml`（`4xxxx-9xxxx` 段）。

```go
// 业务错误：码常量 + 结构化 details（禁硬编码文案——前端按 message_key i18n 渲染）
if err != nil {
    transport.FailOf(c, err)                          // errorx.Error → HTTP 映射 + 信封
    return
}
transport.OK(c, transport.NewPage(list, total, page, size))   // 分页信封

// service/domain 层返回：
return nil, errorx.New(errorx.CodeInvalidParams, "reason", "name required")
return nil, errorx.Wrap(errorx.CodeNotFound, err, "id", id)
```

### 3.3 分页契约

```go
page, size := transport.BindPage(c)          // page(1 起)/page_size(默认 20 上限 100)，超限自动钳制
rows, total, err := m.repo.ListPaged(ctx, transport.Offset(page, size), size)
transport.OK(c, transport.NewPage(rows, total, page, size))   // data:{list,total,page,page_size}
```

### 3.4 权限码（RequirePerm ↔ rbac_apis 种子对偶）

路由经 `transport.Secured(authzMW, perm, h)` 声明权限码（`keys/perm.go` 常量）；同一码必须在模块迁移的 `rbac_apis` 种子行登记（`act` 同值）——`check-rbac-perms` 门禁双向校验，漏登记即 FAIL。种子给 super-admin 通配行之外的角色授权走 rbac 管理页。

### 3.5 OpenAPI 注解义务：契约源 = swag 注解，`swag init --v3.1` 生成入库 `contracts/openapi/{surface}.v1.yaml`（禁手改生成物）；CI `check-openapi` + oasdiff 拦破坏性变更。

三面契约（path 前缀与文件一一对应，paths 集合互斥、组件闭包各自内嵌）：

| 契约文件 | path 前缀 | 当前规模 |
|---------|-----------|---------|
| `admin.v1.yaml` | `/admin/v1/*` | 77 paths（管理端） |
| `client.v1.yaml` | `/client/v1/*` | 13 paths（users5 / message4 / storage2 / WS ticket+upgrade2） |
| `gateway.v1.yaml` | `/gateway/v1/*` | 3 paths（`/ws` upgrade、`/ws/tickets`、业务转发 `{path}` 5 method） |

生成（swag 会把三面注解收进同一 spec，须按面切分后入库）：

```bash
swag init --v3.1 --parseDependency --parseInternal -g cmd/netyadmin/main.go -o <tmp> --ot yaml
node scripts/gen/split-openapi.mjs <tmp>/swagger.yaml   # 按 path 前缀切分 + $ref 闭包
```

注解落在三个前缀之外会切分失败（脚本报错）——新增接入面须先补 `SURFACES` 前缀规则。

```go
// @Summary 列表
// @Tags things
// @ID admin.things.list                     // {surface}.{module}.{resource}.{action} 全局唯一（client./gateway. 同构）
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/things [get]
```

共享 handler（realtime `session.Surface` 的 WS ticket/upgrade 等）无法贴注解时，在组合根以空函数锚点承载（`cmd/netyadmin/openapidocs.go` 惯例——注释说明即可）。

---

## 四加入网关（把你的 API 开放给第三方）

前提：你的端点已按 §3 挂到 client 面（`/client/v1/...`）或 admin 面。网关对**内部路由原样转发**（进程内反代），无需改业务代码。

### 4.1 治理面登记（管理端「开放平台」页，权限组 `/admin/v1/open/*`）

| 步骤 | 操作 | 端点 |
|------|------|------|
| ① 登记你的 API | `gw_apis` 加行（method+**内部 path** 形态，如 `/client/v1/things`；12753 种子=登记台账惯例） | `POST /admin/v1/open/apis` |
| ② 建 scope 组 | `gw_scope_groups` 一行（code 即 scope 值，如 `things_rw`） | `POST /admin/v1/open/scope-groups` |
| ③ 组↔API 绑定 | scope 组覆盖的 API 集 | `PUT /admin/v1/open/apis/scope-apis` |
| ④ 创建应用 | AppKey + scopes（授权组 code 集）；**AppSecret 明文一次性返回**（AES-256-GCM 密文落库，丢失只能 `PUT /open/apps/:id/reset-secret` 重置） | `POST /admin/v1/open/apps` |
| ⑤ IP 组集（可选） | 应用绑定 IP 组：deny 跨组恒优先，组集非空无命中=拒（fail-closed），空=不限 | ipac 管理端点 |
| ⑥ 配额 | `RateLimitEnabled=true` + `Rate`（RPS）；**Rate=0=显式不限流**（免 Redis 往返），关闭面是 Enabled=false | `PUT /open/apps/:id` |

### 4.2 调用方接入（HMAC 签名）

canonical 串（`authn/hmac.go` + `gw/adapters/edge.go` 为权威实现；细节表见 [gateway-canonical.md](gateway-canonical.md)）：

```
stringToSign = METHOD(大写) + "\n" + path(客户端所见完整网关 path,含 /gateway/v1,不含 query)
             + "\n" + timestamp(Unix 秒,±60s 窗) + "\n" + nonce(随机串,TTL 2m 防重放)
             + "\n" + payload
payload     = GET: query 按 key 排序后 "k1=v1&k2=v2" 原串(无 query=空串);非 GET: sha256(body) hex
签名         = HMAC-SHA256(AppSecret, stringToSign) hex 小写
```

请求头（`edge.go buildCredential` 真实契约）：`X-NA-App-Id`（=AppKey）、`X-NA-Timestamp`、`X-NA-Nonce`、`X-NA-Signature`。`gateway-canonical.md` 与本文一致，任选其一实现即可。

Go 调用方示例（单文件完整可运行，依赖仅标准库；`go mod init demo && go run main.go` 即可验证）：

```go
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// gwSign 构造签名（服务端等价实现在 kernel/security/primitives/hmac.go 的
// SignHMAC + gw/adapters/edge.go 的 payloadFor，逐字段对照即可自验）。
func gwSign(method, path, ts, nonce, payload string, secret []byte) string {
	signing := strings.ToUpper(method) + "\n" + path + "\n" + ts + "\n" + nonce + "\n" + payload
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signing))
	return hex.EncodeToString(mac.Sum(nil)) // hex 小写
}

// gwPayload 第五段：GET=query 解码值按 key 排序后 k=v 全值拼接原串（不哈希，
// 服务端 c.Request.URL.Query() 取到的就是解码值——客户端必须用解码值参与）；
// 非 GET=body sha256 hex。
func gwPayload(method string, query url.Values, body []byte) string {
	if strings.EqualFold(method, http.MethodGet) {
		ks := make([]string, 0, len(query))
		for k := range query {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		var sb strings.Builder
		for i, k := range ks {
			for _, v := range query[k] {
				if i > 0 || sb.Len() > 0 {
					sb.WriteString("&")
				}
				sb.WriteString(k)
				sb.WriteString("=")
				sb.WriteString(v)
			}
		}
		return sb.String()
	}
	if len(body) > 0 {
		h := sha256.Sum256(body)
		return hex.EncodeToString(h[:])
	}
	return ""
}

// gwDo 发起一次网关调用（path=客户端所见完整网关 path，含 /gateway/v1 前缀）。
func gwDo(client *http.Client, base, appKey, appSecret, method, path string, query url.Values, body []byte) (string, error) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonceB := make([]byte, 16)
	if _, err := rand.Read(nonceB); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(nonceB) // TTL 2 分钟窗口内不可重复

	sig := gwSign(method, path, ts, nonce, gwPayload(method, query, body), []byte(appSecret))

	u := base + path
	if enc := query.Encode(); enc != "" {
		u += "?" + enc
	}
	req, err := http.NewRequest(strings.ToUpper(method), u, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("X-NA-App-Id", appKey)
	req.Header.Set("X-NA-Timestamp", ts)
	req.Header.Set("X-NA-Nonce", nonce)
	req.Header.Set("X-NA-Signature", sig)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bodyResp), nil
}

func main() {
	resp, err := gwDo(&http.Client{Timeout: 10 * time.Second},
		"http:/7.0.0.1:8080", "<AppKey>", "<AppSecret>",
		http.MethodGet, "/gateway/v1/client/v1/things", url.Values{"page": []string{"1"}}, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(resp)
}
```

调用路径：`GET /gateway/v1/client/v1/things?page=1` ——签名 path 用**完整网关 path**（含 `/gateway/v1` 前缀）；网关验签后剥前缀重写为内部 path（`path.Clean` + `/gateway/*` 防自环），按 `gw_apis` 白名单（method+path）匹配 scope 要求，再原样转发到本进程 client 路由；转发请求注入 `X-NA-App-Id` 头（你的 handler 可经 `transport.PrincipalFrom` 感知调用方 App）。

### 4.3 失败码（18xxx 细分码族 + reason 双轨）

统一信封返回，`details.reason` 承载细分原因（契约见 `contracts/status-codes.yaml` gateway 段）：

| 码 | HTTP | 场景 |
|----|------|------|
| 18101102 | 4013 | App 不存在 / 禁用 |
| 18103104 | 401 | 签名不符或缺凭证 / 时间窗外 |
| 18105 | 401 | nonce 重放 |
| 18106301 | 429 | App 配额超限 / 限流（retryable=true） |
| 18201202 | 403 | scope 不符或授权组失效 / API 不允许 |
| 18401 | 404 | API 未登记（含 `/gateway/*` 防自环拒绝） |
| 18501-18508 | — | WS ticket/频道面（见 §五） |

---

## 五 WS 开发

### 5.1 配置面（`realtime.*`，全部可配禁硬编码）

```yaml
realtime:
  enabled: true                  # false=零注册零开销（WS 表面缺席）
  heartbeat_interval: 30         # 秒
  idle_timeout: 90               # 秒（无 pong 关闭 1001）
  send_queue: 64                 # 满则慢消费者断开 1013
  max_conns_per_principal: 5     # 超限踢最旧 4005（user/admin/service 型）
  max_conns_per_app: 10000       # 单 App 连接上限，超限踢最旧同型 1013（app principal 独立桶）
  max_channels_per_conn: 100
  max_frame_bytes: 65536         # 超限 100904
  message_rate_per_sec: 10       # 每连接超限 4003
  max_message_rate_per_app_sec: 1000 # gateway app 应用维度超限 4003（gateway_app 策略）
  admin_origins: []              # admin surface 浏览器 Origin 白名单
```

### 5.2 频道命名与 ACL（三 surface 差异）

频道由 `realtime/api/channels` 构造函数生成，禁止拼接：`user:<userID>` / `admin:<adminID>` / `app:<appKey>:<name>` / `topic:<name>`；name 段只允许小写字母数字 `-_ .`，长度 ≤128，不含 PII。

| Surface | 鉴权 | 频道 ACL（cmd/netyadmin/wswiring.go 真实实现） |
|---------|------|------|
| admin | Admin JWT + RBAC | `admin:<id>` 仅本人；`topic:<name>` 需权限码 `ws:channel:<name>`；user:/app: 拒绝 |
| gateway | App HMAC | 仅 `app:<本AppKey>:<name>`；name 匹配 App scope `ws.channel.<pattern>`（`path.Match` glob，`*` 通配） |
| client | User JWT | `user:<self>` 私有 + 公开白名单 |

订阅与消息两级校验（subscribe 时一次 + 每条消息发送前再校验，防权限变更后仍可收）。

### 5.3 ticket 流程

ticket 单次使用s 过期/绑定 surface+身份（Redis 存储）：`POST .../ws/tickets`（body `{channels: [...]}` 可空——订阅决策留给服务端 ACL）→ 响应 `data:{ticket, expires_in}` → `GET .../ws?ticket=...` upgrade（ticket 即一次性凭证，upgrade 免 HMAC/JWT）→ 服务端先发 `hello` 帧（`{conn_id, protocol_version, heartbeat_interval}`）。**客户端禁止发 hello**（帧白名单无此项，发则 4006 关连）；主动关闭用 WebSocket close 1000（无应用层 close 帧）。

### 5.4 gateway surface WS 客户端示例（Node.js ≥18，可直接运行）

协议对照：`internal/modules/realtime/api/protocol`（后端唯一源）+ `admin/src/core/ws/client.ts`（admin surface 参考实现，机制同构）。

```js
// gateway-ws-client.mjs —— HMAC 签名拿 ticket → upgrade → hello → 订阅 → 心跳
import crypto from "node:crypto";
import WebSocket from "ws"; // pnpm add ws（浏览器版用原生 WebSocket，同帧协议）

const BASE = "http:/7.0.0.1:8080";
const APP_KEY = "<你的 AppKey>";
const APP_SECRET = "<应用创建时一次性下发的 AppSecret>";

// —— canonical 签名（对照 gw/adapters/edge.go payloadFor + authn/hmac.go）——
// queryValues：{key: value} 解码后原值（与 Go 服务端 c.Request.URL.Query() 取到的
// 解码值同语义——参与签名的是解码值，URL 编码只发生在最终请求行）。
function signRequest(method, path, queryValues = {}, body = "") {
  const ts = String(Math.floor(Date.now() / 1000));
  const nonce = crypto.randomBytes(16).toString("hex");
  let payload = "";
  if (method === "GET") {
    payload = Object.keys(queryValues).sort()
      .map(k => queryValues[k] == null ? "" : `${k}=${queryValues[k]}`)
      .join("&");                                       // 排序解码值 k=v 原串；无 query=空串
  } else if (body.length > 0) {
    payload = crypto.createHash("sha256").update(body).digest("hex");
  }
  const signing = [method.toUpperCase(), path, ts, nonce, payload].join("\n");
  const sig = crypto.createHmac("sha256", APP_SECRET).update(signing).digest("hex");
  return { ts, nonce, sig };
}

// —— ① 拿 ticket（POST /gateway/v1/ws/tickets，走 edge HMAC 管线）——
async function getTicket() {
  const body = JSON.stringify({ channels: [] });         // 空=订阅决策留给服务端两级 ACL
  const { ts, nonce, sig } = signRequest("POST", "/gateway/v1/ws/tickets", "", body);
  const res = await fetch(`${BASE}/gateway/v1/ws/tickets`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-NA-App-Id": APP_KEY, "X-NA-Timestamp": ts,
      "X-NA-Nonce": nonce, "X-NA-Signature": sig,
    },
    body,
  });
  const env = await res.json();                          // 统一信封 {code, message_key, data}
  if (env.code !== "10000") throw new Error(`ticket failed: ${env.code} ${env.message_key}`);
  return env.data.ticket;                                // {ticket, expires_in}
}

// —— ② upgrade + 会话（帧协议 = ws-protocol §3-§7）——
const FRAME = { SUB: "subscribe", UNSUB: "unsubscribe", PING: "ping", PONG: "pong",
  HELLO: "hello", SUBSCRIBED: "subscribed", MESSAGE: "message", ERROR: "error", CLOSE: "close" };
const CLOSE = { AUTH_EXPIRED: 4001, CHANNEL_DENIED: 4002, RATE_LIMITED: 4003,
  PAYLOAD_TOO_LARGE: 4004, DUP_CONN: 4005, PROTOCOL: 4006, REVOKED: 4007, SERVER: 4008 };
const GW_ERR = { CHANNEL_DENIED: "18503", INVALID_CHANNEL: "18504", PROTOCOL: "18505",
  CHANNEL_LIMIT: "18506", RESUME_UNAVAILABLE: "18507", NOT_SUBSCRIBED: "18508" }; // gateway WS error 帧业务码对照（§4.3）

const socket = new WebSocket(`${BASE.replace("http", "ws")}/gateway/v1/ws?ticket=${encodeURIComponent(await getTicket())}`);
let heartbeatMs = 30_000, heartbeatTimer = null, awaitingPong = false;
const wanted = new Set();                                // 期望订阅集（重连恢复用，此处演示单连接）

socket.on("open", () => {
  // 不发 hello（帧白名单无客户端 hello——发则服务端 4006 关连）；等服务端 hello 帧
  // （超时自愈关闭用 1000 Normal——对照 admin client.ts，应用码 4001-4008 是服务端语义）
  const helloDeadline = setTimeout(() => socket.close(1000, "hello timeout"), 10_000);
  socket.on("message", (raw) => {
    const f = JSON.parse(String(raw));                   // 信封 {id,type,channel?,payload,ts,schema_version,seq?}
    switch (f.type) {
      case FRAME.HELLO: {                                // {conn_id, protocol_version, heartbeat_interval}
        clearTimeout(helloDeadline);
        heartbeatMs = f.payload.heartbeat_interval ?? heartbeatMs;
        startHeartbeat();
        const ch = `app:${APP_KEY}:orders.created`;      // 需 App scope ws.channel.orders.*（path.Match glob）
        wanted.add(ch);
        send(FRAME.SUB, { channel: ch });
        break;
      }
      case FRAME.PING: send(FRAME.PONG, { ts: f.ts }); break;          // 服务端心跳即时回 pong
      case FRAME.PONG: awaitingPong = false; break;
      case FRAME.SUBSCRIBED: console.log("subscribed", f.channel, "seq", f.seq); break;
      case FRAME.MESSAGE: console.log("msg", f.channel, f.payload, "seq", f.seq); break;
      case FRAME.ERROR:                                  // 业务码=gateway 18xxx；订阅被拒的频道移出期望集
        console.error("ws error", f.code, f.message_key, f.channel);
        if (f.channel) wanted.delete(f.channel);
        break;
      case FRAME.CLOSE: console.warn("server closing", f.close_code, f.reason); break;
      default: /* 未知类型忽略（向前兼容 §4） */ break;
    }
  });
});
socket.on("close", (code, reason) => {
  stopHeartbeat();
  // 4007 吊销=终态不重连；4001 过期重取 ticket；其余指数退避 1s→2s→…→30s 带 jitter（§7）
  console.warn("closed", code, String(reason));
});

function send(type, extra = {}) {                        // 客户端帧白名单：subscribe/unsubscribe/message/ping/ack/resume
  socket.send(JSON.stringify({ id: crypto.randomUUID(), type, ts: Date.now(), schema_version: 1, ...extra }));
}
function startHeartbeat() {                              // 以 hello.heartbeat_interval 为周期发 ping；一周期无 pong 判死重连
  heartbeatTimer = setInterval(() => {
    if (awaitingPong) return socket.close(1000, "pong timeout"); // 自愈断开=1000（对照 client.ts）；随后走重连退避
    awaitingPong = true; send(FRAME.PING, { ts: Date.now() });
  }, heartbeatMs);
}
function stopHeartbeat() { if (heartbeatTimer) clearInterval(heartbeatTimer); heartbeatTimer = null; awaitingPong = false; }
```

对齐清单（与 ws-protocol.md 逐条）：帧白名单只含 6 种客户端类型 / 不发 hello / ping-pong 双向（客户端按 `heartbeat_interval` 主动 ping + 服务端 ping 即回 pong）/ 关闭码 4001-4008 不自造 / error 帧业务码=gateway 18xxx / payload ≤64KB / 频道 `app:<AppKey>:<name>` 且需 scope `ws.channel.<pattern>`。

### 5.5 admin surface 客户端：直接复用

管理端不要新写：`admin/src/core/ws/client.ts`（`getWsClient()` 单例——多页共用一条连接，防 4005 同身份上限自我挤占）+ `protocol.ts` 常量投影。消费方式见 `admin/src/hooks/useWebsocket.ts`：`onMessage/onError/onStatus` 订阅，`subscribe(channel): Promise<number|undefined>` 等回执。重连/恢复订阅/心跳/鉴权终态识别 client.ts 已内建。

---

## 六测试与质量门禁

纪律全文见 [testing.md](../rules/testing.md)，四条硬约束：

| 项 | 要求 |
|----|------|
| 测试库隔离 | `TEST_DATABASE_DSN` 指向**测试专用库**（如 `netyadmin_v2_test`）；开发库永不作为测试目标；未设置时集成测试显式 Skip（不做假绿） |
| 串行 | 全量测试必须 `go test -race -p 1 ./...`（goose 全局版本水位共享，多包并行互删水位） |
| 断言 | 行为/数据断言，错误路径优先；错误断言必须校验 `code` 与 `message_key`；禁 `t.Skip` 无 issue |
| 门禁 | `node scripts/ci/run.mjs` ALL GREEN；契约测试 Port local/remote 双适配器跑同一 testkit 用例 |

模块集成测试模板参照 `internal/modules/configcenter/configcenter_integration_test.go`（testkit 建连 + 模块迁移号段精确清理）。

---

## 七能力限制（选型前必读）

以下是基座当前的能力边界与使用约束，按行为描述；触发条件成立时需要改基座或调整设计，不要在下游用绕过实现：

| 限制 | 影响与应对 |
|------|-----------|
| Bus `Broadcast` 为 at-most-once（队列满/掉线丢弃） | 热更类广播丢失时由消费侧退避重试 + 每 5 分钟对账收敛（陈旧上界 5 分钟）；需要秒级强一致热更请用 `Cluster` 域（at-least-once + 重投 + DLQ） |
| WS 定向消息（`PublishToPrincipal`）只在当前节点投递 | multi 拓扑下跨节点定向消息需走频道广播或业务侧路由 |
| WS ticket 超过过期桶（2×TTL）后不可辨识 | 桶内区分逻辑过期（18502）与失效（18501）；更早的旧票统一 18501 |
| WS 单 App 限流维度为 appID | 连接数/消息速率已按主体分型，但未按 API/频道细分；应用内子配额需业务侧自管 |
| PII 缓存入 L2 需配置密钥 | `NETYADMIN_CACHE_L2_KEY`（base64 32 字节）缺失时 PII 实体注册失败（fail-closed，禁止明文落共享 Redis） |
| 网关配额 `Rate=0` 表示显式不限流 | 误配 0 等于放开配额；关闭配额应使用 `RateLimitEnabled=false` |
| single 拓扑无 Redis 时能力降级 | Session/Coordination/Broadcast 降级为进程内（仅单机语义）；multi 必须 Redis，缺失=拒绝启动 |
| `deployment.role` 只过滤接入面 | 模块仍全量装配、共享单库；模块级裁剪随拆库部署形态提供 |
| WS 与网关的多节点一致性依赖 Redis | 多机必须同一 Redis（ticket/nonce/配额/频道序号），跨机房部署需评估延迟 |
| `deployment.node_id` 必须全集群唯一 | multi 每副本显式配置（env `NETYADMIN_DEPLOYMENT_NODE_ID`）；同值会导致日志/consumer 标识混叠、GW 统计快照文件互相覆盖 |
| GW 统计快照按节点分文件、聚合只读本机目录 | 同机多进程共享日志目录即可看全量历史；跨机历史聚合由日志采集器承担，管理面不跨机读文件 |

遇到影响架构的限制，先改基座再叠加业务；局部限制在触发条件成立前可按上表应对。

---

## 参考实现位置

- 模块装配与 Port：`internal/kernel/module/registry.go`（Expose/ResolveAs/Build 校验）
- 缓存：`internal/kernel/state/cache/`（Register/EntityConfig/Invalidate、L2 加密）
- 总线：`internal/kernel/bus/bus.go` + `outbox.go`（三域/PublishToOutbox）
- 调度：`internal/kernel/scheduler/spec.go`；Hook：`internal/kernel/transport/` 与 `internal/kernel/hooks/`
- 网关验签与转发：`internal/kernel/security/authn/hmac.go` + `internal/modules/gw/adapters/edge.go`
- WS 协议与 ACL：`internal/modules/realtime/api/{protocol,channels}` + `cmd/netyadmin/wswiring.go`
- 管理端 WS 客户端参考：`admin/src/core/ws/client.ts`

适用版本 v2.0.0

