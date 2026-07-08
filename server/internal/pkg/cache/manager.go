package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	bigcacheStore "github.com/eko/gocache/store/bigcache/v4"
	redisStore "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/pkg/pubsub"
)

var (
	ErrCacheDisabled = errors.New("cache disabled for module")
)

// ConfigCache 配置类数据的缓存接口，对应 Fast 方法系列（L1+L2 chain）。
//
// 适用场景（RULES.md §12.1）：数据变更后容忍秒级延迟全集群生效，
// 走 L1 本地内存 + L2 Redis 共享层，靠 InvalidateByTags 跨节点广播失效。
// 典型持有者：app_service / dict / menu / role / api / message_template /
// content（admin & client）/ storage_config / admin_info 等配置类服务。
//
// 铁律（RULES.md §12.2）：Fast 写必须配套 Fast 读，禁止 Fast 写 + 非 Fast 读混搭。
type ConfigCache interface {
	// FetchFast 模式A（极速模式）：L1 (BigCache) → L2 (Redis) → L3 (DB 回源)
	// L1 关闭时自动降级为模式B（纯 L2）
	// L2 命中时自动回填 L1（带 tags 与 ttl）
	// 失效统一走 InvalidateByTags
	FetchFast(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error

	// SetFast 强制写入 L1+L2（模式A），带过期时间和 tags
	// L2 是 source of truth，写入失败返回 error；L1 写入失败仅 Warn（降级 L2 only）
	SetFast(ctx context.Context, key string, value interface{}, tags []string, ttl time.Duration) error

	// GetFast 强制读取 L1→L2（模式A），L2 命中回填 L1 时带 tags
	// ttl 用于 L1 回填的过期时间，与 L2 一致
	GetFast(ctx context.Context, key string, tags []string, ttl time.Duration, v interface{}) error

	// DeleteFast 删除 L1+L2 chain 中的缓存项（模式A）
	// L1 关闭时降级为 Delete（L2 only）
	// 用于配置类数据的精确删除（配合 Fast 写入路径，避免 L1 残留旧数据）
	DeleteFast(ctx context.Context, key string) error

	// InvalidateByTags 根据标签批量失效 L1+L2 关联 Key
	// 集群模式下通过 Redis Pub/Sub 同步失效其他节点 L1
	InvalidateByTags(ctx context.Context, tags ...string) error

	// IsCacheEnabled 检查指定模块的缓存开关是否开启
	IsCacheEnabled(moduleName string) bool
}

// SecurityCache 安全类数据的缓存接口，对应非 Fast 方法系列（L2 only）。
//
// 适用场景（RULES.md §12.1）：数据变更后要求「立即」全集群生效，
// 只走 L2 (Redis) 共享层，删除一次全集群立即生效，无窗口期。
// 典型持有者：token_store / verification / admin_auth / user_auth /
// pkg/auth/session / middleware/auth / log/error / captcha/store 等安全类服务。
//
// 铁律（RULES.md §12.1）：非 Fast 方法绝不碰 L1，避免多机部署下出现安全窗口期。
// 降级链：L2 → (miss/关闭) → L3(DB)；L2 关闭时降级用 l1Cache 本地兜底。
type SecurityCache interface {
	// Fetch 模式B（标准模式）：L2 (Redis) → L3 (DB 回源)
	// 只走 L2，绝不碰 L1（铁律）
	// L2 关闭时通过 l2() 降级到 l1Cache 本地兜底
	Fetch(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error

	// Set 写入 L2 (Redis)，绝不碰 L1（铁律）
	// 可选 tags 用于 InvalidateByTags 批量失效（如 token 按 userID tag 失效）
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration, tags ...string) error

	// Get 直接读取 L2 (Redis)（模式B），绝不碰 L1
	Get(ctx context.Context, key string, v interface{}) error

	// Delete 删除 L2 (Redis) 中的缓存项，绝不碰 L1（铁律）
	Delete(ctx context.Context, key string) error

	// Exists 判断 L2 中缓存项是否存在（模式B）
	Exists(ctx context.Context, key string) (bool, error)

	// SetNX 仅在 Key 不存在时写入（原子操作，模式B）
	// 用于 Nonce 防重放等需要原子占位的场景
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)

	// Incr 原子自增计数器，并在首次设置时配置 TTL
	// 仅在 Redis（L2）上操作，BigCache（L1）不支持 INCR
	// Redis 不可用时返回 error（fail-closed，不允许跳过原子计数）
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)

	// InvalidateByTags 根据标签批量失效 L2 关联 Key
	// 集群模式下通过 Redis Pub/Sub 同步失效其他节点 L1
	InvalidateByTags(ctx context.Context, tags ...string) error

	// IsCacheEnabled 检查指定模块的缓存开关是否开启
	IsCacheEnabled(moduleName string) bool
}

// CacheLifecycle 缓存生命周期接口，仅由应用装配层（wire.go）持有。
//
// 设计原因：EventBus 在 CacheManager 之后创建（解决循环依赖），
// 通过 SetEventBus 注入；InvalidateL1ByTags 由 PubSubBus
// TopicCacheInvalidation 订阅者调用，用于跨节点 L1 失效。
// 这两个方法不属于业务消费方接口，单独隔离避免泄漏给 service 层。
type CacheLifecycle interface {
	// SetEventBus 注入 PubSubBus 实例（解决循环依赖：CacheManager 先于 EventBus 创建）
	SetEventBus(bus pubsub.EventBus)

	// InvalidateL1ByTags 仅失效本地 L1 缓存（由 PubSubBus TopicCacheInvalidation 订阅者调用）
	InvalidateL1ByTags(ctx context.Context, tags ...string) error
}

type SwitchChecker interface {
	IsCacheEnabled(moduleName string) bool
}

// LazyCacheManager 缓存管理器具体类型，同时实现 ConfigCache、SecurityCache、CacheLifecycle 三个接口。
//
// 设计意图（接口隔离原则）：
//   - 装配层（wire.go）持有 *LazyCacheManager 具体类型，可调用全部方法
//   - 业务消费方只持有其对应的最小接口（ConfigCache 或 SecurityCache）
//   - Go 隐式接口转换：*LazyCacheManager 自动满足三个接口
//
// 三个接口分别对应（RULES.md §12.1 缓存铁律）：
//   - ConfigCache：Fast 系列，L1+L2 chain，配置类数据
//   - SecurityCache：非 Fast 系列，L2 only，安全类数据
//   - CacheLifecycle：仅 wire.go 持有，注入 EventBus + 跨节点 L1 失效
type LazyCacheManager struct {
	cacheManager cache.CacheInterface[any]
	l1Cache      *cache.Cache[any]
	l1Enabled    bool
	switches     SwitchChecker
	prefix       string
	redisClient  *redis.Client
	eventBus     pubsub.EventBus
	eventBusMu   sync.RWMutex

	localNX sync.Map
	l2Cache *cache.Cache[any]

	// flightGroup 合并 Fetch/FetchFast 在同一 key 上的并发回源调用，
	// 防止缓存击穿（热点 key 失效瞬间 N 个请求同时穿透到 DB）。
	// loader 错误会原样传播给所有等待中的 caller，不会被吞掉。
	flightGroup singleflight.Group
}

// DefaultSwitchChecker 给一个总是返回 True 的默认校验器，直到我们实现 configsync
type DefaultSwitchChecker struct{}

func (d *DefaultSwitchChecker) IsCacheEnabled(moduleName string) bool {
	return true
}

// NewLazyCacheManager 创建缓存管理器实例。
//
// 返回具体类型 *LazyCacheManager（而非某个接口），由调用方按需赋值给
// ConfigCache / SecurityCache / CacheLifecycle 三个接口之一。
// 这样 wire.go 装配层可以同时使用三个接口的能力（如 SetEventBus + InvalidateL1ByTags），
// 而业务消费方只持有其对应的最小接口（接口隔离原则）。
//
// 三接口分别对应（RULES.md §12.1 缓存铁律）：
//   - ConfigCache：Fast 系列，L1+L2 chain，配置类数据
//   - SecurityCache：非 Fast 系列，L2 only，安全类数据
//   - CacheLifecycle：仅 wire.go 持有，注入 EventBus + 跨节点 L1 失效
func NewLazyCacheManager(cfg *config.RedisConfig, redisClient *redis.Client, checker SwitchChecker) (*LazyCacheManager, error) {
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
	var l2Cache *cache.Cache[any]

	l1Cache = cache.New[any](l1Store)

	if cfg.Enabled && redisClient != nil {
		l2Store := redisStore.NewRedis(redisClient)
		l2Cache = cache.New[any](l2Store)

		if cfg.L1Enabled {
			cacheMgr = cache.NewChain[any](l1Cache, l2Cache)
		} else {
			cacheMgr = l2Cache
		}
	} else {
		cacheMgr = l1Cache
	}

	mgr := &LazyCacheManager{
		cacheManager: cacheMgr,
		l1Cache:      l1Cache,
		l2Cache:      l2Cache,
		l1Enabled:    cfg.L1Enabled,
		switches:     checker,
		prefix:       cfg.Prefix,
		redisClient:  redisClient,
		localNX:      sync.Map{},
	}

	return mgr, nil
}

// Set 写入 L2 (Redis)，绝不碰 L1（铁律）。
// 带 tags 用于 InvalidateByTags 批量失效（如 token 按 userID tag 失效）。
// L2 未启用时通过 l2() 降级到 l1Cache 本地兜底。
func (m *LazyCacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration, tags ...string) error {
	fullKey := m.buildKey(key)
	data, err := m.marshal(value)
	if err != nil {
		return err
	}
	options := []store.Option{store.WithExpiration(ttl)}
	if len(tags) > 0 {
		options = append(options, store.WithTags(tags))
	}
	return m.l2().Set(ctx, fullKey, data, options...)
}

func (m *LazyCacheManager) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
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

	// 2. 本地模式：使用 sync.Map.LoadOrStore 实现原子操作
	_, loaded := m.localNX.LoadOrStore(fullKey, struct{}{})
	if loaded {
		return false, nil
	}
	if err := m.Set(ctx, key, value, ttl); err != nil {
		m.localNX.Delete(fullKey)
		return false, err
	}
	time.AfterFunc(ttl, func() { m.localNX.Delete(fullKey) })
	return true, nil
}

// l2 返回非 Fast 系列方法应操作的缓存层：
//   - L2 (Redis) 启用时：返回 l2Cache（绝不碰 L1，铁律）
//   - L2 未启用（Redis 关闭）：降级返回 l1Cache（本地兜底，避免直接打 DB）
//
// 这样实现了清晰的分层：
//   - Fast 系列（SetFast/GetFast 等）：显式用 cacheManager（chain = L1+L2 组合）
//   - 非 Fast 系列（Set/Get/Delete/Fetch 等）：用 l2()，正常只走 L2，降级时用 L1 兜底
func (m *LazyCacheManager) l2() cache.CacheInterface[any] {
	if m.l2Cache != nil {
		return m.l2Cache
	}
	return m.l1Cache
}

// getRaw 只读 L2 (Redis)，绝不碰 L1（铁律）。
// L2 未启用时通过 l2() 降级到 l1Cache 本地兜底。
func (m *LazyCacheManager) getRaw(ctx context.Context, key string) ([]byte, error) {
	raw, err := m.l2().Get(ctx, key)
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

func (m *LazyCacheManager) Get(ctx context.Context, key string, v interface{}) error {
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

// Delete 删除 L2 (Redis) 中的缓存项，绝不碰 L1（铁律）。
// L2 未启用时通过 l2() 降级到 l1Cache 本地兜底。
func (m *LazyCacheManager) Delete(ctx context.Context, key string) error {
	fullKey := m.buildKey(key)
	return m.l2().Delete(ctx, fullKey)
}

func (m *LazyCacheManager) Exists(ctx context.Context, key string) (bool, error) {
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

// Incr 原子自增计数器，基于 Redis INCR + EXPIRE 实现。
//
// 设计说明（B2 原子化）：
//   - BigCache（L1）不支持 INCR，只在 Redis（L2）上操作
//   - Redis 不可用时返回 error（fail-closed，不允许跳过原子计数）
//   - 仅在 INCR 返回值 == 1（首次设置）时调用 EXPIRE 设置 TTL
//   - INCR 与 EXPIRE 非完全原子（极小窗口 TTL 未设置），但首次 INCR 必然返回 1，
//     EXPIRE 失败时计数仍存在但无 TTL（永久残留），下次清理由 ClearLoginRetry 兜底
//
// 用于登录失败计数等需防 TOCTOU 竞态的场景。
func (m *LazyCacheManager) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	// fail-closed：Redis 不可用时不允许降级为本地计数（避免多节点计数失真）
	if m.redisClient == nil {
		return 0, fmt.Errorf("incr requires redis client, but redis is unavailable (fail-closed)")
	}

	fullKey := m.buildKey(key)

	// 1. 原子 INCR
	val, err := m.redisClient.Incr(ctx, fullKey).Result()
	if err != nil {
		return 0, err
	}

	// 2. 仅在首次设置（INCR 返回 1）时配置 TTL
	if val == 1 && ttl > 0 {
		if err := m.redisClient.Expire(ctx, fullKey, ttl).Err(); err != nil {
			// EXPIRE 失败仅记录不返回错误：计数已正确递增，
			// TTL 缺失会导致 key 永久残留，但 ClearLoginRetry 会在登录成功时清理
			// 此处选择不破坏原子性（不回滚 INCR），优先保证计数正确
			return val, nil
		}
	}

	return val, nil
}

func (m *LazyCacheManager) buildKey(key string) string {
	if m.prefix != "" {
		return fmt.Sprintf("%s:%s", m.prefix, key)
	}
	return key
}

// Fetch 实现透明查库（模式B：L2 only）
// 参数说明：
// v: 目标接收对象（需传指针，类似于 json.Unmarshal 的 receiver）
// loader: 如果由于开关关闭或 Cache Miss，要执行的数据库回源逻辑。loader 需要返回能 json.Marshal 的对象。
//
// 铁律（RULES.md §12.1）：非 Fast 方法只走 L2 (Redis)，绝不碰 L1。
// 即使 L1 全局开启，Fetch 也只读 L2，不会回填 L1。
// 需要自动回填 L1 请使用 FetchFast（模式A）。
func (m *LazyCacheManager) Fetch(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error {
	fullKey := m.buildKey(key)

	// 如果模块缓存被动态关闭，直接穿透回源
	if !m.switches.IsCacheEnabled(moduleName) {
		val, err := loader()
		if err != nil {
			return err
		}
		return m.assign(val, v)
	}

	// 1. 尝试从 L2 拿数据（非 Fast 只走 L2，绝不碰 L1）
	data, err := m.getRaw(ctx, fullKey)
	if err == nil && len(data) > 0 {
		// Cache Hit
		if err := m.unmarshal(data, v); err == nil {
			return nil
		}
		// 反序列化失败说明 L2 缓存数据损坏，主动删除避免后续请求重复尝试失败
		if delErr := m.l2().Delete(ctx, fullKey); delErr != nil {
			slog.Warn("cache: delete corrupt key failed (Fetch L2)",
				"key", fullKey, "unmarshalErr", err, "delErr", delErr)
		}
	}

	// 2. Cache Miss 或发生错误，用 singleflight 合并同一 key 上的并发回源，
	// 防止缓存击穿（热点 key 失效瞬间 N 个请求同时穿透到 DB）。
	// loader 返回的错误会原样传播给所有等待中的 caller。
	val, err, _ := m.flightGroup.Do(fullKey, func() (interface{}, error) {
		return loader()
	})
	// double-check: sf.Do 等待期间可能已有其他请求将数据写入缓存
	if data2, err2 := m.getRaw(ctx, fullKey); err2 == nil && len(data2) > 0 {
		if err3 := m.unmarshal(data2, v); err3 == nil {
			return nil
		}
	}
	if err != nil {
		return err
	}

	// 3. 校验数据真实性后回写 L2 缓存（只有非 nil 数据才进缓存，绝不碰 L1）
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
			if err := m.l2().Set(ctx, fullKey, dataToCache, options...); err != nil {
				slog.Warn("cache: L2 backfill failed", "key", fullKey, "err", err)
			}
		}
	}

	// 返回结果
	return m.assign(val, v)
}

func (m *LazyCacheManager) isNil(i interface{}) bool {
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

func (m *LazyCacheManager) FetchFast(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error {
	if !m.l1Enabled {
		return m.Fetch(ctx, key, moduleName, tags, ttl, v, loader)
	}

	fullKey := m.buildKey(key)

	if !m.switches.IsCacheEnabled(moduleName) {
		val, err := loader()
		if err != nil {
			return err
		}
		return m.assign(val, v)
	}

	if m.l1Cache != nil {
		data, err := m.l1Cache.Get(ctx, fullKey)
		if err == nil {
			if raw, ok := data.([]byte); ok && len(raw) > 0 {
				if err := m.unmarshal(raw, v); err == nil {
					return nil
				}
				// 反序列化失败说明 L1 缓存数据损坏，主动删除避免后续请求重复尝试失败
				if delErr := m.l1Cache.Delete(ctx, fullKey); delErr != nil {
					slog.Warn("cache: delete corrupt key failed (FetchFast L1)",
						"key", fullKey, "unmarshalErr", err, "delErr", delErr)
				}
			}
		}
	}

	if m.redisClient != nil {
		data, err := m.redisClient.Get(ctx, fullKey).Bytes()
		if err == nil && len(data) > 0 {
			if err := m.unmarshal(data, v); err == nil {
				if m.l1Cache != nil {
					backfillOpts := []store.Option{store.WithExpiration(ttl)}
					if len(tags) > 0 {
						backfillOpts = append(backfillOpts, store.WithTags(tags))
					}
					if err := m.l1Cache.Set(ctx, fullKey, data, backfillOpts...); err != nil {
						slog.Warn("cache: L1 backfill failed (FetchFast)", "key", fullKey, "err", err)
					}
				}
				return nil
			}
			// 反序列化失败说明 L2 缓存数据损坏，主动删除避免后续请求重复尝试失败
			if delErr := m.redisClient.Del(ctx, fullKey).Err(); delErr != nil {
				slog.Warn("cache: delete corrupt key failed (FetchFast L2)",
					"key", fullKey, "unmarshalErr", err, "delErr", delErr)
			}
		}
	}

	// Cache Miss，用 singleflight 合并同一 key 上的并发回源，防止缓存击穿。
	// 与 Fetch 共用同一个 flightGroup，确保 Fetch/FetchFast 在同一 fullKey 上互相去重。
	// loader 返回的错误会原样传播给所有等待中的 caller。
	val, err, _ := m.flightGroup.Do(fullKey, func() (interface{}, error) {
		return loader()
	})
	// double-check: sf.Do 等待期间可能已有其他请求将数据写入缓存（L1 或 L2）
	if data2, err2 := m.getRaw(ctx, fullKey); err2 == nil && len(data2) > 0 {
		if err3 := m.unmarshal(data2, v); err3 == nil {
			return nil
		}
	}
	if err != nil {
		return err
	}

	if !m.isNil(val) {
		dataToCache, err := m.marshal(val)
		if err == nil {
			options := []store.Option{
				store.WithExpiration(ttl),
			}
			if len(tags) > 0 {
				options = append(options, store.WithTags(tags))
			}
			if err := m.cacheManager.Set(ctx, fullKey, dataToCache, options...); err != nil {
				slog.Warn("cache: L2 backfill failed (FetchFast)", "key", fullKey, "err", err)
			}
		}
	}

	return m.assign(val, v)
}

func (m *LazyCacheManager) SetFast(ctx context.Context, key string, value interface{}, tags []string, ttl time.Duration) error {
	fullKey := m.buildKey(key)
	data, err := m.marshal(value)
	if err != nil {
		return err
	}

	options := []store.Option{store.WithExpiration(ttl)}
	if len(tags) > 0 {
		options = append(options, store.WithTags(tags))
	}

	if !m.l1Enabled {
		return m.cacheManager.Set(ctx, fullKey, data, options...)
	}

	// L2 写入：L2 (Redis) 是 source of truth，写入失败必须返回 error，
	// 让调用方感知 L2 故障（数据未进入共享缓存，后续读会 cache miss 回源）。
	if m.l2Cache != nil {
		if err := m.l2Cache.Set(ctx, fullKey, data, options...); err != nil {
			return fmt.Errorf("cache.SetFast: L2 Set failed: %w", err)
		}
	}

	// L1 写入（失败仅 Warn，不阻断主流程；L1 是优化层，故障时降级到 L2 only）
	if m.l1Cache != nil {
		l1Opts := []store.Option{store.WithExpiration(ttl)}
		if len(tags) > 0 {
			l1Opts = append(l1Opts, store.WithTags(tags))
		}
		if err := m.l1Cache.Set(ctx, fullKey, data, l1Opts...); err != nil {
			slog.Warn("cache.SetFast: L1 Set failed (degraded to L2 only)",
				"key", key, "err", err)
		}
	}

	return nil
}

func (m *LazyCacheManager) GetFast(ctx context.Context, key string, tags []string, ttl time.Duration, v interface{}) error {
	if !m.l1Enabled {
		return m.Get(ctx, key, v)
	}

	fullKey := m.buildKey(key)

	if m.l1Cache != nil {
		data, err := m.l1Cache.Get(ctx, fullKey)
		if err == nil {
			if raw, ok := data.([]byte); ok && len(raw) > 0 {
				if err := m.unmarshal(raw, v); err == nil {
					return nil
				}
			}
		}
	}

	if m.redisClient != nil {
		data, err := m.redisClient.Get(ctx, fullKey).Bytes()
		if err == nil && len(data) > 0 {
			if err := m.unmarshal(data, v); err == nil {
				if m.l1Cache != nil {
					backfillOpts := []store.Option{store.WithExpiration(ttl)}
					if len(tags) > 0 {
						backfillOpts = append(backfillOpts, store.WithTags(tags))
					}
					if err := m.l1Cache.Set(ctx, fullKey, data, backfillOpts...); err != nil {
						slog.Warn("cache: L1 backfill failed (GetFast)", "key", fullKey, "err", err)
					}
				}
				return nil
			}
		}
	}

	return fmt.Errorf("cache miss for key: %s", fullKey)
}

// DeleteFast 删除 L1+L2 chain 中的缓存项（模式A）。
//
// 设计意图（RULES.md §12.2 读写配套）：
//   - 配置类数据用 Fast 系列写入 L1+L2，删除时也必须清 L1+L2，否则 L1 残留旧数据
//   - L1 关闭时降级为 L2 only 删除（与 Delete 行为一致）
//
// 错误传播策略（与 SetFast 一致）：
//   - L2 删除失败返回 error（L2 是 source of truth，删除失败意味着缓存仍存在旧数据）
//   - L1 删除失败仅 Warn（L1 是优化层，故障时由 TTL 兜底自愈）
func (m *LazyCacheManager) DeleteFast(ctx context.Context, key string) error {
	fullKey := m.buildKey(key)
	if !m.l1Enabled {
		return m.l2().Delete(ctx, fullKey)
	}

	// L1 删除（失败仅 Warn，L1 是优化层）
	if m.l1Cache != nil {
		if err := m.l1Cache.Delete(ctx, fullKey); err != nil {
			slog.Warn("cache.DeleteFast: L1 Delete failed (degraded to L2 only)",
				"key", key, "err", err)
		}
	}

	// L2 删除（失败返回 error，L2 是 source of truth）
	return m.l2().Delete(ctx, fullKey)
}

func (m *LazyCacheManager) IsCacheEnabled(moduleName string) bool {
	return m.switches.IsCacheEnabled(moduleName)
}

// InvalidateByTags 按 tag 失效缓存。
//
// 设计意图：
//   - 本地调用 m.cacheManager.Invalidate 已经清除了 L1 (BigCache, 节点本地内存)
//     与 L2 (Redis 共享缓存) 两级缓存。
//   - L2 为共享 Redis，一次清理对本集群所有节点立即生效，无需重复清理。
//   - 后续 PubSub Publish 仅通知其他节点清理各自 L1（节点本地内存无法被远端
//     直接清理，必须由各节点本地处理）。
//   - 整体设计：L2 共享缓存无需重复清理；PubSub 通道只用于 L1 跨节点同步。
//
// 错误传播策略（Task 13.1 + Task 14.1）：
//   - L2 (cacheManager.Invalidate) 失败 → 不广播 L1 跨节点失效，直接返回 error。
//     原因：L2 已失败意味着本节点与其他节点的 L2 视图不一致，此时广播 L1 失效
//     会让其他节点基于「L2 已清」的假设去清本地 L1，但实际 L2 可能并未清理，
//     反而延长了不一致窗口。让调用方感知失败并 slog.Error 上报监控。
//   - L2 失效成功后才广播 L1；publish 失败仅 slog.Error（本地失效已成功，
//     跨节点 L1 失效虽漏掉但本地数据正确，下次 InvalidateByTags 或 TTL 兜底）。
func (m *LazyCacheManager) InvalidateByTags(ctx context.Context, tags ...string) error {
	// Task 14.1: L2 失效失败时不广播 L1 跨节点失效，直接返回 error
	if err := m.cacheManager.Invalidate(ctx, store.WithInvalidateTags(tags)); err != nil {
		return err
	}

	m.eventBusMu.RLock()
	bus := m.eventBus
	m.eventBusMu.RUnlock()

	// Task 13.1: L2 失效成功后才广播 L1；不再静默吞掉 publish error
	if bus != nil && len(tags) > 0 {
		payload, _ := json.Marshal(tags)
		if err := bus.Publish(ctx, pubsub.TopicCacheInvalidation, payload); err != nil {
			slog.Error("pubsub publish failed", "topic", pubsub.TopicCacheInvalidation, "tags", tags, "err", err)
		}
	}

	return nil
}

func (m *LazyCacheManager) InvalidateL1ByTags(ctx context.Context, tags ...string) error {
	if m.l1Cache != nil {
		return m.l1Cache.Invalidate(ctx, store.WithInvalidateTags(tags))
	}
	return nil
}

func (m *LazyCacheManager) SetEventBus(bus pubsub.EventBus) {
	m.eventBusMu.Lock()
	defer m.eventBusMu.Unlock()
	m.eventBus = bus
}

func (m *LazyCacheManager) marshal(val interface{}) ([]byte, error) {
	return json.Marshal(val)
}

func (m *LazyCacheManager) unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (m *LazyCacheManager) assign(src interface{}, dest interface{}) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

// 接口断言（编译期检查）：LazyCacheManager 必须同时实现三个隔离接口。
// 任何方法签名变更导致接口未实现时，编译期即可发现，避免运行时断言失败。
var (
	_ ConfigCache    = (*LazyCacheManager)(nil)
	_ SecurityCache  = (*LazyCacheManager)(nil)
	_ CacheLifecycle = (*LazyCacheManager)(nil)
)
