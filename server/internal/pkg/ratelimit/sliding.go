package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// SlidingWindow 通用 Redis ZSET 滑动窗口限流器。
//
// 与 login_ratelimit（pkg/auth）同一套经过验证的算法（ZREMRANGEBYSCORE
// + ZCARD + ZADD），差异在生命周期：login 拆分 Check/Record 两步围绕
// handler 前后调用；本实现为单步 Allow，供 API 中间件在请求入口一次完成，
// 且被拒绝的请求同样计数（抑制重试风暴）。两处语义不同，非重复实现。
type SlidingWindow struct {
	client *redis.Client
	prefix string
}

// NewSlidingWindow 创建滑动窗口限流器；client 为 nil 时返回 nil（调用方不注册）。
func NewSlidingWindow(client *redis.Client, prefix string) *SlidingWindow {
	if client == nil {
		return nil
	}
	return &SlidingWindow{client: client, prefix: prefix}
}

// Allow 判定 key 在 window 窗口内是否放行（max 为阈值）。
// Redis 故障时 fail-open 放行（与 login_ratelimit 一致：限流是保护层，
// 不能因缓存故障阻断业务主路径），仅记录 Warn。
func (l *SlidingWindow) Allow(ctx context.Context, key string, window time.Duration, max int64) bool {
	if l == nil || l.client == nil {
		return true
	}
	fullKey := l.prefix + ":" + key
	now := time.Now().UnixNano()
	cutoff := now - window.Nanoseconds()

	// 1. 清理窗口外记录（失败 fail-open）
	if err := l.client.ZRemRangeByScore(ctx, fullKey, "0", fmt.Sprintf("%d", cutoff)).Err(); err != nil {
		slog.Warn("sliding_window: ZRemRangeByScore failed, fail-open", "key", key, "err", err)
		return true
	}
	// 2. 计数（失败 fail-open）
	count, err := l.client.ZCard(ctx, fullKey).Result()
	if err != nil {
		slog.Warn("sliding_window: ZCard failed, fail-open", "key", key, "err", err)
		return true
	}
	// 3. 无论放行与否都记录本次请求（被拒计数可抑制高频重试）。
	// member 必须全局唯一：同纳秒并发请求若用纯时间戳字符串会被 ZSET 去重导致少计超放
	// （login_ratelimiter 用 ULID 同理）；Score 仍为纳秒时间戳供窗口清理
	member := fmt.Sprintf("%d-%s", now, uuid.NewString())
	if err := l.client.ZAdd(ctx, fullKey, redis.Z{Score: float64(now), Member: member}).Err(); err != nil {
		slog.Warn("sliding_window: ZAdd failed, fail-open", "key", key, "err", err)
		return true
	}
	l.client.Expire(ctx, fullKey, window+time.Second)
	return count < max
}
