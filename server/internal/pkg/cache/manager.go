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

type LazyCacheManager interface {
	// Fetch 模式B（标准模式）：L2 (Redis) → L3 (DB 回源)
	// 如果 L1 全局开启，则走 L1+L2 chain 读取，但 L2 命中时不会回填 L1
	// 需要自动回填 L1 请使用 FetchFast（模式A）
	Fetch(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error

	// FetchFast 模式A（极速模式）：L1 (BigCache) → L2 (Redis) → L3 (DB 回源)
	// L1 关闭时自动降级为模式B（纯 L2）
	// 失效统一走 InvalidateByTags
	FetchFast(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error

	// InvalidateByTags 根据标签批量失效所有关联 Key (如果是集群模式，会通过 Redis Pub/Sub 同步失效)
	InvalidateByTags(ctx context.Context, tags ...string) error

	// Set 写入一个缓存项，带过期时间。可选 tags 用于 InvalidateByTags 批量失效。
	// 不带 tags 的调用方保持原签名兼容（variadic 零值等同旧行为）。
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration, tags ...string) error
	// SetFast 强制写入 L1+L2（模式A），带过期时间和 tags
	SetFast(ctx context.Context, key string, value interface{}, tags []string, ttl time.Duration) error
	// SetNX 仅在 Key 不存在时写入 (原子操作，模式B)
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
	// Get 直接读取一个缓存项（模式B）
	Get(ctx context.Context, key string, v interface{}) error
	// GetFast 强制读取 L1→L2（模式A），L2 命中回填 L1 时带 tags
	// ttl 用于 L1 回填的过期时间，与 L2 一致
	GetFast(ctx context.Context, key string, tags []string, ttl time.Duration, v interface{}) error
	// Delete 删除一个缓存项（模式B）
	Delete(ctx context.Context, key string) error
	// Exists 判断一个缓存项是否存在（模式B）
	Exists(ctx context.Context, key string) (bool, error)
	// Incr 原子自增计数器，并在首次设置时配置 TTL。
	// 仅在 Redis（L2）上操作，BigCache（L1）不支持 INCR。
	// Redis 不可用时返回 error（fail-closed，不允许跳过原子计数）。
	// 返回自增后的当前值。
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)

	// InvalidateL1ByTags 仅失效本地 L1 缓存（由 PubSubBus TopicCacheInvalidation 订阅者调用）
	InvalidateL1ByTags(ctx context.Context, tags ...string) error

	// SetEventBus 注入 PubSubBus 实例（解决循环依赖：CacheManager 先于 EventBus 创建）
	SetEventBus(bus pubsub.EventBus)

	// IsCacheEnabled 检查指定模块的缓存开关是否开启
	IsCacheEnabled(moduleName string) bool

	// GetRedisClient 获取底层 Redis 客户端
	GetRedisClient() *redis.Client
}

type SwitchChecker interface {
	IsCacheEnabled(moduleName string) bool
}

type lazyCacheManager struct {
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

	mgr := &lazyCacheManager{
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
func (m *lazyCacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration, tags ...string) error {
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
func (m *lazyCacheManager) l2() cache.CacheInterface[any] {
	if m.l2Cache != nil {
		return m.l2Cache
	}
	return m.l1Cache
}

// getRaw 只读 L2 (Redis)，绝不碰 L1（铁律）。
// L2 未启用时通过 l2() 降级到 l1Cache 本地兜底。
func (m *lazyCacheManager) getRaw(ctx context.Context, key string) ([]byte, error) {
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

// Delete 删除 L2 (Redis) 中的缓存项，绝不碰 L1（铁律）。
// L2 未启用时通过 l2() 降级到 l1Cache 本地兜底。
func (m *lazyCacheManager) Delete(ctx context.Context, key string) error {
	fullKey := m.buildKey(key)
	return m.l2().Delete(ctx, fullKey)
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
func (m *lazyCacheManager) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
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

func (m *lazyCacheManager) FetchFast(ctx context.Context, key string, moduleName string, tags []string, ttl time.Duration, v interface{}, loader func() (interface{}, error)) error {
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

func (m *lazyCacheManager) SetFast(ctx context.Context, key string, value interface{}, tags []string, ttl time.Duration) error {
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

func (m *lazyCacheManager) GetFast(ctx context.Context, key string, tags []string, ttl time.Duration, v interface{}) error {
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

func (m *lazyCacheManager) GetRedisClient() *redis.Client {
	return m.redisClient
}

func (m *lazyCacheManager) IsCacheEnabled(moduleName string) bool {
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
func (m *lazyCacheManager) InvalidateByTags(ctx context.Context, tags ...string) error {
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

func (m *lazyCacheManager) InvalidateL1ByTags(ctx context.Context, tags ...string) error {
	if m.l1Cache != nil {
		return m.l1Cache.Invalidate(ctx, store.WithInvalidateTags(tags))
	}
	return nil
}

func (m *lazyCacheManager) SetEventBus(bus pubsub.EventBus) {
	m.eventBusMu.Lock()
	defer m.eventBusMu.Unlock()
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
