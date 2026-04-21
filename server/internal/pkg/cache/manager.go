package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	bigcacheStore "github.com/eko/gocache/store/bigcache/v4"
	redisStore "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/pkg/pubsub"
)

var (
	ErrCacheDisabled = errors.New("cache disabled for module")
)

type LazyCacheManager interface {
	// Fetch 具有透明缓存能力的获取方法。
	// 按照 L1 (Local) -> L2 (Redis) -> Loader (DB) 的顺序获取。
	// 如果命中 L2，会自动回填 L1。如果执行 Loader，会自动回填 L1 和 L2。
	Fetch(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error

	// InvalidateByTags 根据标签批量失效所有关联 Key (如果是集群模式，会通过 Redis Pub/Sub 同步失效)
	InvalidateByTags(ctx context.Context, tags ...string) error

	// Set 强制写入一个缓存项，带过期时间
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	// SetNX 仅在 Key 不存在时写入 (原子操作)
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
	// Get 直接读取一个缓存项
	Get(ctx context.Context, key string, v interface{}) error
	// Delete 删除一个缓存项
	Delete(ctx context.Context, key string) error
	// Exists 判断一个缓存项是否存在
	Exists(ctx context.Context, key string) (bool, error)

	// InvalidateL1ByTags 仅失效本地 L1 缓存（由 PubSubBus 订阅者调用，避免递归）
	InvalidateL1ByTags(ctx context.Context, tags ...string) error

	// SetEventBus 注入 PubSubBus 实例（解决循环依赖：CacheManager 先于 EventBus 创建）
	SetEventBus(bus pubsub.EventBus)

	// RateLimit 限流校验
	RateLimit(ctx context.Context, key string, rate int, capacity int) (bool, error)
}

type SwitchChecker interface {
	IsCacheEnabled(moduleName string) bool
}

type lazyCacheManager struct {
	cacheManager cache.CacheInterface[any]
	l1Cache      *cache.Cache[any]
	switches     SwitchChecker
	prefix       string
	redisClient  *redis.Client
	eventBus     pubsub.EventBus

	localLimiters sync.Map
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

	// 1. 初始化 L1 (本地 BigCache) - 配置参数来自 config.toml
	localTTL := 10 * time.Minute
	if cfg.LocalTTLMin > 0 {
		localTTL = time.Duration(cfg.LocalTTLMin) * time.Minute
	}

	bcConfig := bigcache.DefaultConfig(localTTL)
	bcConfig.Shards = 1024
	if cfg.LocalMaxSizeMB > 0 {
		bcConfig.HardMaxCacheSize = cfg.LocalMaxSizeMB
	} else {
		bcConfig.HardMaxCacheSize = 256 // 默认 256MB
	}
	if cfg.LocalMaxEntryKB > 0 {
		bcConfig.MaxEntrySize = cfg.LocalMaxEntryKB * 1024
	} else {
		bcConfig.MaxEntrySize = 500 * 1024 // 默认 500KB
	}

	bigcacheClient, err := bigcache.New(context.Background(), bcConfig)
	if err != nil {
		return nil, fmt.Errorf("初始化 BigCache 失败: %w", err)
	}
	l1Store := bigcacheStore.NewBigcache(bigcacheClient)

	var cacheMgr cache.CacheInterface[any]
	var l1Cache *cache.Cache[any]

	if cfg.Enabled && redisClient != nil {
		l2Store := redisStore.NewRedis(redisClient)
		l2Cache := cache.New[any](l2Store)

		if cfg.L1Enabled {
			l1Cache = cache.New[any](l1Store)
			cacheMgr = cache.NewChain[any](l1Cache, l2Cache)
		} else {
			cacheMgr = l2Cache
		}
	} else {
		l1Cache = cache.New[any](l1Store)
		cacheMgr = l1Cache
	}

	mgr := &lazyCacheManager{
		cacheManager:  cacheMgr,
		l1Cache:       l1Cache,
		switches:      checker,
		prefix:        cfg.Prefix,
		redisClient:   redisClient,
		localLimiters: sync.Map{},
	}

	return mgr, nil
}

func (m *lazyCacheManager) RateLimit(ctx context.Context, key string, r int, capacity int) (bool, error) {
	// 1. 如果 Redis 开启，使用 Redis 脚本限流 (分布式准确)
	if m.redisClient != nil {
		// 这里借用一下我们现有的 Lua 脚本逻辑，但直接写在 manager 里以减少依赖
		script := `
local bucket_key = KEYS[1]
local rate = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local last_tokens = tonumber(redis.call("HGET", bucket_key, "tokens"))
local last_time = tonumber(redis.call("HGET", bucket_key, "last_time"))

if last_tokens == nil then
    last_tokens = capacity
    last_time = now
end

local delta = math.max(0, now - last_time)
local generated = delta * rate
local current_tokens = math.min(capacity, last_tokens + generated)

local allowed = false
if current_tokens >= requested then
    current_tokens = current_tokens - requested
    allowed = true
end

redis.call("HSET", bucket_key, "tokens", current_tokens, "last_time", now)
redis.call("EXPIRE", bucket_key, 86400)

return allowed and 1 or 0
`
		fullKey := m.buildKey("ratelimit:" + key)
		res, err := m.redisClient.Eval(ctx, script, []string{fullKey}, r, capacity, time.Now().Unix(), 1).Result()
		if err != nil {
			return false, err
		}
		return res.(int64) == 1, nil
	}

	// 2. 如果 Redis 未开启，降级为本地令牌桶限流
	limiter, _ := m.localLimiters.LoadOrStore(key, rate.NewLimiter(rate.Limit(r), capacity))
	return limiter.(*rate.Limiter).Allow(), nil
}

func (m *lazyCacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	fullKey := m.buildKey(key)
	data, err := m.marshal(value)
	if err != nil {
		return err
	}
	return m.cacheManager.Set(ctx, fullKey, data, store.WithExpiration(ttl))
}

func (m *lazyCacheManager) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	fullKey := m.buildKey(key)

	// 1. Redis 模式 (原生原子支持)
	if m.redisClient != nil {
		data, err := m.marshal(value)
		if err != nil {
			return false, err
		}
		res, err := m.redisClient.SetArgs(ctx, fullKey, data, redis.SetArgs{
			Mode: "NX",
			TTL:  ttl,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return false, nil
			}
			return false, err
		}
		return res == "OK", nil
	}

	// 2. 本地模式 (非绝对原子，但对单机应用足够)
	exists, _ := m.Exists(ctx, key)
	if exists {
		return false, nil
	}
	err := m.Set(ctx, key, value, ttl)
	return err == nil, err
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

	// 3. 校验数据真实性后再回写缓存 (只有非 nil 数据才进缓存)
	if !m.isNil(val) {
		dataToCache, err := m.marshal(val)
		if err == nil {
			// 设置 Tag 和 TTL
			options := []store.Option{
				store.WithExpiration(ttl),
			}
			if len(tags) > 0 {
				options = append(options, store.WithTags(tags))
			}
			_ = m.cacheManager.Set(ctx, fullKey, dataToCache, options...)
		}
	}

	// 返回结果
	return m.assign(val, v)
}

func (m *lazyCacheManager) isNil(i interface{}) bool {
	if i == nil {
		return true
	}
	vi := reflect.ValueOf(i)
	switch vi.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return vi.IsNil()
	default:
		return false
	}
}

func (m *lazyCacheManager) InvalidateByTags(ctx context.Context, tags ...string) error {
	err := m.cacheManager.Invalidate(ctx, store.WithInvalidateTags(tags))

	if m.eventBus != nil && len(tags) > 0 {
		payload, _ := json.Marshal(tags)
		_ = m.eventBus.Publish(ctx, pubsub.TopicCacheInvalidation, payload)
	}

	return err
}

func (m *lazyCacheManager) InvalidateL1ByTags(ctx context.Context, tags ...string) error {
	if m.l1Cache != nil {
		return m.l1Cache.Invalidate(ctx, store.WithInvalidateTags(tags))
	}
	return nil
}

func (m *lazyCacheManager) SetEventBus(bus pubsub.EventBus) {
	m.eventBus = bus
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
