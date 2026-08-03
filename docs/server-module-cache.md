# 缓存模块详解

本文档详细介绍 NetyAdmin 缓存模块的架构设计、A/B 双模式机制、使用方式和二次开发指南。

---

## 一、模块概述

缓存模块提供统一的缓存抽象层，支持 **A/B 双模式** 架构，根据业务场景自动选择最优存储链路。

### 1.1 核心特性

- **A/B 双模式**：极速模式（L1+L2）与标准模式（L2 only），按业务场景自动选择
- **L1/L2 二级缓存**：本地 BigCache (L1) + Redis (L2) 分层架构，兼顾性能与一致性
- **多机缓存一致性**：基于 PubSubBus 的分布式缓存失效广播，确保集群环境缓存同步
- **透明缓存**：业务层无感知切换，自动处理缓存穿透、回源逻辑
- **Tags 批量失效**：支持按标签批量清除缓存，跨机器自动同步
- **动态开关**：支持运行时开启/关闭缓存，无需重启服务
- **Key 规范**：统一的 Key 命名规范，严禁硬编码

### 1.2 A/B 双模式设计

| 模式 | 名称 | 存储链路 | 适用场景 |
|------|------|----------|----------|
| **模式A** | 极速模式 | L1 (BigCache) + L2 (Redis) + L3 (DB回源) | 开放平台 API 权限校验等每次请求都要执行的场景 |
| **模式B** | 标准模式 | L2 (Redis) + L3 (DB回源) | RBAC、字典、存储配置、内容分类、消息模板等 |

**降级规则**：

- L1 关闭时 → 模式A 自动降级为模式B（纯 Redis）
- Redis 关闭时 → 模式A 和模式B 都降级为纯 BigCache

> **注意**：IPAC（IP 访问控制）不走缓存模块，它有自有的进程内全量内存设计（`sync.RWMutex + map`），CIDR 网段匹配不适合 key→value 缓存模式。

---

## 二、目录结构

```
server/internal/pkg/cache/
├── manager.go          # 缓存管理器（LazyCacheManager），实现 ConfigCache / SecurityCache / CacheLifecycle 三个接口
└── registry.go         # Key/Tag 注册表与工厂函数
```

---

## 三、架构设计

### 3.1 引擎组合矩阵

缓存管理器内部维护两个核心缓存实例：

| 变量 | 类型 | 说明 |
|------|------|------|
| `cacheManager` | `CacheInterface[any]` | 主缓存引擎，供模式B方法使用。L1 开启时为 Chain(L1, L2)，否则为 L2 |
| `l1Cache` | `*Cache[any]` | 独立的 L1 实例，供模式A方法手动编排读写链路 |

| 配置状态 | cacheManager (模式B) | FetchFast (模式A) | 说明 |
|----------|----------------------|-------------------|------|
| Redis开 + L1开 | Chain(L1, L2) | 手动编排 L1→L2→DB | **正常模式**：Fast 手动编排 L1+L2，标准走 chain |
| Redis开 + L1关 | L2 (Redis) | 降级为 Fetch (纯 L2) | **L1 降级**：Fast 退化为标准模式 |
| Redis关 | L1 (BigCache) | 降级为 Fetch (纯 L1) | **Redis 降级**：都用本地缓存 |

> **设计说明**：模式A（FetchFast）不依赖 chain cache，而是手动编排 L1→L2→DB 的读取链路，并在 L2 命中时自动回填 L1（带 tags）。这样设计是因为 gocache 的 chain cache 在 L1 miss、L2 hit 时不会回填 L1，无法满足极速模式的需求。

### 3.2 缓存管理器接口（三接口拆分）

`*LazyCacheManager` 同时实现以下三个接口，通过 Go 的隐式接口转换自动满足。消费者按需持有对应接口，编译器强制约束可用方法集：

#### ConfigCache（6 方法，L1+L2 链路）

用于 **配置类数据**（开放平台 API 权限、RBAC、字典、存储配置、内容分类、消息模板等），走 L1 (BigCache) + L2 (Redis) 加速链路：

```go
type ConfigCache interface {
    FetchFast(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error
    SetFast(ctx context.Context, key string, value interface{}, tags []string, ttl time.Duration) error
    GetFast(ctx context.Context, key string, tags []string, ttl time.Duration, v interface{}) error
    DeleteFast(ctx context.Context, key string) error
    InvalidateByTags(ctx context.Context, tags ...string) error
    IsCacheEnabled(ctx context.Context, moduleName string) bool
}
```

#### SecurityCache（9 方法，L2 only）

用于 **安全/一次性数据**（验证码、Nonce 防重放、登录锁定、Token 黑名单等），仅走 L2 (Redis)，不需要 L1 加速：

```go
type SecurityCache interface {
    Fetch(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    Get(ctx context.Context, key string, v interface{}) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
    Incr(ctx context.Context, key string) (int64, error)
    InvalidateByTags(ctx context.Context, tags ...string) error
    IsCacheEnabled(ctx context.Context, moduleName string) bool
}
```

#### CacheLifecycle（2 方法，Wire 装配专用）

```go
type CacheLifecycle interface {
    SetEventBus(bus pubsub.EventBus)
    InvalidateL1ByTags(ctx context.Context, tags ...string) error
}
```

#### 消费者字段命名约定

| 消费者类型 | 字段名 | 持有接口 | 示例 |
|-----------|--------|----------|------|
| 配置类服务 | `cacheFast` | `ConfigCache` | dict、menu、role、api、button、message、storage config、content category |
| 安全类服务 | `cacheSlow` | `SecurityCache` | captcha store、verification、user auth、token store、error log |
| 混合消费者（admin/app service） | `cacheFast` + `cacheSlow` | `ConfigCache` + `SecurityCache` | admin service、open_platform app service |

> **注意**：
> - `GetRedisClient()` 和 `GetCacheMgr()` 已移除。`wire.go` 直接接收 `*redis.Client` 并传递给需要 Redis 的组件。
> - `RateLimit` 方法已抽离到独立的 `internal/pkg/ratelimit/` 包中。
> - 三接口拆分通过编译器强制执行 Fast/Non-Fast 方法隔离，杜绝「配置类数据误用非 Fast 方法绕过 L1 加速」的问题。
	
	### 3.3 方法与引擎对照表

| 方法 | 所属接口 | 读取链路 | 写入链路 | 说明 |
|------|----------|----------|----------|------|
| `FetchFast` | ConfigCache | 手动 L1→L2→DB | L2 + L1 回填 | L2 命中时自动回填 L1（带 tags） |
| `SetFast` | ConfigCache | — | L1 + L2 分别写入 | 支持 tags，L1 关闭时降级为 L2 写入 |
| `GetFast` | ConfigCache | 手动 L1→L2 | — | L2 命中时回填 L1（带 tags） |
| `DeleteFast` | ConfigCache | — | L1 + L2 分别删除 | 与 SetFast 配套 |
| `InvalidateByTags` | ConfigCache / SecurityCache | — | L2 + PubSub 广播 | 失效 L2 并广播，其他节点仅失效 L1 |
| `IsCacheEnabled` | ConfigCache / SecurityCache | — | — | 检查模块缓存开关 |
| `Fetch` | SecurityCache | L2→DB | L2 | 标准模式，不走 L1 |
| `Set` | SecurityCache | — | L2 | 标准写入 |
| `Get` | SecurityCache | L2 | — | 标准读取 |
| `Delete` | SecurityCache | — | L2 | 标准删除 |
| `Exists` | SecurityCache | L2 | — | 检查 Key 是否存在 |
| `SetNX` | SecurityCache | — | Redis 原子操作 | Nonce 防重放等场景 |
| `Incr` | SecurityCache | — | Redis 原子操作 | 计数器（如登录重试次数） |
| `SetEventBus` | CacheLifecycle | — | — | 注入 PubSubBus 实例 |
| `InvalidateL1ByTags` | CacheLifecycle | — | L1 only | 仅失效本地 L1（PubSub 订阅者调用） |

### 3.4 singleflight 缓存击穿保护（Task 4）

`Fetch` / `FetchFast` 内部用 `golang.org/x/sync/singleflight` 合并同一 key 的并发 loader 调用，避免缓存击穿（cache stampede）。

#### 问题背景

当某个热点 key 在 L1 + L2 同时失效的瞬间，若 100 个并发请求同时 cache miss，原实现会触发 100 次 DB 回源（loader 调用），导致 DB 瞬时压力骤增，严重时拖垮数据库。这是经典的「缓存击穿」问题。

#### 实现要点

```go
type cacheManager struct {
    // ... 其他字段
    flightGroup singleflight.Group  // 合并并发 loader 调用
}

func (m *cacheManager) Fetch(ctx, key, moduleName, tags, ttl, v, loader) error {
    // singleflight.Do 同一 key 的并发调用只执行一次 loader，其余 99 个等待结果共享
    result, err, _ := m.flightGroup.Do(key, func() (interface{}, error) {
        return loader()  // 仅 1 个请求执行 DB 回源
    })
    if err != nil { return err }
    // 写入 cacheManager（其他 99 个请求共享此结果）
    return m.cacheManager.Set(ctx, key, result, ttl)
}
```

#### 关键约定

1. **loader error 正确传播**：singleflight 的 `Do` 返回 `(result, error, shared)`，error 必须透传给所有 99 个等待请求，不吞错。若 loader 返回 error，所有等待请求都收到相同 error，不会写入缓存。
2. **`Fetch` 与 `FetchFast` 都接入 singleflight**：两个方法共享同一个 `flightGroup`，避免同一 key 在两个方法中分别触发 loader。
3. **测试覆盖**：`cache/manager_test.go` 验证 100 并发请求仅触发 1 次 loader，且 error 正确传播到所有等待请求。
4. **不适用于 `Set` / `Get` / `Delete`**：这些方法不涉及 loader 调用，无需 singleflight。
5. **`flightGroup` 是 `cacheManager` 的字段**：每个 `cacheManager` 实例独立维护一个 flightGroup，不跨实例共享。

#### 与 L1 回填的协作

`FetchFast` 中 L2 命中时回填 L1 的逻辑不受 singleflight 影响——只有 L1 + L2 都 miss 时才走 loader，此时 singleflight 生效；L2 命中直接返回不触发 loader。

---

## 四、L1/L2 二级缓存架构

### 4.1 单机模式（无 Redis）

```
┌─────────────────────────────────────┐
│           Application               │
│  ┌─────────────────────────────┐    │
│  │      L1: BigCache           │    │
│  │   (本地内存，ultra-fast)    │    │
│  └─────────────────────────────┘    │
└─────────────────────────────────────┘
```

### 4.2 多机模式（有 Redis）

```
┌─────────────────────────────────────────────────────────────┐
│                      多机部署环境                           │
│  ┌──────────────┐        Redis Cluster        ┌──────────┐  │
│  │   Machine A  │◄───────────────────────────►│  L2存储  │  │
│  │ ┌──────────┐ │        (共享缓存层)         │ (Redis)  │  │
│  │ │ L1: Big  │ │                             └──────────┘  │
│  │ │  Cache   │ │                                   ▲       │
│  │ └──────────┘ │         Pub/Sub 广播              │       │
│  └──────────────┘◄──────────────────────────────────┘       │
│         ▲                                                   │
│         │         ┌──────────────┐                          │
│         └────────►│   Machine B  │                          │
│   PubSubBus       │ ┌──────────┐ │                          │
│   缓存失效广播    │ │ L1: Big  │ │                          │
│                   │ │  Cache   │ │                          │
│                   │ └──────────┘ │                          │
│                   └──────────────┘                          │
└─────────────────────────────────────────────────────────────┘
```

### 4.3 读写流程

**读取流程**（Fetch / FetchFast）：

**模式B - Fetch**：

1. 检查模块缓存开关（`cache_switches`）
2. 尝试从 cacheManager 读取（L1 开启时为 chain，否则为 L2）→ 命中直接返回
3. Cache Miss，执行 Loader 回源数据库 → 结果写入 cacheManager（带 tags 和 TTL）
4. **注意**：当 L1 开启时，Fetch 走 chain cache 读取，但 chain 的 Get 在 L1 miss、L2 hit 时**不会回填 L1**

**模式A - FetchFast**：

1. 检查模块缓存开关（`cache_switches`）
2. 尝试从 L1 (BigCache) 读取 → 命中直接返回
3. 尝试从 L2 (Redis) 读取 → 命中则**回填 L1（带 tags）**并返回
4. L1 和 L2 都未命中，执行 Loader 回源数据库 → 结果写入 cacheManager（带 tags 和 TTL）
5. L1 关闭时自动降级为模式B（调用 Fetch）

**写入流程**（Set / SetFast）：

- 模式A (SetFast)：分别写入 L1 (BigCache) 和 L2 (Redis)，**不支持 tags**
- 模式B (Set)：写入 cacheManager（L1 开启时写入 chain，否则写入 L2）

**失效流程**（InvalidateByTags）：

1. 本地执行缓存失效（standardCache + fastCache）
2. 通过 PubSubBus 向所有机器广播失效信号
3. 其他机器收到广播后，仅失效本地 L1（避免重复失效 L2）

---

## 五、多机部署与缓存一致性

### 5.1 缓存失效广播机制

当某台机器执行 `InvalidateByTags` 时：

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Machine A   │     │  PubSubBus   │     │  Machine B   │
│              │     │  (Redis/Mem) │     │              │
│ Invalidate   │────►│  Topic:      │────►│  收到广播    │
│ ByTags(tags) │     │ cache_inval  │     │              │
│              │     │ idation      │     │ 仅失效本地L1 │
│ 两个引擎失效 │     │              │     │ (不碰L2)     │
└──────────────┘     └──────────────┘     └──────────────┘
```

**关键设计决策**：

1. **统一频道**：缓存失效通过 PubSubBus 的统一频道广播，消息体中 `topic` 字段为 `cache_invalidation`，与配置热更等 Topic 共享频道
2. **仅失效 L1**：收到广播的机器只失效本地 L1 (BigCache)，不操作 L2 (Redis)
3. **避免递归**：订阅者调用 `InvalidateL1ByTags` 而非 `InvalidateByTags`，避免无限递归
4. **幂等性**：缓存失效是幂等操作，多次执行无副作用

#### 5.1.1 InvalidateByTags 设计意图（为什么 PubSub 仅同步 L1）

`InvalidateByTags` 在本地执行时**已同时清理 L1 (BigCache) 与 L2 (Redis 共享)**。L2 是共享 Redis，一次清理对所有节点立即生效，**无需也不应该重复清理**。因此 PubSub Publish 仅通知其他节点清各自 L1（节点本地内存无法被其他节点直接清理，必须靠广播同步）。

整体设计：

| 层级 | 谁清理 | 何时清理 |
|------|--------|----------|
| **L2 (Redis 共享)** | 发起 `InvalidateByTags` 的节点本地直接清理 | 一次清理，全集群立即生效 |
| **L1 (本地 BigCache)** | 发起节点本地清理 + 其他节点收到 PubSub 广播后调 `InvalidateL1ByTags` 清理 | 异步同步 |

**反模式**：若其他节点收到广播后也清理 L2，会产生重复 Redis 删除操作（虽幂等但浪费 IO），且无法解决「L1 跨节点内存无法共享」的核心问题。正确做法是收到广播的节点仅清 L1，L2 信任发起节点已清理。

#### 5.1.2 错误传播策略（Task 14）

`InvalidateByTags` 的错误传播严格遵循「L2 失败优先 + 不扩大不一致窗口」原则：

```text
InvalidateByTags(tags)
       │
       ▼
1. cacheManager.Invalidate(tags)   ← L1 + L2 本地失效
       │
       ├── 失败 ──► 不广播 L1 跨节点失效，直接返回 error
       │            （L2 已失败意味着本节点与其他节点 L2 视图不一致，
       │             此时广播 L1 失效会让其他节点基于「L2 已清」的
       │             假设去清本地 L1，但实际 L2 可能并未清理，
       │             反而延长不一致窗口。让调用方感知失败并 slog.Error）
       │
       └── 成功 ──► 2. bus.Publish(TopicCacheInvalidation, tags)
                          │
                          ├── 失败 ──► slog.Error 上报（本地失效已成功，
                          │            跨节点 L1 失效漏掉由 TTL 兜底）
                          │
                          └── 成功 ──► return nil
```

**关键约定**：

1. **L2 失败不广播 L1**：避免在已知失败的视图上叠加广播，扩大不一致窗口。调用方收到 error 后应 `slog.Error` 上报监控（Task 14.2），由监控告警触发人工介入或 TTL 兜底。
2. **Publish 失败仅日志不返回 error**：本地 L1+L2 已清，数据正确；跨节点 L1 失效漏掉是「最终一致性延迟」，由 TTL 或下次 `InvalidateByTags` 兜底。返回 error 会让调用方误以为本地失效也失败。
3. **调用方必须 slog.Error**：所有 `InvalidateByTags(...)` 调用点必须用 `slog.Error("invalidate cache failed", ...)` 上报，**禁止用 `slog.Warn`**。缓存失效失败是数据一致性问题，应可被监控告警捕获（Task 14.2）。

```go
// ✅ 正确范式（Task 14.2）
if err := s.cacheFast.InvalidateByTags(ctx, cache.TagXxx); err != nil {
    slog.Error("invalidate cache failed", "tag", cache.TagXxx, "err", err)
}

// ❌ 反模式：禁止用 slog.Warn
// if err := s.cacheFast.InvalidateByTags(ctx, cache.TagXxx); err != nil {
//     slog.Warn("invalidate cache failed", "tag", cache.TagXxx, "err", err)  // 不可被告警捕获
// }
```

4. **Invalidate 失败不阻断主流程**：Service 层在 `tm.Commit()` 成功后调用 `InvalidateByTags`，失败仅 `slog.Error` 不返回 error（DB 已提交，缓存由 TTL 兜底自愈）。这是「DB 是 source of truth，缓存是优化」的设计原则。

#### 5.1.3 PubSub 重连 L1 兜底设计（P1-2 fix）

**背景**：PubSub `RedisDriver.subscribeLoop` 在断连恢复后会触发 `OnReconnect` 回调（Task 13.3 兜底机制）。`cache/manager.go` 的 `SetEventBus` 注册了 `reloadL1All` 作为重连回调，目的是兜底「断连期间漏收 `cache_invalidation` 广播导致本地 L1 持有 stale 条目」的场景。

**问题（thundering herd 风险）**：

原实现 `reloadL1All` 调用 `m.l1BigCache.Reset()` 全量清空 L1。这会引发 cache stampede：

1. `Reset()` 同时清空所有 L1 条目
2. 下一次读取批次会对 N 个不同 key 同时 cache miss
3. `singleflight`（§3.4）仅合并**同一 key** 上的并发 loader，**不跨 key 去重**
4. 1000 个不同 key → 1000 个并发 loader → DB 过载 → 延迟尖峰

**设计决策（Option A：保留 L1，TTL 兜底）**：

`reloadL1All` 改为 **no-op（仅 `slog.Warn` 告警，不清空 L1）**。理由：cache stampede 风险大于断连期间的短暂 staleness。

| 选项 | staleness | stampede 风险 | 复杂度 | 选择 |
|------|-----------|---------------|--------|------|
| **A. 不清空 L1（TTL 兜底）** | 短暂（受 L1 TTL 约束） | 无 | 低 | ✅ |
| B. paced-reload warming | 无 | 受控（限流回源） | 高 | 未来可选 |
| C. 清空 L1（原方案） | 无 | 高（thundering herd） | 低 | ❌ |

**staleness 边界保证**：

- L1 条目有自己的 TTL（`FetchFast`/`SetFast`/`GetFast` 通过 `store.WithExpiration` 设置），断连期间累积的 stale 条目会在 TTL 到期后自然失效
- L2 (Redis) 重连后仍是 source of truth；`FetchFast` 在 L2 命中时会自动回填 L1
- 断连期间发起方无法清理 L2，但断连恢复后第一次 `InvalidateByTags` 会同步清理 L2

**强一致性场景建议**：

| 场景 | 配置 |
|------|------|
| 禁用 L1 | `[redis] l1_enabled = false` |
| 缩短 L1 TTL | `[redis] local_ttl_min = 1`（1 分钟） |
| 走标准模式（不走 L1 回填） | 业务用 `Fetch`（模式B），不用 `FetchFast`（模式A） |

**方法保留**：`reloadL1All` 未删除，作为未来 paced-reload / warming 方案（Option B）的扩展点，无需改动 `wire.go` 的 `OnReconnect` 注册。当前 L1 默认关闭（见 `wire.go` 注释「当前 L1 关闭」），此回调在 L1 关闭时无实际作用。

### 5.2 部署建议

| 部署模式 | Redis | L1 (BigCache) | 适用场景 |
|---------|-------|---------------|---------|
| 单机开发 | 可选 | 启用 | 开发环境，快速启动 |
| 单机生产 | 建议启用 | 启用 | 小型项目，数据持久化 |
| 多机集群 | **必须** | 启用 | 中大型项目，高可用 |

**多机部署 checklist**：

- [ ] Redis 配置正确，`enabled = true`
- [ ] 所有机器使用同一个 Redis 实例/集群
- [ ] `prefix` 配置一致（避免频道隔离导致广播失效）
- [ ] 防火墙放行 Redis 端口（默认 6379）

---

## 六、配置说明

### 6.1 配置文件（config.yaml）

```yaml
redis:
  enabled: true
  host: localhost
  port: 6379
  password: ""
  db: 0
  prefix: "netyadmin"

  # L1 缓存配置
  l1_enabled: true       # L1 开关：控制模式A是否启用 L1 加速
  local_max_size_mb: 256 # L1 最大内存占用（MB）
  local_max_entry_kb: 500 # L1 单条记录最大大小（KB）
  local_ttl_min: 10      # L1 兜底 TTL（分钟），仅当 l1Cache.Set 不带 WithExpiration 时生效
```

> **语义说明**：
>
> - `l1_enabled` 仅控制模式A（Fast 方法）是否走 L1，模式B（标准方法）始终不走 L1
> - `local_ttl_min` 是 BigCache 初始化的兜底默认 TTL，正常走模式A的 Fast 方法时，L1 使用用户传入的 TTL（与 L2 一致）
> - 模式A 降级为模式B 时，TTL 完全一致（都用用户传入的 TTL），无任何行为差异

### 6.2 动态配置（sys_configs）

| 配置项 | Group | Key | 说明 |
|--------|-------|-----|------|
| RBAC 缓存开关 | cache_switches | rbac | true/false |
| 字典缓存开关 | cache_switches | dict | true/false |
| 配置缓存开关 | cache_switches | config | true/false |

---

## 七、Key 注册表规范 (Registry)

**强制规范**：严禁在业务 Service 中硬编码任何字符串作为缓存 Key 或 Tag。必须在 `internal/pkg/cache/registry.go` 中统一定义。

### 7.1 定义原则

1. **Key 函数化**：接收唯一标识（如 ID, Code），返回格式化后的 Key 字符串
2. **Tag 语义化**：Tag 用于关联一组 Key，便于批量失效

### 7.2 示例代码 (registry.go)

```go
// Key 工厂函数
func KeyAppInfo(appKey string) string { return fmt.Sprintf("open:app:info:%s", appKey) }
func KeyMsgTemplate(code string) string { return fmt.Sprintf("msg:template:%s", code) }
func KeyAppNonce(appKey, nonce string) string { return fmt.Sprintf("open:nonce:%s:%s", appKey, nonce) }
func KeyLoginLock(userID string) string { return fmt.Sprintf("user:login:lock:%s", userID) }
func KeyLoginRetryCount(userID string) string { return fmt.Sprintf("user:login:retry:%s", userID) }

// Tag 常量
const (
    TagApp         = "open:app"
    TagMsgTemplate = "msg:template"
    TagRBACMenu    = "rbac:menu"
)
```

---

## 八、使用指南

### 8.1 如何选择模式

```
需要极致速度？（每次 HTTP 请求都要校验）
  ├─ 是 → 使用 ConfigCache (cacheFast)：FetchFast / SetFast / GetFast / DeleteFast
  └─ 否（安全/一次性数据） → 使用 SecurityCache (cacheSlow)：Fetch / Set / Get / Delete / SetNX / Incr
```

### 8.2 典型场景

```go
// 场景1：开放平台 API 权限校验（每次请求都调用）→ 用 ConfigCache
s.cacheFast.FetchFast(ctx, cache.KeyAppApis(appID), "open_api", tags, ttl, &apis, loader)

// 场景2：RBAC 菜单树（登录后加载一次）→ 用 ConfigCache
s.cacheFast.FetchFast(ctx, cache.KeyMenuTree(), "rbac", tags, ttl, &tree, loader)

// 场景3：字典数据（页面加载时读取）→ 用 ConfigCache
s.cacheFast.FetchFast(ctx, cache.KeyDictData(code), "dict", tags, ttl, &list, loader)

// 场景4：验证码（一次性写入消费）→ 用 SecurityCache
s.cacheSlow.Set(ctx, cache.KeyVerificationCode("captcha", id), value, ttl)

// 场景5：Nonce 防重放（一次性校验）→ 用 SecurityCache（SetNX）
s.cacheSlow.SetNX(ctx, cache.KeyAppNonce(appKey, nonce), "1", 60*time.Second)

// 场景6：账户锁定（登录安全）→ 用 SecurityCache
s.cacheSlow.Set(ctx, cache.KeyLoginLock(userID), "1", lockDuration)
```

### 8.3 读多写少场景 (FetchFast + Tags)

```go
func (s *appService) GetAppByKey(ctx context.Context, appKey string) (*open_platform.App, error) {
    var app open_platform.App
    key := cache.KeyAppInfo(appKey)
    err := s.cacheFast.FetchFast(ctx, key, cache.TagApp, []string{cache.TagApp, cache.TagAppKey(appKey)}, 1*time.Hour, &app, func() (interface{}, error) {
        return s.repo.GetByKey(ctx, appKey)
    })
    return &app, err
}
```

### 8.4 变更失效逻辑 (Invalidate)

```go
func (s *appService) UpdateApp(ctx context.Context, app *open_platform.App) error {
    if err := s.repo.Update(ctx, app); err != nil {
        return err
    }
    // InvalidateByTags 同时失效 L1+L2，开发者不需要关心数据在哪个引擎
    return s.cacheFast.InvalidateByTags(ctx, cache.TagApp, cache.TagAppKey(app.AppKey))
}
```

### 8.5 防重放校验 (SetNX)

```go
nonceKey := cache.KeyAppNonce(appKey, nonce)
set, err := s.cacheSlow.SetNX(ctx, nonceKey, "1", 60*time.Second)
if err != nil || !set {
    return errorx.CodeSignatureFailed
}
```

---

## 九、各模块缓存使用一览

### 9.1 使用 ConfigCache（配置类，L1+L2）的模块

| 模块 | 文件 | 字段名 | 使用的方法 |
|------|------|--------|-----------|
| 开放平台应用 | `service/open_platform/app.go` | `cacheFast` | FetchFast, DeleteFast |
| 开放平台 API | `service/open_platform/api.go` | `cacheFast` | FetchFast |
| RBAC-Admin | `service/system/admin.go` | `cacheFast` | FetchFast, SetFast, InvalidateByTags |
| RBAC-Role | `service/system/role.go` | `cacheFast` | FetchFast, InvalidateByTags |
| RBAC-Menu | `service/system/menu.go` | `cacheFast` | FetchFast, InvalidateByTags |
| RBAC-API | `service/system/api.go` | `cacheFast` | InvalidateByTags |
| RBAC-Button | `service/system/button.go` | `cacheFast` | InvalidateByTags |
| 字典 | `service/dict/dict.go` | `cacheFast` | FetchFast, InvalidateByTags |
| 存储配置 | `service/storage/config.go` | `cacheFast` | FetchFast, InvalidateByTags |
| 内容分类 | `service/content/category.go` | `cacheFast` | FetchFast, InvalidateByTags |
| 消息模板 | `service/message/message.go` | `cacheFast` | FetchFast, InvalidateByTags |

### 9.2 使用 SecurityCache（安全类，L2 only）的模块

| 模块 | 文件 | 字段名 | 使用的方法 |
|------|------|--------|-----------|
| 验证码 | `pkg/captcha/store.go` | `cacheSlow` | Set, Get, Delete |
| 用户验证 | `service/user/verification.go` | `cacheSlow` | Set, Get, Delete, Exists |
| 用户认证 | `service/user/user_auth.go` | `cacheSlow` | Set, Get, Delete, Exists, Incr |
| 用户管理 | `service/user/user_admin.go` | `cacheSlow` | Set, Delete, Exists |
| Token 存储 | `service/user/token_store.go` | `cacheSlow` | Set, Get, Delete, Exists |
| 错误日志（指纹压制） | `service/log/error.go` | `cacheSlow` | SetNX |
| Admin 认证 | `service/system/admin_auth.go` | `cacheSlow` | Set, Get, Delete, Exists |

### 9.3 混合消费者（同时持有 ConfigCache + SecurityCache）

| 模块 | 文件 | 字段名 | 说明 |
|------|------|--------|------|
| Admin 服务 | `service/system/admin.go` | `cacheFast` + `cacheSlow` | 既有 RBAC 配置缓存，又有登录状态缓存 |
| 开放平台应用 | `service/open_platform/app.go` | `cacheFast` + `cacheSlow` | 既有 API 权限配置缓存，又有 Nonce 防重放 |

### 9.4 不走缓存模块的模块

| 模块 | 原因 |
|------|------|
| IP 访问控制 (IPAC) | 自有进程内全量内存设计，CIDR 网段匹配不适合 key→value 缓存 |

---

## 十、二次开发示例

### 10.1 新增模块缓存

**1. 在 registry.go 中定义 Key 和 Tag**

```go
// internal/pkg/cache/registry.go

// 新增内容模块的 Key 工厂
var (
    KeyArticleInfo  = func(id uint) string { return fmt.Sprintf("article:%d:info", id) }
    KeyArticleList  = func(page, size int) string { return fmt.Sprintf("article:list:%d:%d", page, size) }
    KeyCategoryTree = func() string { return "category:tree" }
)

// 新增 Tags
const (
    TagContent = "content"
    TagArticle = "article"
    TagCategory = "category"
)
```

**2. 在服务中使用缓存**

```go
// internal/service/content/article.go

type articleService struct {
    repo      ArticleRepository
    cacheFast cache.ConfigCache
}

func (s *articleService) GetArticle(ctx context.Context, id uint) (*entity.Article, error) {
    var result *entity.Article
    err := s.cacheFast.FetchFast(
        ctx,
        cache.KeyArticleInfo(id),
        "content",
        []string{cache.TagContent, cache.TagArticle},
        30*time.Minute,
        &result,
        func() (interface{}, error) {
            return s.repo.GetByID(ctx, id)
        },
    )
    return result, err
}

func (s *articleService) CreateArticle(ctx context.Context, article *entity.Article) error {
    if err := s.repo.Create(ctx, article); err != nil {
        return err
    }
    return s.cacheFast.InvalidateByTags(ctx, cache.TagArticle)
}
```

**3. 配置动态开关**

```sql
INSERT INTO sys_configs (group_name, key_name, value, description) VALUES
('cache_switches', 'content', 'true', '内容模块缓存开关');
```

### 10.2 新增极速缓存场景

```go
// 如：API 限流配置（每次请求都读取）
s.cacheFast.FetchFast(ctx, cache.KeyRateLimitConfig(apiID), "rate_limit", tags, ttl, &config, loader)

// 如：特征库（高频访问）
s.cacheFast.FetchFast(ctx, cache.KeyFeatureLib(libID), "feature", tags, ttl, &lib, loader)
```

---

## 十一、最佳实践

1. **合理设置 TTL**：读多写少的数据设置较长 TTL（如菜单树），频繁变更的数据设置较短 TTL
2. **使用 Tags**：为相关缓存设置相同 Tag，便于批量失效
3. **模块隔离**：不同业务模块使用不同的模块名，便于独立控制
4. **回源保护**：回源函数中做好错误处理，避免缓存穿透
5. **大对象处理**：超过 1MB 的数据建议压缩后存储
6. **Key 统一管理**：无论是模式A还是模式B，所有缓存 Key 和 Tag 必须在 `registry.go` 中统一定义，严禁硬编码
7. **避免混用接口**：同一个 Key 不要混用 ConfigCache 和 SecurityCache 的方法，避免两个引擎中存在同一 Key 的副本
8. **InvalidateByTags 优先**：变更数据后优先使用 InvalidateByTags 而非 Delete，确保缓存统一失效
9. **Fast 方法的 tags 支持**：FetchFast、SetFast、GetFast 均支持 tags 参数，确保 L1 回填和写入时 tag 关联正确，InvalidateByTags 可统一失效
10. **L1 TTL 与 L2 一致**：模式A的 Fast 方法中，L1 使用用户传入的 TTL（与 L2 完全一致），`local_ttl_min` 仅作为 BigCache 初始化的兜底默认值。降级模式B 时 TTL 无差异

---

## 十二、相关文档

- [Server 架构设计](./server-architecture.md)
- [统一消息总线详解](./server-module-pubsub.md)
- [开放平台详解](./server-module-open-platform.md)
- [用户模块详解](./server-module-user.md)
