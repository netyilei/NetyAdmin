package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	bigcacheStore "github.com/eko/gocache/store/bigcache/v4"
	redisStore "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"

	"NetyAdmin/internal/config"
)

var (
	ErrCacheDisabled = errors.New("cache disabled for module")
)

type LazyCacheManager interface {
	// Fetch 具有透明缓存能力的获取方法。
	// 如果 Redis 存在对应 key，并且 moduleSwitch 开启，则返回 Redis 中数据。
	// 否则，调用 loader() 拿到最新数据，自动写入带 tags 的 Redis，并返回。
	Fetch(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error

	// InvalidateByTags 根据标签批量失效所有关联 Key
	InvalidateByTags(ctx context.Context, tags ...string) error

	// Set 强制写入一个缓存项，带过期时间
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	// Get 直接读取一个缓存项
	Get(ctx context.Context, key string, v interface{}) error
	// Delete 删除一个缓存项
	Delete(ctx context.Context, key string) error
	// Exists 判断一个缓存项是否存在
	Exists(ctx context.Context, key string) (bool, error)
}

type SwitchChecker interface {
	IsCacheEnabled(moduleName string) bool
}

type lazyCacheManager struct {
	cacheManager *cache.Cache[any]
	switches     SwitchChecker
	prefix       string
	redisClient  *redis.Client
}

// DefaultSwitchChecker 给一个总是返回 True 的默认校验器，直到我们实现 configsync
type DefaultSwitchChecker struct{}

func (d *DefaultSwitchChecker) IsCacheEnabled(moduleName string) bool {
	return true
}

func NewLazyCacheManager(cfg *config.RedisConfig, redisClient *redis.Client, checker SwitchChecker) (LazyCacheManager, error) {
	if checker == nil {
		checker = &DefaultSwitchChecker{}
	}

	var backendStore store.StoreInterface

	if cfg.Enabled {
		// Redis 模式
		backendStore = redisStore.NewRedis(redisClient)
	} else {
		// BigCache 本地内存模式降级
		bigcacheClient, err := bigcache.New(context.Background(), bigcache.DefaultConfig(10*time.Minute))
		if err != nil {
			return nil, fmt.Errorf("初始化 BigCache 失败: %w", err)
		}
		backendStore = bigcacheStore.NewBigcache(bigcacheClient)
	}

	// 初始化 gocache 引擎。我们存储 any 以通吃所有序列化的格式 (Redis 读出来是 string, BigCache 是 []byte)
	cacheMgr := cache.New[any](backendStore)

	return &lazyCacheManager{
		cacheManager: cacheMgr,
		switches:     checker,
		prefix:       cfg.Prefix,
		redisClient:  redisClient,
	}, nil
}

func (m *lazyCacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	fullKey := m.buildKey(key)
	data, err := m.marshal(value)
	if err != nil {
		return err
	}
	return m.cacheManager.Set(ctx, fullKey, data, store.WithExpiration(ttl))
}

func (m *lazyCacheManager) getRaw(ctx context.Context, key string) ([]byte, error) {
	raw, err := m.cacheManager.Get(ctx, key)
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

func (m *lazyCacheManager) Get(ctx context.Context, key string, v interface{}) error {
	fullKey := m.buildKey(key)
	data, err := m.getRaw(ctx, fullKey)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return fmt.Errorf("cached data is empty for key: %s", fullKey)
	}
	return json.Unmarshal(data, v)
}

func (m *lazyCacheManager) Delete(ctx context.Context, key string) error {
	fullKey := m.buildKey(key)
	return m.cacheManager.Delete(ctx, fullKey)
}

func (m *lazyCacheManager) Exists(ctx context.Context, key string) (bool, error) {
	// gocache v4 doesn't have Exists, we can try to get raw
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

func (m *lazyCacheManager) buildKey(key string) string {
	if m.prefix != "" {
		return fmt.Sprintf("%s:%s", m.prefix, key)
	}
	return key
}

// Fetch 实现透明查库
// 参数说明：
// v: 目标接收对象（需传指针，类似于 json.Unmarshal 的 receiver）
// loader: 如果由于开关关闭或 Cache Miss，要执行的数据库回源逻辑。loader 需要返回能 json.Marshal 的对象。
func (m *lazyCacheManager) Fetch(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error {
	fullKey := m.buildKey(key)

	// 如果模块缓存被动态关闭，直接穿透回源
	if !m.switches.IsCacheEnabled(moduleName) {
		val, err := loader()
		if err != nil {
			return err
		}
		return m.assign(val, v)
	}

	// 1. 尝试从缓存拿数据
	data, err := m.getRaw(ctx, fullKey)
	if err == nil && len(data) > 0 {
		// Cache Hit
		if err := m.unmarshal(data, v); err == nil {
			return nil
		}
	}

	// 2. Cache Miss 或发生错误，调用 Loader 查库
	val, err := loader()
	if err != nil {
		return err
	}

	// 3. 回写缓存
	dataToCache, err := m.marshal(val)
	if err == nil {
		// 设置 Tag 和 TTL
		options := []store.Option{
			store.WithExpiration(ttl),
			store.WithTags(tags),
		}
		_ = m.cacheManager.Set(ctx, fullKey, dataToCache, options...)
	}

	// 返回结果
	return m.assign(val, v)
}

func (m *lazyCacheManager) InvalidateByTags(ctx context.Context, tags ...string) error {
	return m.cacheManager.Invalidate(ctx, store.WithInvalidateTags(tags))
}

func (m *lazyCacheManager) marshal(val interface{}) ([]byte, error) {
	return json.Marshal(val)
}

func (m *lazyCacheManager) unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (m *lazyCacheManager) assign(src interface{}, dest interface{}) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}
