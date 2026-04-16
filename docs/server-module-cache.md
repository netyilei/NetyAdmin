# 缓存模块详解

本文档详细介绍 NetyAdmin 缓存模块的架构设计、使用方式和二次开发指南。

---

## 一、模块概述

缓存模块提供统一的缓存抽象层，支持 Redis 和本地内存（BigCache）双引擎，根据配置自动切换。

### 1.1 核心特性

- **双引擎支持**：Redis / BigCache 自动切换
- **透明缓存**：业务层无感知切换
- **Tags批量失效**：支持按标签批量清除缓存
- **动态开关**：支持运行时开启/关闭缓存
- **Key规范**：统一的Key命名规范

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

### 3.2 存储优先级与降级逻辑

系统采用 **“自适应双引擎”** 架构：
1.  **配置驱动**: 优先读取 `config.toml` 中的 `[redis].enabled`。
2.  **写透模式**: `Fetch` 成功后，会根据配置同时写入 Redis 和本地 BigCache（L1/L2 二级缓存结构）。
3.  **原子模拟**: 在未开启 Redis 的单机环境下，`SetNX` 通过内存锁模拟原子性，`RateLimit` 通过 `golang.org/x/time/rate` 保证单机限流准确。

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

## 五、Key 注册表规范 (Registry)

**强制规范**: 严禁在业务 Service 中硬编码任何字符串作为缓存 Key 或 Tag。必须在 `internal/pkg/cache/registry.go` 中统一定义。

### 5.1 定义原则
1.  **Key 函数化**: 接收唯一标识（如 ID, Code），返回格式化后的 Key 字符串。
2.  **Tag 语义化**: Tag 用于关联一组 Key。如修改用户资料后，失效 `TagUser(userID)` 对应的所有列表和详情缓存。

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
