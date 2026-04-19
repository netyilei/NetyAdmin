# 缓存模块 A/B 双模式升级改造文档

> 本文档是缓存模块从"单一引擎"升级为"A/B 双模式"的完整改造指南。
> 新会话 AI 必须严格按照本文档执行，不可遗漏任何步骤。

---

## 一、改造目标

### 1.1 现状问题

当前缓存模块只有一个引擎实例 `cacheManager`，所有业务共享同一个存储链路：

- `L1Enabled=true + Redis=true` → 所有业务都走 `Chain(BigCache, Redis)` → L1 被所有业务共享
- `L1Enabled=false + Redis=true` → 所有业务都走纯 Redis

**问题**：L1（BigCache）是稀缺的本地内存资源（默认 256MB），RBAC、字典、消息模板等非速度敏感的业务也会占用 L1 空间，导致真正需要极致速度的开放平台 API 权限校验、IP 过滤等场景被挤占。

### 1.2 改造目标

将缓存模块升级为 **A/B 双模式**：

| 模式 | 名称 | 存储链路 | 适用场景 |
|------|------|----------|----------|
| **模式A** | 极速模式 | L1 (BigCache) + L2 (Redis) + L3 (DB回源) | 开放平台API权限等每次请求都要校验的场景 |
| **模式B** | 标准模式 | L2 (Redis) + L3 (DB回源) | RBAC、字典、存储配置、内容分类、消息模板等 |

**降级规则**：

- L1 关闭时 → 模式A 自动降级为模式B（纯 Redis）
- Redis 关闭时 → 模式A 和模式B 都降级为纯 BigCache

### 1.3 设计原则

- **零注册**：开发者不需要预先注册缓存场景，通过方法名即可区分模式
- **向后兼容**：所有现有方法（Fetch/Set/Get/Delete）保持不变，行为从"共享引擎"变为"模式B引擎"
- **新增 Fast 后缀方法**：FetchFast/SetFast/GetFast/DeleteFast 走模式A引擎
- **InvalidateByTags 同时失效两个引擎**：开发者不需要关心数据在哪个引擎
- **Key 统一管理**：无论是模式A还是模式B，所有缓存 Key 和 Tag **必须**在 `server/internal/pkg/cache/registry.go` 中统一定义，严禁在业务代码中硬编码 Key 字符串

---

## 二、接口变更

### 2.1 LazyCacheManager 接口（改造后）

```go
type LazyCacheManager interface {
    // ===== 模式B（标准模式）: L2 Redis + L3 DB =====
    // 适合：RBAC、字典、存储配置、内容分类、消息模板等
    // L1 关闭/开启不影响此模式，始终只走 L2
    Fetch(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    Get(ctx context.Context, key string, v interface{}) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)

    // ===== 模式A（极速模式）: L1 本地 + L2 Redis + L3 DB =====
    // 适合：开放平台API权限等每次请求都要校验的场景
    // 注意：IPAC（IP过滤）不走缓存模块，它有自有的进程内全量内存设计，不要改造
    // L1 关闭时自动降级为模式B（纯 L2）
    // Redis 也关闭时降级为纯 L1 (BigCache)
    FetchFast(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error
    SetFast(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    GetFast(ctx context.Context, key string, v interface{}) error
    DeleteFast(ctx context.Context, key string) error

    // ===== 共用方法 =====
    // InvalidateByTags 同时失效 standardCache 和 fastCache 两个引擎
    InvalidateByTags(ctx context.Context, tags ...string) error

    // SetNX 仅在 Key 不存在时写入（走标准模式B引擎）
    SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)

    // GetRedisClient 获取底层 Redis 客户端
    GetRedisClient() *redis.Client

    // RateLimit 限流校验
    RateLimit(ctx context.Context, key string, rate int, capacity int) (bool, error)

    // ListenInvalidation 启动监听分布式失效信号（内部使用）
    ListenInvalidation(ctx context.Context)
}
```

### 2.2 方法与引擎对照表

| 方法 | 引擎 | 说明 |
|------|------|------|
| `Fetch` | standardCache | 模式B：L2(Redis) only |
| `Set` | standardCache | 模式B |
| `Get` | standardCache | 模式B |
| `Delete` | standardCache | 模式B |
| `Exists` | standardCache | 模式B |
| `SetNX` | standardCache | 模式B（Nonce防重放等场景不需要L1） |
| `FetchFast` | fastCache | 模式A：L1+L2 chain |
| `SetFast` | fastCache | 模式A |
| `GetFast` | fastCache | 模式A |
| `DeleteFast` | fastCache | 模式A |
| `InvalidateByTags` | 两个引擎都失效 | 确保不管数据在哪个引擎都能被清除 |
| `RateLimit` | 不走缓存引擎 | 直接用 Redis Lua 或本地令牌桶 |

---

## 三、内部实现变更

### 3.1 结构体变更

**改造前**：

```go
type lazyCacheManager struct {
    cacheManager cache.CacheInterface[any]  // 单一引擎
    switches     SwitchChecker
    prefix       string
    redisClient  *redis.Client
    localLimiters sync.Map
}
```

**改造后**：

```go
type lazyCacheManager struct {
    // 模式B引擎：L2 (Redis) only
    // Redis 关闭时降级为 L1 (BigCache)
    standardCache cache.CacheInterface[any]

    // 模式A引擎：L1 (BigCache) + L2 (Redis) chain
    // L1 关闭时降级为 standardCache（即纯 L2）
    // Redis 也关闭时降级为 L1 (BigCache)
    fastCache cache.CacheInterface[any]

    switches     SwitchChecker
    prefix       string
    redisClient  *redis.Client
    localLimiters sync.Map
}
```

### 3.2 初始化逻辑变更

**改造前**（`NewLazyCacheManager`）：

```go
// 旧逻辑：根据配置组合确定一个引擎
if cfg.Enabled && redisClient != nil {
    l2Store := redisStore.NewRedis(redisClient)
    l2Cache := cache.New[any](l2Store)
    if cfg.L1Enabled {
        l1Cache := cache.New[any](l1Store)
        cacheMgr = cache.NewChain[any](l1Cache, l2Cache)  // 所有业务共享
    } else {
        cacheMgr = l2Cache
    }
} else {
    cacheMgr = cache.New[any](l1Store)
}
```

**改造后**（`NewLazyCacheManager`）：

```go
// 新逻辑：始终初始化两个引擎

// 1. standardCache（模式B）：始终只走 L2，Redis 关闭时降级为 L1
if cfg.Enabled && redisClient != nil {
    l2Store := redisStore.NewRedis(redisClient)
    standardCache = cache.New[any](l2Store)
} else {
    // Redis 关闭，降级为 BigCache
    standardCache = cache.New[any](l1Store)
}

// 2. fastCache（模式A）：L1 + L2 chain，L1 关闭时降级为 standardCache
if cfg.Enabled && redisClient != nil {
    l2Store := redisStore.NewRedis(redisClient)
    l2Cache := cache.New[any](l2Store)
    if cfg.L1Enabled {
        l1Cache := cache.New[any](l1Store)
        fastCache = cache.NewChain[any](l1Cache, l2Cache)
    } else {
        // L1 关闭，降级为纯 L2（和 standardCache 相同）
        fastCache = l2Cache
    }
} else {
    // Redis 关闭，降级为 BigCache
    fastCache = cache.New[any](l1Store)
}
```

### 3.3 引擎组合矩阵

| 配置状态 | standardCache | fastCache | 说明 |
|----------|---------------|-----------|------|
| Redis开 + L1开 | L2 (Redis) | Chain(L1, L2) | **正常模式**：Fast走L1+L2，标准走L2 |
| Redis开 + L1关 | L2 (Redis) | L2 (Redis) | **L1降级**：Fast退化为标准模式 |
| Redis关 | L1 (BigCache) | L1 (BigCache) | **Redis降级**：都用本地缓存 |

### 3.4 FetchFast 实现

```go
// FetchFast 模式A极速缓存：L1 -> L2 -> Loader(DB)
// L1 关闭时自动降级为纯 L2（与 Fetch 行为一致）
func (m *lazyCacheManager) FetchFast(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error {
    fullKey := m.buildKey(key)

    if !m.switches.IsCacheEnabled(moduleName) {
        val, err := loader()
        if err != nil {
            return err
        }
        return m.assign(val, v)
    }

    // 1. 尝试从 fastCache 拿数据
    data, err := m.getRawFrom(ctx, fullKey, m.fastCache)
    if err == nil && len(data) > 0 {
        if err := m.unmarshal(data, v); err == nil {
            return nil
        }
    }

    // 2. Cache Miss，调用 Loader 查库
    val, err := loader()
    if err != nil {
        return err
    }

    // 3. 回写 fastCache
    if !m.isNil(val) {
        dataToCache, err := m.marshal(val)
        if err == nil {
            options := []store.Option{store.WithExpiration(ttl)}
            if len(tags) > 0 {
                options = append(options, store.WithTags(tags))
            }
            _ = m.fastCache.Set(ctx, fullKey, dataToCache, options...)
        }
    }

    return m.assign(val, v)
}
```

### 3.5 SetFast / GetFast / DeleteFast 实现

```go
func (m *lazyCacheManager) SetFast(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    fullKey := m.buildKey(key)
    data, err := m.marshal(value)
    if err != nil {
        return err
    }
    return m.fastCache.Set(ctx, fullKey, data, store.WithExpiration(ttl))
}

func (m *lazyCacheManager) GetFast(ctx context.Context, key string, v interface{}) error {
    fullKey := m.buildKey(key)
    data, err := m.getRawFrom(ctx, fullKey, m.fastCache)
    if err != nil {
        return err
    }
    if len(data) == 0 {
        return fmt.Errorf("cached data is empty for key: %s", fullKey)
    }
    return json.Unmarshal(data, v)
}

func (m *lazyCacheManager) DeleteFast(ctx context.Context, key string) error {
    fullKey := m.buildKey(key)
    return m.fastCache.Delete(ctx, fullKey)
}
```

### 3.6 getRawFrom 辅助方法

现有 `getRaw` 方法硬编码了 `m.cacheManager`，需要改为可指定引擎的版本：

```go
// getRawFrom 从指定引擎获取原始数据
func (m *lazyCacheManager) getRawFrom(ctx context.Context, key string, engine cache.CacheInterface[any]) ([]byte, error) {
    raw, err := engine.Get(ctx, key)
    if err != nil {
        return nil, err
    }
    switch v := raw.(type) {
    case []byte:
        return v, nil
    case string:
        return []byte(v), nil
    default:
        return nil, fmt.Errorf("unexpected cache data type: %T", raw)
    }
}
```

同时修改现有 `getRaw` 方法，改为委托调用：

```go
func (m *lazyCacheManager) getRaw(ctx context.Context, key string) ([]byte, error) {
    return m.getRawFrom(ctx, key, m.standardCache)
}
```

### 3.7 InvalidateByTags 变更

**改造后**需要同时失效两个引擎：

```go
func (m *lazyCacheManager) InvalidateByTags(ctx context.Context, tags ...string) error {
    // 1. 失效 standardCache
    err1 := m.standardCache.Invalidate(ctx, store.WithInvalidateTags(tags))

    // 2. 失效 fastCache
    err2 := m.fastCache.Invalidate(ctx, store.WithInvalidateTags(tags))

    // 3. 广播失效信号给其他实例
    if m.redisClient != nil && len(tags) > 0 {
        channel := internalRedis.ChannelConfigSync(m.prefix) + ":cache_invalidation"
        payload, _ := json.Marshal(tags)
        _ = m.redisClient.Publish(ctx, channel, payload).Err()
    }

    if err1 != nil {
        return err1
    }
    return err2
}
```

### 3.8 ListenInvalidation 变更

收到广播后需要同时失效两个引擎：

```go
func (m *lazyCacheManager) ListenInvalidation(ctx context.Context) {
    if m.redisClient == nil {
        return
    }

    channel := internalRedis.ChannelConfigSync(m.prefix) + ":cache_invalidation"
    sub := m.redisClient.Subscribe(ctx, channel)

    go func() {
        defer sub.Close()
        ch := sub.Channel()
        for {
            select {
            case <-ctx.Done():
                return
            case msg := <-ch:
                var tags []string
                if err := json.Unmarshal([]byte(msg.Payload), &tags); err == nil {
                    _ = m.standardCache.Invalidate(ctx, store.WithInvalidateTags(tags))
                    _ = m.fastCache.Invalidate(ctx, store.WithInvalidateTags(tags))
                }
            }
        }
    }()
}
```

### 3.9 现有方法改造

所有现有方法（Fetch/Set/Get/Delete/Exists）从 `m.cacheManager` 改为 `m.standardCache`：

```go
// Set - 改为使用 standardCache
func (m *lazyCacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    fullKey := m.buildKey(key)
    data, err := m.marshal(value)
    if err != nil {
        return err
    }
    return m.standardCache.Set(ctx, fullKey, data, store.WithExpiration(ttl))
}

// Get - 改为使用 standardCache
func (m *lazyCacheManager) Get(ctx context.Context, key string, v interface{}) error {
    fullKey := m.buildKey(key)
    data, err := m.getRaw(ctx, fullKey)  // getRaw 已改为使用 standardCache
    if err != nil {
        return err
    }
    if len(data) == 0 {
        return fmt.Errorf("cached data is empty for key: %s", fullKey)
    }
    return json.Unmarshal(data, v)
}

// Delete - 改为使用 standardCache
func (m *lazyCacheManager) Delete(ctx context.Context, key string) error {
    fullKey := m.buildKey(key)
    return m.standardCache.Delete(ctx, fullKey)
}

// Exists - 改为使用 standardCache
func (m *lazyCacheManager) Exists(ctx context.Context, key string) (bool, error) {
    fullKey := m.buildKey(key)
    _, err := m.getRaw(ctx, fullKey)
    if err == nil {
        return true, nil
    }
    if errors.Is(err, store.NotFound{}) {
        return false, nil
    }
    return false, err
}

// Fetch - 改为使用 standardCache
func (m *lazyCacheManager) Fetch(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error {
    fullKey := m.buildKey(key)

    if !m.switches.IsCacheEnabled(moduleName) {
        val, err := loader()
        if err != nil {
            return err
        }
        return m.assign(val, v)
    }

    data, err := m.getRaw(ctx, fullKey)
    if err == nil && len(data) > 0 {
        if err := m.unmarshal(data, v); err == nil {
            return nil
        }
    }

    val, err := loader()
    if err != nil {
        return err
    }

    if !m.isNil(val) {
        dataToCache, err := m.marshal(val)
        if err == nil {
            options := []store.Option{store.WithExpiration(ttl)}
            if len(tags) > 0 {
                options = append(options, store.WithTags(tags))
            }
            _ = m.standardCache.Set(ctx, fullKey, dataToCache, options...)
        }
    }

    return m.assign(val, v)
}
```

**注意**：`SetNX` 方法保持不变，它直接操作 Redis 或走标准模式，不涉及引擎切换。

---

## 四、业务层改造

### 4.1 需要改用 Fast 方法的模块

#### 4.1.1 开放平台 API 服务 (`server/internal/service/open_platform/api.go`)

| 方法 | 当前调用 | 改造后调用 |
|------|----------|------------|
| `ListAllApis` | `s.cacheMgr.Fetch(ctx, key, ...)` | `s.cacheMgr.FetchFast(ctx, key, ...)` |
| `GetAppAllowedApis` | `s.cacheMgr.Fetch(ctx, key, ...)` | `s.cacheMgr.FetchFast(ctx, key, ...)` |

**完整改造后的代码**：

```go
func (s *openApiService) ListAllApis(ctx context.Context) ([]*open_platform.OpenApi, error) {
    var list []*open_platform.OpenApi
    key := cache.KeyOpenApiAll()
    err := s.cacheMgr.FetchFast(ctx, key, cache.TagOpenApi, []string{cache.TagOpenApi}, 3600*time.Second, &list, func() (interface{}, error) {
        return s.apiRepo.ListAll(ctx)
    })
    return list, err
}

func (s *openApiService) GetAppAllowedApis(ctx context.Context, appID string) ([]string, error) {
    var apiKeys []string
    key := cache.KeyAppApis(appID)
    err := s.cacheMgr.FetchFast(ctx, key, cache.TagApp, []string{cache.TagApp, cache.TagAppID(appID)}, 3600*time.Second, &apiKeys, func() (interface{}, error) {
        // ... loader 逻辑不变
    })
    return apiKeys, err
}
```

#### 4.1.2 开放平台应用服务 (`server/internal/service/open_platform/app.go`)

| 方法 | 当前调用 | 改造后调用 |
|------|----------|------------|
| `GetAppByKey` | `s.cacheMgr.Fetch(ctx, key, ...)` | `s.cacheMgr.FetchFast(ctx, key, ...)` |
| `VerifyAppScope` | `s.cacheMgr.Fetch(ctx, key, ...)` | `s.cacheMgr.FetchFast(ctx, key, ...)` |
| `ListAvailableScopes` | `s.cacheMgr.Fetch(ctx, key, ...)` | `s.cacheMgr.FetchFast(ctx, key, ...)` |
| `CreateScopeGroup` | `s.cacheMgr.Delete(ctx, ...)` | `s.cacheMgr.DeleteFast(ctx, ...)` |
| `UpdateScopeGroup` | `s.cacheMgr.Delete(ctx, ...)` | `s.cacheMgr.DeleteFast(ctx, ...)` |
| `DeleteScopeGroup` | `s.cacheMgr.Delete(ctx, ...)` | `s.cacheMgr.DeleteFast(ctx, ...)` |

**注意**：`InvalidateByTags` 不需要改，因为它同时失效两个引擎。

**完整改造后的关键方法**：

```go
func (s *appService) GetAppByKey(ctx context.Context, appKey string) (*open_platform.App, error) {
    var app open_platform.App
    key := cache.KeyAppInfo(appKey)
    err := s.cacheMgr.FetchFast(ctx, key, cache.TagApp, []string{cache.TagApp, cache.TagAppKey(appKey)}, 3600*time.Second, &app, func() (interface{}, error) {
        return s.repo.GetByKey(ctx, appKey)
    })
    if err != nil {
        return nil, err
    }
    return &app, nil
}

func (s *appService) VerifyAppScope(ctx context.Context, appID string, requiredScope string) (bool, error) {
    if requiredScope == "" {
        return true, nil
    }
    var scopes []string
    key := cache.KeyAppScopes(appID)
    err := s.cacheMgr.FetchFast(ctx, key, cache.TagApp, []string{cache.TagApp, cache.TagAppID(appID)}, 3600*time.Second, &scopes, func() (interface{}, error) {
        return s.repo.GetAppScopes(ctx, appID)
    })
    if err != nil {
        return false, err
    }
    for _, s := range scopes {
        if s == requiredScope {
            return true, nil
        }
    }
    return false, nil
}

func (s *appService) ListAvailableScopes(ctx context.Context) ([]map[string]string, error) {
    var groups []*open_platform.AppScopeGroup
    key := cache.KeyAppAvailableScopes()
    err := s.cacheMgr.FetchFast(ctx, key, cache.TagApp, []string{cache.TagApp, "app_scopes"}, 3600*time.Second, &groups, func() (interface{}, error) {
        allGroups, err := s.repo.ListScopeGroups(ctx)
        if err != nil {
            return nil, err
        }
        enabledGroups := make([]*open_platform.AppScopeGroup, 0)
        for _, g := range allGroups {
            if g.Status == open_platform.AppStatusEnabled {
                enabledGroups = append(enabledGroups, g)
            }
        }
        return enabledGroups, nil
    })
    if err != nil {
        return nil, err
    }
    res := make([]map[string]string, 0, len(groups))
    for _, g := range groups {
        res = append(res, map[string]string{
            "name": g.Name,
            "code": g.Code,
        })
    }
    return res, nil
}

func (s *appService) CreateScopeGroup(ctx context.Context, group *open_platform.AppScopeGroup) error {
    if err := s.repo.CreateScopeGroup(ctx, group); err != nil {
        return err
    }
    _ = s.cacheMgr.DeleteFast(ctx, cache.KeyAppAvailableScopes())
    return nil
}

func (s *appService) UpdateScopeGroup(ctx context.Context, group *open_platform.AppScopeGroup) error {
    if err := s.repo.UpdateScopeGroup(ctx, group); err != nil {
        return err
    }
    _ = s.cacheMgr.DeleteFast(ctx, cache.KeyAppAvailableScopes())
    return nil
}

func (s *appService) DeleteScopeGroup(ctx context.Context, id uint64) error {
    if err := s.repo.DeleteScopeGroup(ctx, id); err != nil {
        return err
    }
    _ = s.cacheMgr.DeleteFast(ctx, cache.KeyAppAvailableScopes())
    return nil
}
```

#### 4.1.3 IP访问控制服务 (`server/internal/service/ipac/ipac.go`)

IPAC 当前使用 `sync.RWMutex + map` 做全量内存缓存，有自己的热更机制（Redis Pub/Sub 通知重载）。
这个模块**不走缓存模块的 Fetch**，它的内存数据结构（`[]*net.IPNet`）不适合 key→value 缓存。

**IPAC 不需要改造，也不应该改造。** 保持现有设计。原因：

1. IPAC 的查询是 CIDR 网段匹配（`net.IPNet.Contains()`），不是 key→value 精确查找
2. 它已经是"进程内全量加载"模式，速度已经是最快的，比模式A更快（零序列化开销）
3. 硬塞进模式A反而会引入 JSON 序列化/反序列化开销，性能倒退

### 4.2 保持不变（模式B）的模块

以下模块的所有缓存调用**不需要任何修改**，它们自动走模式B：

| 模块 | 文件 | 使用的缓存方法 |
|------|------|----------------|
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

### 4.3 开放平台中间件 (`server/internal/middleware/open_platform_auth.go`)

中间件本身**不需要改造**。它通过 Service 层间接使用缓存：

```
中间件 → appSvc.GetAppByKey() → cacheMgr.FetchFast()  ← 已在 Service 层改造
中间件 → apiSvc.GetAppAllowedApis() → cacheMgr.FetchFast()  ← 已在 Service 层改造
中间件 → appSvc.GetCacheMgr().SetNX() → cacheMgr.SetNX()  ← 走标准模式B，Nonce不需要L1
中间件 → ipacSvc.CheckIP() → IPAC自有内存  ← 不走缓存模块
```

---

## 五、配置变更

### 5.1 config.toml 无需修改

现有配置项完全兼容：

```toml
[redis]
enabled = true
prefix = "so"
l1_enabled = false       # L1 开关：控制模式A是否生效
local_max_size_mb = 256  # L1 最大内存
local_max_entry_kb = 500 # L1 单条最大
local_ttl_min = 10       # L1 默认TTL
```

**语义变化**：

- 改造前：`l1_enabled` 控制所有业务是否走 L1
- 改造后：`l1_enabled` 仅控制模式A（Fast方法）是否走 L1，模式B（标准方法）始终不走 L1

### 5.2 config.go 无需修改

`RedisConfig` 结构体字段不变。

---

## 六、完整改造步骤清单

### 步骤1：改造 `server/internal/pkg/cache/manager.go`

1. 将 `lazyCacheManager.cacheManager` 字段拆分为 `standardCache` 和 `fastCache`
2. 修改 `NewLazyCacheManager` 初始化逻辑，构建两个引擎
3. 新增 `getRawFrom` 辅助方法
4. 修改 `getRaw` 委托给 `getRawFrom(ctx, key, m.standardCache)`
5. 修改 `Fetch` 方法使用 `m.standardCache`
6. 修改 `Set` 方法使用 `m.standardCache`
7. 修改 `Get` 方法使用 `m.standardCache`（通过 getRaw）
8. 修改 `Delete` 方法使用 `m.standardCache`
9. 修改 `Exists` 方法使用 `m.standardCache`（通过 getRaw）
10. 修改 `Fetch` 内部的回写使用 `m.standardCache.Set`
11. 新增 `FetchFast` 方法，逻辑同 Fetch 但使用 `m.fastCache`
12. 新增 `SetFast` 方法，逻辑同 Set 但使用 `m.fastCache`
13. 新增 `GetFast` 方法，逻辑同 Get 但使用 `m.fastCache`
14. 新增 `DeleteFast` 方法，逻辑同 Delete 但使用 `m.fastCache`
15. 修改 `InvalidateByTags` 同时失效两个引擎
16. 修改 `ListenInvalidation` 同时失效两个引擎
17. 在接口 `LazyCacheManager` 中新增4个 Fast 方法声明

### 步骤2：改造 `server/internal/service/open_platform/api.go`

1. `ListAllApis`：`Fetch` → `FetchFast`
2. `GetAppAllowedApis`：`Fetch` → `FetchFast`

### 步骤3：改造 `server/internal/service/open_platform/app.go`

1. `GetAppByKey`：`Fetch` → `FetchFast`
2. `VerifyAppScope`：`Fetch` → `FetchFast`
3. `ListAvailableScopes`：`Fetch` → `FetchFast`
4. `CreateScopeGroup`：`Delete` → `DeleteFast`，同时将硬编码 `"app:available_scopes"` 替换为 `cache.KeyAppAvailableScopes()`
5. `UpdateScopeGroup`：`Delete` → `DeleteFast`，同时将硬编码 `"app:available_scopes"` 替换为 `cache.KeyAppAvailableScopes()`
6. `DeleteScopeGroup`：`Delete` → `DeleteFast`，同时将硬编码 `"app:available_scopes"` 替换为 `cache.KeyAppAvailableScopes()`

> **红线提醒**：所有缓存 Key 必须通过 `cache/registry.go` 的工厂函数生成，严禁硬编码字符串。

### 步骤4：编译验证

```bash
cd server && go build ./...
```

### 步骤5：更新文档

更新 `docs/server-module-cache.md`，反映 A/B 双模式设计。

---

## 七、不需要改造的文件

以下文件**不需要任何修改**：

| 文件 | 原因 |
|------|------|
| `server/internal/pkg/cache/registry.go` | Key/Tag 定义不变，所有模式A/B的Key统一在此管理，严禁硬编码 |
| `server/internal/config/config.go` | 配置结构不变 |
| `server/config.toml` | 配置项不变 |
| `server/internal/app/wire.go` | 依赖注入不变 |
| `server/internal/service/system/*.go` | RBAC 走模式B，方法不变 |
| `server/internal/service/dict/dict.go` | 字典走模式B，方法不变 |
| `server/internal/service/storage/config.go` | 存储走模式B，方法不变 |
| `server/internal/service/content/category.go` | 分类走模式B，方法不变 |
| `server/internal/service/message/message.go` | 消息走模式B，方法不变 |
| `server/internal/pkg/captcha/store.go` | 验证码走模式B，方法不变 |
| `server/internal/service/user/verification.go` | 验证走模式B，方法不变 |
| `server/internal/service/ipac/ipac.go` | IPAC自有内存，不走缓存模块 |
| `server/internal/middleware/open_platform_auth.go` | 通过Service层间接使用 |

---

## 八、开发者使用指南

### 8.1 如何选择模式

```
需要极致速度？（每次HTTP请求都要校验）
  ├─ 是 → 用 FetchFast / SetFast / GetFast / DeleteFast
  └─ 否 → 用 Fetch / Set / Get / Delete
```

### 8.2 典型场景

```go
// 场景1：开放平台API权限校验（每次请求都调用）→ 用 Fast
s.cacheMgr.FetchFast(ctx, cache.KeyAppApis(appID), "open_api", tags, ttl, &apis, loader)

// 场景2：RBAC菜单树（登录后加载一次）→ 用标准
s.cacheMgr.Fetch(ctx, cache.KeyMenuTree(), "rbac", tags, ttl, &tree, loader)

// 场景3：字典数据（页面加载时读取）→ 用标准
s.cacheMgr.Fetch(ctx, cache.KeyDictData(code), "dict", tags, ttl, &list, loader)

// 场景4：验证码（一次性写入消费）→ 用标准
s.cacheMgr.Set(ctx, cache.KeyVerificationCode("captcha", id), value, ttl)

// 场景5：Nonce防重放（一次性校验）→ 用标准（SetNX）
s.cacheMgr.SetNX(ctx, cache.KeyAppNonce(appKey, nonce), "1", 60*time.Second)
```

### 8.3 失效缓存

```go
// InvalidateByTags 同时失效两个引擎，开发者不需要关心数据在哪个引擎
s.cacheMgr.InvalidateByTags(ctx, cache.TagApp)
s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu)
```

### 8.4 二次开发新增缓存

```go
// 新增极速场景（如：API限流配置、特征库等）
// ⚠️ key 必须在 cache/registry.go 中定义工厂函数，严禁硬编码
s.cacheMgr.FetchFast(ctx, cache.KeyXxx(param), module, tags, ttl, &result, loader)

// 新增标准场景（如：业务配置、用户偏好等）
// ⚠️ key 必须在 cache/registry.go 中定义工厂函数，严禁硬编码
s.cacheMgr.Fetch(ctx, cache.KeyXxx(param), module, tags, ttl, &result, loader)
```

---

## 九、验证方案

### 9.1 编译验证

```bash
cd d:\NetyAdmin\server && go build ./...
```

### 9.2 功能验证

1. 启动服务（`l1_enabled = true`）
2. 调用开放平台API，确认请求正常通过
3. 在 admin-web 修改接口权限关联的API
4. 再次调用开放平台API，确认权限变更立即生效（InvalidateByTags 清除 fastCache）
5. 将 `l1_enabled` 改为 `false`，重启服务
6. 重复步骤2-4，确认降级后功能正常

### 9.3 L1 隔离验证

1. 启动服务（`l1_enabled = true`）
2. 大量调用 RBAC 相关接口（走模式B，不应进入 L1）
3. 检查 BigCache 统计，确认只有开放平台相关的 key 占用 L1
4. 大量调用开放平台接口（走模式A，应进入 L1）
5. 检查 BigCache 统计，确认 L1 中有开放平台相关 key

---

## 十、风险与注意事项

1. **BigCache 初始化**：当前代码只创建一个 `bigcacheClient`，两个引擎如果都需要 BigCache（如 Redis 关闭时），会共享同一个 BigCache 实例。这是安全的，因为 BigCache 本身是并发安全的。但如果需要严格隔离，可以为 fastCache 创建独立的 BigCache 实例。**建议**：当前共享即可，未来如有需要再拆分。

2. **Key 冲突**：standardCache 和 fastCache 使用相同的 key 命名空间（都通过 `buildKey` 加前缀）。如果同一个 key 同时用 Fetch 和 FetchFast 写入，两个引擎中都会有该 key 的副本。这不是问题，因为 InvalidateByTags 会同时清除两个引擎。但开发者应避免对同一个 key 混用两种模式。

3. **SetNX 不区分模式**：SetNX 始终走标准模式B。如果未来有需要在模式A中使用 SetNX，可以新增 SetNXFast 方法。当前场景（Nonce 防重放）不需要 L1 加速。

4. **captcha 模块**：captcha/store.go 直接使用 `cache.Set/Get/Delete`，这些方法改造后走 standardCache。验证码不需要 L1 加速，这是正确的。
