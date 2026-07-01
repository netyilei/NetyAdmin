// Package ratelimit 提供基于 github.com/ulule/limiter/v3 的限流能力。
//
// 设计目标：
//   - 替换原先散落在 cache.LazyCacheManager 中的 RateLimit 方法（含 Lua 脚本 + 本地令牌桶）。
//   - 单一职责：本包只做限流，不关心缓存语义。
//   - 双后端：Redis 启用时走分布式固定窗口（跨节点一致），否则降级为内存存储。
//
// 算法说明：
//   - ulule/limiter 内部采用固定窗口（fixed window）计数。
//   - 原自研实现是令牌桶（token bucket），支持 burst capacity。
//   - 为保持语义等价，将窗口大小设为 capacity 个请求，窗口周期设为 capacity/rate 秒，
//     即"窗口内最多 capacity 次，平均速率 rate/秒"，等价于令牌桶的持续补充速率 + 突发上限。
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/ulule/limiter/v3"
	memoryStore "github.com/ulule/limiter/v3/drivers/store/memory"
	redisStore "github.com/ulule/limiter/v3/drivers/store/redis"
)

// Limiter 限流器，封装 ulule/limiter 并提供"令牌桶语义"接口。
// 内部根据 Redis 是否可用自动选择分布式或本地内存后端。
type Limiter struct {
	store  limiter.Store
	prefix string
}

// New 创建限流器实例。
//
// 参数：
//   - redisClient: Redis 客户端，非 nil 时启用分布式限流；nil 时降级为进程内内存限流。
//   - prefix: 限流 Key 前缀（用于多实例隔离，与缓存前缀保持一致）。
func New(redisClient *redis.Client, prefix string) *Limiter {
	var store limiter.Store
	if redisClient != nil {
		// 分布式：Redis 固定窗口，跨节点一致
		s, err := redisStore.NewStore(redisClient)
		if err != nil {
			// Redis store 创建失败（理论上极少发生），降级为内存
			store = memoryStore.NewStore()
		} else {
			store = s
		}
	} else {
		// 单机：内存固定窗口
		store = memoryStore.NewStore()
	}

	return &Limiter{
		store:  store,
		prefix: prefix,
	}
}

// Allow 判断指定 key 在当前窗口内是否允许通过一次请求。
//
// 参数：
//   - ctx: 上下文
//   - key: 限流标识（如 appKey），不含前缀，由本方法拼接
//   - rate: 持续补充速率（每秒允许的请求数）
//   - capacity: 突发上限（窗口内最大请求数）
//
// 返回 true 表示放行，false 表示已触发限流。
// 当 rate 或 capacity 非正时直接放行（与原实现一致）。
func (l *Limiter) Allow(ctx context.Context, key string, rate, capacity int) (bool, error) {
	if rate <= 0 || capacity <= 0 {
		return true, nil
	}

	// 构造令牌桶等价的固定窗口：
	//   窗口大小 = capacity 次请求
	//   窗口周期 = capacity / rate 秒（保证平均速率为 rate/秒）
	period := time.Duration(capacity) * time.Second / time.Duration(rate)
	if period < time.Second {
		period = time.Second // 最小窗口 1 秒，避免除零或过小
	}
	rl := limiter.Rate{
		Period:    period,
		Limit:     int64(capacity),
		Formatted: fmt.Sprintf("%d-S", capacity), // 标识，便于调试
	}

	// 用独立 Limiter 实例保证该 (period, limit) 组合的计数隔离
	instance := limiter.New(l.store, rl)

	// 拼接完整 key：prefix:ratelimit:key（与原 cache.buildKey 行为一致）
	fullKey := "ratelimit:" + key
	if l.prefix != "" {
		fullKey = l.prefix + ":" + fullKey
	}

	limiterCtx, err := instance.Get(ctx, fullKey)
	if err != nil {
		return false, err
	}
	return !limiterCtx.Reached, nil
}
