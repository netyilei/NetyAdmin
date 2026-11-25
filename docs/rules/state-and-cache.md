# 状态子系统：缓存层级与降级策略（V2）

> 版本：v1.0 · 2026-11-11
> 单一事实源：`contracts/state-policies.yaml`
> 范围：缓存（L1/L2/DB）、协调原语（锁/限流/Nonce/幂等/选主）、会话（Token/Session）、消息总线降级
> 结论：缓存不采用双接口 + 自动归一化；拆成**三个平面**，策略显式声明、启动期解析、运行期只降级不改造。

---

## 1. V2 原则

1. **DB 是唯一真相源**；缓存永远不是权威数据。
2. **按失败语义分平面**：缓存（性能）/ 协调（正确性）/ 会话（安全）/ 总线（消息）。不同平面不同策略。
3. **策略显式**：每个平面/原语在 `contracts/state-policies.yaml` 声明 `on_unavailable`，代码不写隐式回退。
4. **启动期解析，运行期只降级**：模式在启动时解析一次并公开；运行期只允许 `NORMAL → DEGRADED → NORMAL` 的健康状态变化，不重新解释配置。
5. **多机正确性优先**：本地回退只在 `topology=single` 允许；多机下协调原语缺失必须 fail-closed 或显式降级（带告警）。
6. **模块自治**：每个模块在 `api/keys` 定义自己的 key/tag；缓存策略在 Manifest 声明；禁止跨模块读数据 key。
7. **可观测**：hit/miss、降级、失效、stale、回退都有指标与诊断。
8. **可测试**：故障注入矩阵 + profile 矩阵 + 并发/竞态测试。

---

## 2. 四个平面

| 平面 | 目的 | 典型内容 | 可用性要求 | 失败策略 |
|------|------|---------|-----------|---------|
| **Cache** | 性能 | 配置、RBAC、内容、字典（已删）、存储配置、模板 | 可选 | 回源 DB / 跳过缓存 |
| **Coordination** | 正确性 | 分布式锁、限流、Nonce、幂等、选主、计数器 | 按原语声明 | fail-open / fail-closed / 单机 local |
| **Session** | 安全 | Token 吊销、会话、刷新轮换、登录状态 | 必需 | DB 回退 / fail-closed |
| **Bus** | 消息 | 缓存失效广播、配置热更、IPAC 重载 | 多机必需 | 单机 memory / 多机 fail-fast 或 TTL 兜底 |

> 关键：**Cache 平面不应承载安全/正确性数据**——四个平面严格分离，不做混合缓存。

---

## 3. 模式解析

### 3.1 配置

```yaml
deployment:
  topology: single | multi        # 必填；不再从 multi_node 猜测

state:
  mode: strict | auto            # 生产默认 strict；auto 仅用于开发/演示
  cache:
    l1: { max_mb: 128, per_entity_max_mb: 32, max_entry_kb: 512 }  # 常开，无 enabled 开关
    l2: { enabled: true }         # 多机必开；单机可选（单机无 Redis 时 L1→DB）
    default_consistency: event    # event | strong | bounded | bypass
    default_class: shared         # hot_local | shared | durable
  coordination:
    backend: redis
  session:
    backend: redis
  bus:
    driver: redis | memory
```

### 3.2 strict 模式（默认）

启动期校验，不合法直接失败并给出修正建议：

| 组合 | 结果 |
|------|------|
| `topology=multi` 且 L2 关闭 | 失败：多机必须启用 L2（跨节点失效/协调）；建议 `l2.enabled=true` 或 `topology=single` |
| `topology=multi` 且 `bus.driver=memory` | 失败：memory 总线不出进程 |
| 旧配置含 `l1_enabled=false` | 失败：L1 常开、无开关；删除该字段 |
| `topology=multi` 且 Redis 启动不可达 | 失败：协调/会话不可保证 |
| `topology=single` 且 Redis 不可达 | 允许：L1-only，协调按 §5 单机回退 |

### 3.3 auto 模式（仅开发）

解析规则（多机自动 L2、多机 bus 自动 redis；L1 常开、不参与归一化）：

- 只在启动期执行一次；
- 解析结果写入日志与 `/diagnostics/state`；
- 运行期不改变配置语义；
- 生产环境禁止 auto（启动警告；生产 config 校验失败）。

### 3.4 启动健康检查与诊断

```
GET /admin/v1/system/diagnostics/state
{
  "topology": "multi",
  "mode": "strict",
  "cache": {"l1": "on", "l2": "on", "consistency": "event"},
  "coordination": {"backend": "redis", "degraded": false},
  "session": {"backend": "redis", "degraded": false},
  "bus": {"driver": "redis", "connected": true},
  "redis": {"connected": true, "latency_ms": 1},
  "degraded_features": []
}
```

健康状态：`NORMAL` / `L2_DEGRADED` / `L1_DEGRADED` / `CACHE_BYPASS` / `COORDINATION_DEGRADED` / `SESSION_DEGRADED` / `BUS_DEGRADED`。状态变化打日志 + 指标；恢复自动。

---

## 4. Cache 平面

### 4.1 层级

```
L0  request-local（可选，单次请求内去重）
L1  process memory（BigCache）
L2  Redis（共享）
L3  DB（source of truth）
```

- **L1 常开，无开关**；只承载 `hot_local` 实体；`shared`/`durable` 即使调用 cache API 也跳过 L1。
- L2：多机必开；单机可选（单机无 Redis 时 L1→DB）。
- L3：DB，唯一真相源。
- `strong` 一致性跳过 L1；`bounded` 只用 TTL；`bypass`/`durable` 直接 DB。
- L1 只用于 Cache 平面；Coordination/Session 永远不碰 L1（不做 L2→L1 降级）。
- **运行装配**（cmd/netyadmin）：L2 在场与 coordination 同源判定（`redis.addr` 配置且探活可用——无独立 `enabled` 开关）；coord=nil（single 无 Redis）按 `single_optional` 装配 L1+loader 直读 DB；multi 缺 Redis 由 db-connect 段 fail-fast 兜底。Cache 面经 Registry Services 以 `kernel.Cache` Expose，下游模块 `ResolveAs[*cache.Store]` 消费并 `Register` 自己的 entity。

### 4.1.1 分级与准入（替代手工选层）

开发者**只声明 Entity 的 `class`**，由平台映射层级；禁止手工选 L1/L2/L3。

| class | 层级 | 适用 | 默认 |
|-------|------|------|------|
| `hot_local` | L1+L2+DB | 小、全局共享、每请求读、读多写少、可声明 max_staleness | 否 |
| `shared` | L2+DB | 跨节点共享、高基数/中大 value/中短 TTL | 是 |
| `durable` | DB | 权威/事务/强一致/复杂查询 | — |

`hot_local` 准入（CI 校验，全部满足）：

- 每节点 QPS ≥ 500 或请求占比 ≥ 50%；
- value ≤ 64KB；每节点 working set ≤ 64MB；单 entity L1 quota ≤ 32MB；
- 读写比 ≥ 10:1；
- 声明 `max_staleness`，且 TTL ≤ max_staleness；
- `invalidation=generation`，且有明确失效事件；
- `classification != pii`；非 security/session/coordination；
- 在平台白名单内：RBAC 权限快照、gateway App/Scope 白名单、config/feature flags、route metadata、公钥/证书、高频模板/存储配置。

> CI 门禁：`scripts/ci/check-cache-admission.mjs`（run.mjs 常驻）校验契约完整性、代码镜像
> （白名单/值上限/auto_demote 数值）与生产 `hot_local` EntityConfig 声明字段完整性；
> QPS/请求占比/读写比/working set 属容量评审输入（无代码载体，由 Review 承担）。

判定顺序（开发者与 CI 统一使用）：

1. 权威/事务/强一致/复杂查询 → `durable`（DB）。
2. 安全/会话/协调 → 对应平面，**永不 L1**。
3. 非读多写少，或高基数/大 value → `shared`（L2）。
4. 满足 `hot_local` 全部准入 → `hot_local`（L1+L2）。
5. 其余 → `shared`。

自动降级：L1 命中率 < 70% 或 QPS < 100 持续 10 分钟 → 自动降为 `shared`（L2）并告警；**不自动升级**，升级需评审。L1 常开不等于“所有 entity 都进 L1”。实现：`state/cache/demote.go`（命中率档累计口径；QPS 档 10m 分桶滚动窗口，观测未满窗不判定——防启动即降级；两档同节拍=每 512 次 miss 评估）。

### 4.2 类型化 API

```go
type Entity[V any] struct {
    Name         string        // 模块内唯一，如 "app"
    Class        Class         // hot_local | shared | durable（默认 shared）
    Key          func(args ...string) string // 来自 api/keys
    TTL          time.Duration
    MaxStaleness time.Duration // hot_local 必填；TTL ≤ MaxStaleness
    NegativeTTL  time.Duration // 0 = 不缓存 NotFound
    Consistency  Consistency   // event | strong | bounded | bypass
    Invalidation Invalidation  // generation | exact | ttl_only
    Classification DataClass   // public | internal | confidential | pii
    MaxValueSize int
}

func GetOrLoad[V any](ctx context.Context, e Entity[V], keyArgs []string, loader func(context.Context) (V, error)) (V, error)
func Invalidate(ctx context.Context, e Entity[any]) error
func Delete(ctx context.Context, e Entity[any], keyArgs ...string) error
```

- `GetOrLoad` 统一实现：L1→L2→loader→写回；singleflight；负缓存；jitter；世代校验。
- 业务只持有 `cache.Port`（接口），不直接碰 Redis/BigCache；适配器在 `adapters/cache`。
- Key/Tag 由模块 `api/keys` 生成（见命名规范）。

### 4.3 一致性模型

| 模型 | L1 | 失效 | 适用 |
|------|----|------|------|
| `strong` | 跳过 | 精确删除 L2 | 会话外的高敏配置；读少写多 |
| `event` | 使用 | 提交后失效 + 世代广播 | 配置/RBAC/内容（默认） |
| `bounded` | 使用 | 只靠 TTL | 统计/非关键数据 |
| `bypass` | 不用 | 不缓存 | 强一致读、调试 |

### 4.4 失效：世代（generation）优先

V2 默认使用**世代号**而不是批量删 key。L2 键命名空间由装配注入（禁止硬编码 root/env）：

```
<root>:<env>:cache:<entity>:<key>     # 值键（信封携带 generation——§4.7）；如 na:dev:cache:rbac.permissions:u1
<root>:<env>:cache:gen:<entity>       # 世代计数键；如 na:dev:cache:gen:rbac.permissions
```

- 键格式登记 `contracts/namespaces.yaml kernel_plane_keys`；namespace（`<root>:<env>:cache`）由 `cmd/netyadmin cacheNamespace` 注入（`cfg.Redis.Prefix + env`），键构造唯一入口在 `internal/kernel/state/cache` Store（session/coordination 同惯例）。
- 每个 entity 有一个 generation（L2 计数键 + L1 本地快照）。
- 失效 = `INCR generation`（L2）；跨节点传播当前为**拉收敛**——各节点读取时经 genFor 快照窗口（1s）拉取对齐，漂移计数 `na_cache_generation_drift_total`；总线推送广播（`platform.cache.generation.changed`）随多机部署批升级（已在路线图登记），窗口内脏读由 TTL 兜底。
- 读：比较条目 generation 与当前 generation；旧条目视为 miss（不必立即删除，TTL 自然回收）。
- 写回：loader 开始前记录 generation；写回前若 generation 已变，丢弃结果——**解决"失效前开始、失效后写回"的竞态**。
- 精确删除（`Delete`）仍可用，用于单条数据变更。
- 大批量失效（菜单树、分类树）用 generation，避免 SMEMBERS+DEL 大量 key。
- 代价：每次读需当前 generation；L1 本地 map 命中时几乎零成本；未命中时读 L2 一次（可短 TTL 本地缓存）。

### 4.5 读/写流程

**读（GetOrLoad）**

1. `strong` 跳过 L1；否则 L1 hit → 校验 generation → 返回。
2. L2 hit → 校验 generation → 回填 L1（带 generation）→ 返回。
3. miss → singleflight 合并同 key 的 loader。
4. loader 返回 NotFound → 写负缓存（`NegativeTTL`）。
5. loader 成功 → 记录 generation 快照；写 L2（含 generation/TTL/jitter）；写 L1（可选）。
6. 写回前 generation 变化 → 丢弃写回，返回新值但不缓存（下次读会重新加载）。

**写/失效（提交后）**

```
DB commit → OnCommit → cache.Invalidate(entity)
  1. INCR generation (L2)
  2. 本地 L1 标记（无需逐条删除）
  3. 跨节点：拉收敛——其他节点读取时 genFor 快照窗口（1s）拉到新 generation
     （总线推送广播随多机部署批升级）
  4. 旧 L1 条目在读取时被 generation 校验淘汰
```

错误策略：

- generation 递增失败 → 返回 error，不广播；调用方 `slog.Error`；旧数据由 TTL 兜底。
- 广播失败 → 本地已生效，其他节点由 TTL 兜底；`slog.Error` + 指标。
- L2 读取失败 → 按 entity `OnError`：`bypass`（回源 DB，默认）或 `fail`（strong）。
- L1 写失败 → 降级 L2 only，Warn + 指标。

### 4.6 防击穿/雪崩/穿透

| 问题 | 方案 |
|------|------|
| 击穿 | singleflight（同 key 合并）；`stale-while-revalidate` 可选 |
| 雪崩 | TTL jitter（±10%）；世代失效不批量清 L1 |
| 穿透 | 负缓存（NotFound，短 TTL）；可选布隆过滤器 |
| 热 key | L1 命中；可选本地副本/请求合并；监控 top keys |
| 大 value | `MaxValueSize` 限制；超限不缓存 + 指标 |
| 脏数据 | 解码失败视为 miss 并删除；schema 版本 |

### 4.7 PII 与序列化

- 信封：`{"v":1,"gen":123,"data":...}`；JSON 默认，可换 msgpack 适配器。
- `classification=pii`：写入 L2 前用 kernel PII AES-GCM 加密；L1 可保留明文（进程内存）或同样加密（按策略）。
  - 实装：`EntityConfig.Classification`（public/internal/confidential/pii，默认 internal）；
    pii 实体 L2 值 AES-256-GCM 加密（密文 base64 入信封 + `enc=true`），L1 恒明文
    （contracts `encrypt_l1: false`）；解密失败/密钥不符视为 miss（自然回填）——`state/cache/l2_encrypt.go`。
  - 密钥：env-only `NETYADMIN_CACHE_L2_KEY`（base64 32 字节，装配注入 `cache.WithL2EncryptKey`）；
    **缺失=pii 实体注册失败（fail-closed）**，非 pii 实体不受影响。hot_local 禁止 pii（admission）。
- 禁止在 key/tag 中出现 PII；禁止缓存密码/密钥/完整证件号。
- 缓存值包含 `schema_version`；不兼容变更升版本，旧条目视为 miss。

### 4.8 模块集成

Manifest 示例：

```yaml
state:
  cache:
    entities:
      - name: app
        class: hot_local
        key: gateway.app
        ttl: 5m
        max_staleness: 10s
        consistency: event
        invalidation: generation
        negative_ttl: 30s
        classification: internal
      - name: menu_tree
        class: hot_local
        key: rbac.menu_tree
        ttl: 10m
        max_staleness: 30s
        consistency: event
        invalidation: generation
      - name: user_profile
        class: shared
        key: identity.user_profile
        ttl: 5m
        consistency: event
        invalidation: exact
        classification: confidential
```

- `class` 必填；**禁止 `level` 手工选层**。
- `max_staleness` 仅 `hot_local` 必填，且 `TTL ≤ max_staleness`。
- `key` 指向 `api/keys` 的 builder；Manifest 只引用，不写格式。
- 未声明的 entity 不得使用缓存（Registry 校验）。

---

## 5. Coordination 平面

### 5.1 原语

| 原语 | 接口 | 典型用途 |
|------|------|---------|
| Lock | `Acquire(ctx,key,ttl) (release, error)` | 防重入、单飞任务 |
| RateLimit | `Allow(ctx,policy,dimension) (Decision,error)` | 登录/API/网关配额 |
| Nonce | `Use(ctx,scope,nonce,ttl) (bool,error)` | 防重放 |
| Idempotency | `Begin(ctx,key,ttl) (Result,error)` | 幂等写 |
| Leader | `Campaign(ctx,group) (Leader,error)` | 调度选主 |
| Counter | `Incr(ctx,key,ttl) (int64,error)` | 登录失败计数 |

### 5.2 策略（显式声明）

`contracts/state-policies.yaml` 定义每个原语在 `single` / `multi` 下的行为：

| 原语 | single + Redis 不可用 | multi + Redis 不可用 | 说明 |
|------|----------------------|----------------------|------|
| Lock | local mutex | fail-closed | 多机本地锁无意义 |
| RateLimit(login) | fail-open | fail-open | 登录入口不能被限流器阻断 |
| RateLimit(api) | fail-open | fail-open | 保护层，不阻断业务 |
| RateLimit(gateway quota) | fail-closed | fail-closed | 配额是商业约束 |
| Nonce | local SetNX | fail-closed | 多机防重放必须精确 |
| Idempotency | local/DB | fail-closed | 不能重复扣款/发货 |
| Leader | local single | fail-closed 或 PG 租约 | 任务选主 |
| Counter | local counter | fail-closed | 跨节点计数不能失真 |

规则：

- 本地回退只在 `topology=single` 允许；多机禁止（配置校验 + 运行时检查）。
- 每个原语必须声明 `on_unavailable`，不允许代码里隐式判断。
- 回退触发时打指标 `coordination_fallback_total{primitive,topology}` + 限频日志。

## 6. Session 平面

| 能力 | 正常 | Redis 不可用（single） | Redis 不可用（multi） |
|------|------|----------------------|----------------------|
| JWT 校验 | 本地公钥 | 本地公钥（无需 Redis） | 本地公钥 |
| Token 吊销检查 | Redis/短 TTL | DB 回退（可配置） | fail-closed（默认） |
| Refresh 轮换 | Redis 原子 | fail-closed | fail-closed |
| 登录状态 | Redis | DB 回退或 fail-closed | fail-closed |
| 验证码 | Redis/DB | DB/内存（单机） | fail-closed 或 DB |

- JWT 校验本身不依赖 Redis；只有吊销/轮换/会话状态依赖。
- `token_version`（用户级/端级）缓存短 TTL；读缓存失败 → DB 回退（若策略允许）或 fail-closed。
- 安全默认 fail-closed；任何"跳过吊销检查"的降级必须显式配置并记录安全审计。

---

## 7. Bus 平面

- `topology=multi` 必须 `bus.driver=redis`；strict 模式启动失败，auto 模式自动切换并告警（auto 模式保留自动切换行为）。
- 运行期 Redis 断开：
  - 缓存失效广播丢失 → 其他节点 L1 由 generation/TTL 兜底；`state=BUS_DEGRADED`。
  - 配置热更/IPAC 重载延迟 → 指标告警；恢复后主动 `ForceReload`。
- `topology=single` 可用 memory；无跨节点语义。

---

## 8. 降级矩阵（总表）

| 能力 | 正常 | Redis 不可用·单机 | Redis 不可用·多机 | L1 内存压力 | 缓存关闭/durable |
|------|------|------------------|------------------|------------|------------------|
| 配置缓存 | L1→L2→DB | L1→DB | L1→DB（stale 有界）或 fail-fast | BigCache 容量淘汰（`l1.max_mb`=128MB/node）后回源 L2/DB；单条 >`max_entry_kb`(512KB) 不入 L1 | durable 类直读 DB（levels=[l3]，零 L1/L2） |
| 强一致缓存 | L2→DB | DB 或 fail | fail-closed | 不涉 L1（strong.l1=false） | DB |
| bounded 缓存（TTL-only） | L1→L2→DB | L1→DB | L1→DB（stale 有界） | 淘汰旧条目，回源 L2/DB | DB |
| 登录限流 | Redis | fail-open | fail-open | 不涉 L1 | 不变 |
| API 限流 | Redis | fail-open | fail-open | 不涉 L1 | 不变 |
| 网关配额 | Redis | fail-closed | fail-closed | 不涉 L1 | 不变 |
| Nonce | Redis | local SetNX | fail-closed | 不涉 L1 | 不变 |
| 锁 | Redis | local mutex | fail-closed | 不涉 L1 | 不变 |
| 幂等 | Redis | local/DB | fail-closed | 不涉 L1 | 不变 |
| 选主 | Redis | local | fail-closed/PG | 不涉 L1 | 不变 |
| 计数（counter） | Redis INCR（首设固定窗口 TTL） | local | fail-closed | 不涉 L1 | 不变 |
| 登录验证码 | Redis GETDEL 一次性 | 进程内存储（TTL 3min、容量 10k 满=拒绝） | fail-closed | 不涉 L1 | 不变 |
| Token 吊销 | Redis | DB 回退/fail | fail-closed | 不涉 L1 | 不变 |
| 会话 | Redis | DB/fail | fail-closed | 不涉 L1 | 不变 |
| 失效广播 | Redis | memory | degraded + TTL | generation/TTL | 不变 |
| 总线（事件投递） | Redis Streams（multi）/ memory（single） | local memory | fail-fast（启动拒绝） | 不涉 L1 | 不适用（DLQ 需 DB；no-DB 形态=丢弃计数） |

> 单元格的"fail-open/fail-closed/local/DB"必须与 `contracts/state-policies.yaml` 一致；文档与配置由 CI 校验（`check-contracts-mirror.mjs` 覆盖 coordination/rate_limit/session 三段；cache 段由 `check-cache-admission.mjs` 校验）。

> **计数部分失败**：`IncrCounter` 为 INCR→PEXPIRE 两步非原子——真抖动下 INCR 成功、PEXPIRE 失败时错误如实上抛（调用方可见，不静默），但计数键残留无 TTL。调用方必须按错误重试（不做静默吞掉）；原子化改造（Lua 原子 INCR+PEXPIRE，与 `IncrSequence` 同形）留待 counter 首个生产消费方（当前无生产消费方）。

> **只读副本探针**：`/health/ready` 的 `pg_replica` 为降级型探针（`health.RegisterDegradable`）——副本故障明细记 `degraded:` 不翻转 ready（写面仍可用，不摘全流量）；读面回退主库/报错由读消费方决定（`Cluster.Read` 使用方）。

---

## 9. 可观测性

| 指标 | 说明 |
|------|------|
| `cache_requests_total{module,entity,level,result}` | hit/miss/error/bypass |
| `cache_latency_seconds{level}` | L1/L2/loader |
| `cache_stale_served_total{entity}` | 世代过期但仍被读到的次数 |
| `cache_invalidation_total{entity,result}` | 失效成功/失败 |
| `cache_generation{entity}` | 当前世代 |
| `coordination_fallback_total{primitive,topology}` | 本地/降级回退次数 |
| `state_mode_info{mode,topology}` | 解析后的模式 |
| `state_degraded{component}` | 降级状态 |
| `session_db_fallback_total` | 会话 DB 回退 |
| `bus_degraded_seconds` | 总线降级时长 |

诊断端点：`/admin/v1/system/diagnostics/state`（模式、组件状态、各模块 entity 策略、noop/回退列表）。

---

## 10. 验证要点

1. **模式矩阵**：single/multi × L1/L2 开关 × Redis 可达/不可达 × strict/auto。
2. **故障注入**：启动时 Redis 不可达；运行中断连/恢复；L2 写失败；广播丢失；L1 损坏。
3. **并发**：100 并发同 key 只回源一次；失效与回填竞态（generation guard）；Delete 与 Get 竞态。
4. **一致性**：节点 A 写→失效→节点 B 读；L1 旧值在 generation 变化后被判定 miss；TTL 兜底。
5. **降级语义**：每行降级矩阵都有测试；fail-open/fail-closed 断言。
6. **性能**：L1 命中 < 1ms；L2 命中 p99 < 10ms（同机房）；lean profile 无多余 goroutine/连接。
7. **安全**：PII 加密落 L2；key/tag 无 PII；吊销 fail-closed。

---

## 11. 反模式

- ❌ 把 L1 当通用缓存；未声明 `class` 就用 L1。
- ❌ 手工选层（`level`）而非声明 `class`。
- ❌ L1 承载安全/会话/协调/PII。
- ❌ 无命中率/容量指标就默认 L1。
- ❌ 用缓存承载安全状态（token/验证码/Nonce）并允许 L1 回退。
- ❌ 多机 + Redis 不可用时本地回退限流/Nonce/锁。
- ❌ 缓存作为 source of truth；DB 提交后不失效。
- ❌ 失效只删 L1 不删 L2 / 先 L1 后 L2（会回填旧值）。
- ❌ 无 generation 的"删了又被 loader 写回"。
- ❌ 无 TTL、无 jitter、无负缓存。
- ❌ 跨模块直接读数据 key。
- ❌ PII 明文落共享 Redis。
- ❌ 运行期静默改变模式。
