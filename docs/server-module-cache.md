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
├── manager.go          # 缓存管理器（LazyCacheManager），A/B 双引擎实现
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

### 3.2 缓存管理器接口 (LazyCacheManager)

```go
type LazyCacheManager interface {
    // ===== 模式B（标准模式）: L2 Redis + L3 DB =====
    // 适合：RBAC、字典、存储配置、内容分类、消息模板等
    // L1 开启时走 chain(L1,L2) 读取，但 L2 命中不回填 L1
    // 需要自动回填 L1 请使用 FetchFast（模式A）
    Fetch(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    Get(ctx context.Context, key string, v interface{}) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)

    // ===== 模式A（极速模式）: L1 本地 + L2 Redis + L3 DB =====
    // 适合：开放平台 API 权限等每次请求都要校验的场景
    // 手动编排 L1→L2→DB 链路，L2 命中时自动回填 L1（带 tags）
    // L1 关闭时自动降级为模式B（纯 L2）
    FetchFast(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error
    // SetFast 写入 L1+L2，支持 tags，L1 关闭时降级为 cacheManager 写入
    SetFast(ctx context.Context, key string, value interface{}, tags []string, ttl time.Duration) error
    // GetFast 读取 L1→L2，L2 命中回填 L1 时带 tags
    // ttl 用于计算 L1 回填过期时间：min(ttl, local_ttl_min)
    GetFast(ctx context.Context, key string, tags []string, ttl time.Duration, v interface{}) error
    // DeleteFast 删除 L1+L2
    DeleteFast(ctx context.Context, key string) error

    // ===== 共用方法 =====
    // InvalidateByTags 失效 cacheManager 并通过 PubSub 广播，其他节点仅失效 L1
    InvalidateByTags(ctx context.Context, tags ...string) error

    // InvalidateL1ByTags 仅失效本地 L1 缓存（由 PubSubBus 订阅者调用，避免递归）
    InvalidateL1ByTags(ctx context.Context, tags ...string) error

    // SetNX 原子性写入（走 Redis 原生 NX，Nonce 防重放等场景不需要 L1）
    SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)

    // RateLimit 分布式/单机自适应限流（Redis Lua 脚本或本地令牌桶）
    RateLimit(ctx context.Context, key string, rate int, capacity int) (bool, error)

    // SetEventBus 注入 PubSubBus 实例
    SetEventBus(bus pubsub.EventBus)

    // GetRedisClient 获取底层 Redis 客户端
    GetRedisClient() *redis.Client
}
```

### 3.3 方法与引擎对照表

| 方法 | 读取链路 | 写入链路 | 说明 |
|------|----------|----------|------|
| `Fetch` | cacheManager (chain/L2) | cacheManager | 模式B：L2(Redis) only，L1 开启时走 chain 但 L2 命中不回填 L1 |
| `Set` | — | cacheManager | 模式B：写入 cacheManager |
| `Get` | cacheManager | — | 模式B |
| `Delete` | — | cacheManager | 模式B |
| `Exists` | cacheManager | — | 模式B |
| `SetNX` | — | Redis 原子操作 | 模式B（Nonce 防重放等场景不需要 L1） |
| `FetchFast` | 手动 L1→L2→DB | cacheManager + L1 回填 | 模式A：L2 命中时自动回填 L1（带 tags） |
| `SetFast` | — | L1 + L2 分别写入 | 模式A：支持 tags，L1 关闭时降级为 cacheManager 写入 |
| `GetFast` | 手动 L1→L2 | — | 模式A：L2 命中时回填 L1（带 tags） |
| `DeleteFast` | — | L1 + L2 分别删除 | 模式A |
| `InvalidateByTags` | — | cacheManager + PubSub 广播 | 失效 cacheManager 并广播，其他节点仅失效 L1 |
| `RateLimit` | — | Redis Lua / 本地令牌桶 | 不走缓存引擎，直接用 Redis Lua 脚本或本地令牌桶 |

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

### 6.1 配置文件（config.toml）

```toml
[redis]
enabled = true
host = "localhost"
port = 6379
password = ""
db = 0
prefix = "netyadmin"

# L1 缓存配置
l1_enabled = true       # L1 开关：控制模式A是否启用 L1 加速
local_max_size_mb = 256 # L1 最大内存占用（MB）
local_max_entry_kb = 500 # L1 单条记录最大大小（KB）
local_ttl_min = 10      # L1 兜底 TTL（分钟），仅当 l1Cache.Set 不带 WithExpiration 时生效
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
  ├─ 是 → 用 FetchFast / SetFast / GetFast / DeleteFast
  └─ 否 → 用 Fetch / Set / Get / Delete
```

### 8.2 典型场景

```go
// 场景1：开放平台 API 权限校验（每次请求都调用）→ 用 Fast
s.cacheMgr.FetchFast(ctx, cache.KeyAppApis(appID), "open_api", tags, ttl, &apis, loader)

// 场景2：RBAC 菜单树（登录后加载一次）→ 用标准
s.cacheMgr.Fetch(ctx, cache.KeyMenuTree(), "rbac", tags, ttl, &tree, loader)

// 场景3：字典数据（页面加载时读取）→ 用标准
s.cacheMgr.Fetch(ctx, cache.KeyDictData(code), "dict", tags, ttl, &list, loader)

// 场景4：验证码（一次性写入消费）→ 用标准
s.cacheMgr.Set(ctx, cache.KeyVerificationCode("captcha", id), value, ttl)

// 场景5：Nonce 防重放（一次性校验）→ 用标准（SetNX）
s.cacheMgr.SetNX(ctx, cache.KeyAppNonce(appKey, nonce), "1", 60*time.Second)

// 场景6：账户锁定（登录安全）→ 用标准
s.cacheMgr.Set(ctx, cache.KeyLoginLock(userID), "1", lockDuration)
```

### 8.3 读多写少场景 (Fetch + Tags)

```go
func (s *appService) GetAppByKey(ctx context.Context, appKey string) (*open_platform.App, error) {
    var app open_platform.App
    key := cache.KeyAppInfo(appKey)
    err := s.cacheMgr.FetchFast(ctx, key, cache.TagApp, []string{cache.TagApp, cache.TagAppKey(appKey)}, 1*time.Hour, &app, func() (interface{}, error) {
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
    // InvalidateByTags 同时失效两个引擎，开发者不需要关心数据在哪个引擎
    return s.cacheMgr.InvalidateByTags(ctx, cache.TagApp, cache.TagAppKey(app.AppKey))
}
```

### 8.5 防重放校验 (SetNX)

```go
nonceKey := cache.KeyAppNonce(appKey, nonce)
set, err := s.cacheMgr.SetNX(ctx, nonceKey, "1", 60*time.Second)
if err != nil || !set {
    return errorx.CodeSignatureFailed
}
```

---

## 九、各模块缓存使用一览

### 9.1 使用极速模式（模式A）的模块

| 模块 | 文件 | 使用的方法 |
|------|------|-----------|
| 开放平台应用 | `service/open_platform/app.go` | FetchFast, DeleteFast |
| 开放平台 API | `service/open_platform/api.go` | FetchFast |

### 9.2 使用标准模式（模式B）的模块

| 模块 | 文件 | 使用的方法 |
|------|------|-----------|
| RBAC-Admin | `service/system/admin.go` | Fetch, Set, Exists, InvalidateByTags |
| RBAC-Role | `service/system/role.go` | Fetch, InvalidateByTags |
| RBAC-Menu | `service/system/menu.go` | Fetch, InvalidateByTags |
| RBAC-API | `service/system/api.go` | InvalidateByTags |
| RBAC-Button | `service/system/button.go` | InvalidateByTags |
| 字典 | `service/dict/dict.go` | Fetch, InvalidateByTags |
| 存储配置 | `service/storage/config.go` | Fetch, InvalidateByTags |
| 内容分类 | `service/content/category.go` | Fetch, InvalidateByTags |
| 消息模板 | `service/message/message.go` | Fetch, InvalidateByTags |
| 验证码 | `pkg/captcha/store.go` | Set, Get, Delete |
| 用户验证 | `service/user/verification.go` | Set, Get, Delete, Exists |
| 用户锁定 | `service/user/user.go` | Set, Get, Delete, Exists |

### 9.3 不走缓存模块的模块

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
    repo         ArticleRepository
    cacheManager cache.LazyCacheManager
}

func (s *articleService) GetArticle(ctx context.Context, id uint) (*entity.Article, error) {
    var result *entity.Article
    err := s.cacheManager.Fetch(
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
    return s.cacheManager.InvalidateByTags(ctx, cache.TagArticle)
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
s.cacheManager.FetchFast(ctx, cache.KeyRateLimitConfig(apiID), "rate_limit", tags, ttl, &config, loader)

// 如：特征库（高频访问）
s.cacheManager.FetchFast(ctx, cache.KeyFeatureLib(libID), "feature", tags, ttl, &lib, loader)
```

---

## 十一、最佳实践

1. **合理设置 TTL**：读多写少的数据设置较长 TTL（如菜单树），频繁变更的数据设置较短 TTL
2. **使用 Tags**：为相关缓存设置相同 Tag，便于批量失效
3. **模块隔离**：不同业务模块使用不同的模块名，便于独立控制
4. **回源保护**：回源函数中做好错误处理，避免缓存穿透
5. **大对象处理**：超过 1MB 的数据建议压缩后存储
6. **Key 统一管理**：无论是模式A还是模式B，所有缓存 Key 和 Tag 必须在 `registry.go` 中统一定义，严禁硬编码
7. **避免混用模式**：同一个 Key 不要混用 Fetch 和 FetchFast，避免两个引擎中存在同一 Key 的副本
8. **InvalidateByTags 优先**：变更数据后优先使用 InvalidateByTags 而非 Delete，确保缓存统一失效
9. **Fast 方法的 tags 支持**：FetchFast、SetFast、GetFast 均支持 tags 参数，确保 L1 回填和写入时 tag 关联正确，InvalidateByTags 可统一失效
10. **L1 TTL 与 L2 一致**：模式A的 Fast 方法中，L1 使用用户传入的 TTL（与 L2 完全一致），`local_ttl_min` 仅作为 BigCache 初始化的兜底默认值。降级模式B 时 TTL 无差异

---

## 十二、相关文档

- [Server 架构设计](./server-architecture.md)
- [统一消息总线详解](./server-module-pubsub.md)
- [开放平台详解](./server-module-open-platform.md)
- [用户模块详解](./server-module-user.md)
