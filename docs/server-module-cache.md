# 缓存模块详解

本文档详细介绍 NetyAdmin 缓存模块的架构设计、使用方式和二次开发指南。

---

## 一、模块概述

缓存模块提供统一的缓存抽象层，支持 Redis 和本地内存（BigCache）双引擎，根据配置自动切换。

### 1.1 核心特性

- **L1/L2 二级缓存**：本地 BigCache (L1) + Redis (L2) 分层架构，兼顾性能与一致性
- **多机缓存一致性**：基于 Redis Pub/Sub 的分布式缓存失效广播，确保集群环境缓存同步
- **透明缓存**：业务层无感知切换，自动处理缓存穿透、回源逻辑
- **Tags批量失效**：支持按标签批量清除缓存，跨机器自动同步
- **动态开关**：支持运行时开启/关闭缓存，无需重启服务
- **Key规范**：统一的Key命名规范，严禁硬编码

---

## 二、目录结构

```
server/internal/pkg/cache/
├── manager.go          # 缓存管理器（LazyCacheManager）
└── registry.go         # Key注册表与工厂
```

---

## 三、架构设计

### 3.1 缓存管理器接口 (LazyCacheManager)

`LazyCacheManager` 是对底层存储的统一抽象，通过接口解耦业务逻辑与具体的存储实现（Redis/BigCache）。

```go
type LazyCacheManager interface {
    // --- 核心能力 ---
    
    // Fetch 具有透明缓存能力的获取方法
    // 流程：检查开关 -> 命中缓存则返回 -> 未命中执行 loader -> 结果异步落库 -> 返回
    Fetch(ctx context.Context, key string, module string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error

    // InvalidateByTags 根据标签批量失效所有关联 Key (支持分布式同步)
    InvalidateByTags(ctx context.Context, tags ...string) error

    // --- 基础原子操作 ---
    
    // Set 强制写入缓存
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    
    // SetNX 原子性写入 (仅当 Key 不存在时写入)
    // 场景：分布式锁模拟、防重放 Nonce 校验
    SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
    
    // Get/Delete/Exists 基础操作
    Get(ctx context.Context, key string, v interface{}) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)

    // --- 高级治理能力 ---

    // RateLimit 分布式/单机自适应限流
    // 机制：Redis 模式下执行 Lua 滑动窗口脚本；本地模式降级为令牌桶算法
    RateLimit(ctx context.Context, key string, rate int, capacity int) (bool, error)

    // GetRedisClient 获取原生客户端（仅用于 Pub/Sub 等特殊扩展）
    GetRedisClient() *redis.Client
}
```

### 3.2 L1/L2 二级缓存架构

系统采用 **L1/L2 分层缓存** 架构，根据部署模式自动适配：

#### 单机模式（无 Redis）

```
┌─────────────────────────────────────┐
│           Application               │
│  ┌─────────────────────────────┐    │
│  │      L1: BigCache           │    │
│  │   (本地内存， ultra-fast)    │    │
│  └─────────────────────────────┘    │
└─────────────────────────────────────┘
```

#### 多机模式（有 Redis）

```
┌─────────────────────────────────────────────────────────────┐
│                      多机部署环境                              │
│  ┌──────────────┐        Redis Cluster        ┌──────────┐  │
│  │   Machine A  │◄───────────────────────────►│  L2存储   │  │
│  │ ┌──────────┐ │        (共享缓存层)          │ (Redis)  │  │
│  │ │ L1: Big  │ │                             └──────────┘  │
│  │ │  Cache   │ │                                   ▲       │
│  │ └──────────┘ │         Pub/Sub 广播              │       │
│  └──────────────┘◄──────────────────────────────────┘       │
│         ▲                                                   │
│         │         ┌──────────────┐                          │
│         └────────►│   Machine B  │                          │
│   缓存失效广播     │ ┌──────────┐ │                          │
│                   │ │ L1: Big  │ │                          │
│                   │ │  Cache   │ │                          │
│                   │ └──────────┘ │                          │
│                   └──────────────┘                          │
└─────────────────────────────────────────────────────────────┘
```

#### 读写流程

**读取流程**（Fetch）：

1. 检查模块缓存开关（`cache_switches`）
2. 尝试从 L1 (BigCache) 读取 → 命中直接返回
3. L1 未命中，尝试从 L2 (Redis) 读取 → 命中则回填 L1 并返回
4. L2 未命中，执行 Loader 回源数据库 → 结果异步写入 L1 和 L2

**写入流程**（Set）：

- 同时写入 L1 (BigCache) 和 L2 (Redis)
- 带 Tags 的缓存可被批量失效

**失效流程**（InvalidateByTags）：

1. 本地执行缓存失效（L1 + L2）
2. 通过 Redis Pub/Sub 向所有机器广播失效信号
3. 其他机器收到广播后，仅失效本地 L1（避免重复失效 L2）

#### 配置参数

```toml
[redis]
enabled = true          # 启用 Redis 即进入多机模式
host = "localhost"
port = 6379
prefix = "netyadmin"    # 全局 Key 前缀

[cache]
local_ttl = 10          # L1 本地缓存 TTL（分钟）
local_max_size_mb = 256 # L1 最大内存占用（MB）
l1_enabled = true       # 是否启用 L1 缓存（多机模式下）
```

---

## 四、配置说明

### 4.1 配置文件（config.toml）

```toml
[redis]
enabled = true
host = "localhost"
port = 6379
password = ""
db = 0
prefix = "netyadmin"

[cache]
# 本地缓存过期时间（分钟）
local_ttl = 10
# 最大本地缓存条目数
local_max_entries = 10000
```

### 4.2 动态配置（sys_configs）

| 配置项 | Group | Key | 说明 |
|--------|-------|-----|------|
| RBAC缓存开关 | cache_switches | rbac | true/false |
| 字典缓存开关 | cache_switches | dict | true/false |
| 配置缓存开关 | cache_switches | config | true/false |

---

## 五、多机部署与缓存一致性

### 5.1 架构概述

在多机部署环境下，NetyAdmin 采用 **L1/L2 + Pub/Sub** 方案保证缓存一致性：

- **L1 (BigCache)**: 每台机器的本地内存缓存，访问速度极快（微秒级）
- **L2 (Redis)**: 共享的分布式缓存层，所有机器共享
- **Pub/Sub 广播**: 基于 Redis 发布订阅的缓存失效同步机制

### 5.2 缓存失效广播机制

当某台机器执行 `InvalidateByTags` 时：

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Machine A   │     │    Redis     │     │  Machine B   │
│              │     │   Pub/Sub    │     │              │
│ Invalidate   │────►│  Channel:    │────►│  收到广播     │
│ ByTags(tags) │     │ cache_inval  │     │              │
│              │     │ idation      │     │ 仅失效本地L1  │
│ L1+L2失效    │     │              │     │ (不碰L2)     │
└──────────────┘     └──────────────┘     └──────────────┘
```

**关键设计决策**：

1. **独立频道**: 缓存失效使用独立频道 `{prefix}:channel:cache_invalidation`，与配置热更频道 `{prefix}:channel:config_sync` 解耦，避免互相干扰

2. **仅失效 L1**: 收到广播的机器只失效本地 L1 (BigCache)，不操作 L2 (Redis)。因为 L2 在发起失效的机器上已经被清除，其他机器直接回源 L2 即可拿到最新数据

3. **幂等性**: 缓存失效是幂等操作，多次执行无副作用

### 5.3 核心代码实现

**失效广播**（`InvalidateByTags`）：

```go
func (m *lazyCacheManager) InvalidateByTags(ctx context.Context, tags ...string) error {
    // 1. 本地失效 L1 + L2
    err := m.cacheManager.Invalidate(ctx, store.WithInvalidateTags(tags))
    
    // 2. 向集群广播（如果启用了 Redis）
    if m.redisClient != nil && len(tags) > 0 {
        channel := internalRedis.ChannelCacheInvalidation(m.prefix)
        payload, _ := json.Marshal(tags)
        _ = m.redisClient.Publish(ctx, channel, payload).Err()
    }
    return err
}
```

**监听广播**（`ListenInvalidation`）：

```go
func (m *lazyCacheManager) ListenInvalidation(ctx context.Context) {
    channel := internalRedis.ChannelCacheInvalidation(m.prefix)
    sub := m.redisClient.Subscribe(ctx, channel)
    
    go func() {
        defer sub.Close()
        ch := sub.Channel()
        for msg := range ch {
            var tags []string
            if err := json.Unmarshal([]byte(msg.Payload), &tags); err == nil {
                // 仅失效 L1，避免重复失效 L2
                if m.l1Cache != nil {
                    _ = m.l1Cache.Invalidate(ctx, store.WithInvalidateTags(tags))
                }
            }
        }
    }()
}
```

### 5.4 部署建议

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

## 六、Key 注册表规范 (Registry)

**强制规范**: 严禁在业务 Service 中硬编码任何字符串作为缓存 Key 或 Tag。必须在 `internal/pkg/cache/registry.go` 中统一定义。

### 5.1 定义原则

1. **Key 函数化**: 接收唯一标识（如 ID, Code），返回格式化后的 Key 字符串。
2. **Tag 语义化**: Tag 用于关联一组 Key。如修改用户资料后，失效 `TagUser(userID)` 对应的所有列表和详情缓存。

### 5.2 示例代码 (registry.go)

```go
// Key 定义
func KeyAppInfo(appKey string) string { return fmt.Sprintf("open:app:info:%s", appKey) }
func KeyMsgTemplate(code string) string { return fmt.Sprintf("msg:template:%s", code) }

// Tag 定义
const (
    TagApp = "open:app"
    TagMsgTemplate = "msg:template"
)
```

---

## 六、使用示例 (重构后的标准用法)

### 6.1 读多写少场景 (Fetch + Tags)

```go
func (s *appService) GetAppByKey(ctx context.Context, appKey string) (*system.App, error) {
    var app system.App
    key := cache.KeyAppInfo(appKey)
    // 使用 TagApp 方便管理员更新时批量失效
    err := s.cacheMgr.Fetch(ctx, key, cache.TagApp, []string{cache.TagApp, "app:"+appKey}, 1*time.Hour, &app, func() (interface{}, error) {
        return s.repo.GetByKey(ctx, appKey)
    })
    return &app, err
}
```

### 6.2 变更失效逻辑 (Invalidate)

```go
func (s *appService) UpdateApp(ctx context.Context, app *system.App) error {
    if err := s.repo.Update(ctx, app); err != nil { return err }
    // 精准失效该应用的缓存
    return s.cacheMgr.InvalidateByTags(ctx, "app:"+app.AppKey)
}
```

### 6.3 防重放校验 (SetNX)

```go
// OpenPlatform 签名校验中的 Nonce 防重放
nonceKey := cache.KeyAppNonce(appKey, nonce)
set, err := s.cacheMgr.SetNX(ctx, nonceKey, "1", 60*time.Second)
if err != nil || !set {
    return errorx.CodeSignatureFailed // 重复请求
}
```

---

## 七、二次开发示例

### 7.1 新增模块缓存

```go
// internal/pkg/cache/registry.go

// 新增内容模块的Key工厂
var (
    KeyArticleInfo   = func(id uint) string { return fmt.Sprintf("article:%d:info", id) }
    KeyArticleList   = func(page, size int) string { return fmt.Sprintf("article:list:%d:%d", page, size) }
    KeyCategoryTree  = func() string { return "category:tree" }
)

// 新增Tags
const (
    TagContent = "content"
    TagArticle = "article"
    TagCategory = "category"
)
```

### 7.2 在服务中使用缓存

```go
// internal/service/content/article.go

type articleService struct {
    repo          ArticleRepository
    cacheManager  *cache.LazyCacheManager
}

func (s *articleService) GetArticle(ctx context.Context, id uint) (*entity.Article, error) {
    result, err := s.cacheManager.Fetch(
        ctx,
        "content",                              // 模块名（对应cache_switches:content）
        cache.KeyArticleInfo(id),
        []string{cache.TagContent, cache.TagArticle},
        30*time.Minute,
        func() (any, error) {
            return s.repo.GetByID(ctx, id)
        },
    )
    if err != nil {
        return nil, err
    }
    return result.(*entity.Article), nil
}

func (s *articleService) CreateArticle(ctx context.Context, article *entity.Article) error {
    // 1. 创建文章
    if err := s.repo.Create(ctx, article); err != nil {
        return err
    }
    
    // 2. 清除列表缓存
    s.cacheManager.DeleteByTag(ctx, cache.TagArticle)
    
    return nil
}
```

### 7.3 配置动态开关

```sql
-- 在sys_configs中添加内容模块缓存开关
INSERT INTO sys_configs (group, key, value, description) VALUES
('cache_switches', 'content', 'true', '内容模块缓存开关');
```

---

## 八、监控与调试

### 8.1 缓存命中率

```go
// 在管理后台添加缓存统计接口
func (h *SystemHandler) GetCacheStats(c *gin.Context) {
    stats := h.cacheManager.GetStats()
    response.Success(c, gin.H{
        "hits":       stats.Hits,
        "misses":     stats.Misses,
        "hit_rate":   float64(stats.Hits) / float64(stats.Hits+stats.Misses),
        "keys_count": stats.KeysCount,
    })
}
```

### 8.2 缓存Key列表

```go
// 获取指定tag的所有key
keys := h.cacheManager.GetKeysByTag(ctx, cache.TagRBAC)
```

---

## 九、最佳实践

1. **合理设置TTL**：读多写少的数据设置较长TTL（如菜单树），频繁变更的数据设置较短TTL
2. **使用Tags**：为相关缓存设置相同tag，便于批量失效
3. **模块隔离**：不同业务模块使用不同的模块名，便于独立控制
4. **回源保护**：回源函数中做好错误处理，避免缓存穿透
5. **大对象处理**：超过1MB的数据建议压缩后存储

---

## 十、相关文档

- [Server架构设计](./server-architecture.md)
- [配置热同步](./server-module-config.md)
