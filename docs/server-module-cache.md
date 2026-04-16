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

### 3.1 缓存管理器

```go
// LazyCacheManager 统一缓存管理器
type LazyCacheManager struct {
    redisClient *redis.Client
    localCache  *bigcache.BigCache
    config      *Config
}

// Fetch 通用缓存获取方法
func (m *LazyCacheManager) Fetch(
    ctx context.Context,
    moduleName string,      // 模块名（用于动态开关）
    key string,             // 缓存key
    tags []string,          // 标签（用于批量失效）
    ttl time.Duration,      // 过期时间
    fetch func() (any, error), // 回源函数
) (any, error)
```

### 3.2 存储优先级

```
1. 检查模块缓存开关（sys_configs: cache_switches:{moduleName}）
   ↓ 关闭则直接回源
2. 检查Redis（如果启用）
   ↓ 命中则返回
3. 检查本地缓存
   ↓ 命中则返回
4. 执行回源函数
   ↓
5. 写入缓存（Redis + 本地）
   ↓
6. 返回数据
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

## 五、Key注册表

### 5.1 预定义Key工厂

```go
// internal/pkg/cache/registry.go

// Key工厂函数
var (
    // RBAC相关
    KeyAdminInfo    = func(adminID uint) string { return fmt.Sprintf("admin:%d:info", adminID) }
    KeyRoleInfo     = func(roleID uint) string { return fmt.Sprintf("role:%d:info", roleID) }
    KeyMenuTree     = func() string { return "menu:tree" }
    KeyRoleMenus    = func(roleID uint) string { return fmt.Sprintf("role:%d:menus", roleID) }
    KeyRoleButtons  = func(roleID uint) string { return fmt.Sprintf("role:%d:buttons", roleID) }
    KeyRoleAPIs     = func(roleID uint) string { return fmt.Sprintf("role:%d:apis", roleID) }
    
    // 字典相关
    KeyDictType     = func(code string) string { return fmt.Sprintf("dict:type:%s", code) }
    KeyDictData     = func(typeCode string) string { return fmt.Sprintf("dict:data:%s", typeCode) }
    
    // 配置相关
    KeySysConfig    = func(group, key string) string { return fmt.Sprintf("config:%s:%s", group, key) }
)

// Tags定义
const (
    TagRBAC     = "rbac"
    TagDict     = "dict"
    TagConfig   = "config"
    TagMenu     = "menu"
    TagRole     = "role"
)
```

---

## 六、使用示例

### 6.1 基础使用

```go
// 获取管理员信息（带缓存）
func (s *adminService) GetAdminInfo(ctx context.Context, adminID uint) (*vo.AdminInfo, error) {
    result, err := s.cacheManager.Fetch(
        ctx,
        "rbac",                                    // 模块名
        cache.KeyAdminInfo(adminID),              // key
        []string{cache.TagRBAC, cache.TagRole},   // tags
        10*time.Minute,                           // ttl
        func() (any, error) {
            // 回源函数：从数据库查询
            return s.repo.GetByID(ctx, adminID)
        },
    )
    if err != nil {
        return nil, err
    }
    return result.(*vo.AdminInfo), nil
}
```

### 6.2 批量失效缓存

```go
// 角色权限变更后，清除相关缓存
func (s *adminService) UpdateRolePermissions(ctx context.Context, roleID uint, menuIDs []uint) error {
    // 1. 更新数据库
    if err := s.repo.UpdateRoleMenus(ctx, roleID, menuIDs); err != nil {
        return err
    }
    
    // 2. 清除相关缓存
    keys := []string{
        cache.KeyRoleMenus(roleID),
        cache.KeyRoleInfo(roleID),
    }
    s.cacheManager.Delete(ctx, keys...)
    
    // 3. 按tag批量清除
    s.cacheManager.DeleteByTag(ctx, cache.TagRBAC)
    
    return nil
}
```

### 6.3 直接操作缓存

```go
// 设置缓存
err := s.cacheManager.Set(ctx, "custom:key", data, 5*time.Minute)

// 获取缓存
val, err := s.cacheManager.Get(ctx, "custom:key")

// 删除缓存
err := s.cacheManager.Delete(ctx, "key1", "key2")

// 检查存在
exists := s.cacheManager.Exists(ctx, "custom:key")
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
