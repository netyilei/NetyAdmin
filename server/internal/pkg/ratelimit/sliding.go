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

// slidingWindowScript 单脚本原子完成"清理窗口外→计数→记录本次→续期"。
//
// 必须原子：分步 ZCARD/ZADD 在并发下存在 check-then-act 竞态——多个请求
// 同时读到 count<max 全部放行（运行时测试实测 100 并发限 10 超放至 11~45）。
// 返回加入前的成员数，调用方以 count < max 判定放行。
// member 由调用方保证唯一（uuid 后缀）：同纳秒纯时间戳会被 ZSET 去重少计。
var slidingWindowScript = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '0', ARGV[1])
local count = redis.call('ZCARD', KEYS[1])
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[3])
redis.call('PEXPIRE', KEYS[1], ARGV[4])
return count
`)

// counterKey 是计数 ZSET key 的唯一构造点：prefix + 维度键 + 窗口 + 阈值。
// 同一维度键可能被多条规则共用，只按维度键共享 ZSET 会让不同窗口/阈值的
// 规则互相污染计数，且短窗口规则的 PEXPIRE 会把长窗口计数一并缩短。
func counterKey(prefix, key string, window time.Duration, max int64) string {
	return fmt.Sprintf("%s:%s:%d:%d", prefix, key, window.Nanoseconds(), max)
}

// Allow 判定 key 在 window 窗口内是否放行（max 为阈值）。
// 被拒绝的请求同样计数（抑制高频重试）。Redis 故障时 fail-open 放行
// （与 login_ratelimit 一致：限流是保护层，不能因缓存故障阻断业务主路径）。
//
// 同参数不同作用域的规则（如同 window/max 但 paths 不同）仍需调用方
// 在 key 中带规则标识（见 middleware.NewAPIRateLimit）。
func (l *SlidingWindow) Allow(ctx context.Context, key string, window time.Duration, max int64) bool {
	if l == nil || l.client == nil {
		return true
	}
	now := time.Now().UnixNano()
	count, err := slidingWindowScript.Run(ctx, l.client,
		[]string{counterKey(l.prefix, key, window, max)},
		now-window.Nanoseconds(), // ARGV[1] 窗口下界（cutoff）
		now,                      // ARGV[2] score（纳秒时间戳）
		fmt.Sprintf("%d-%s", now, uuid.NewString()), // ARGV[3] 唯一 member
		(window + time.Second).Milliseconds(),       // ARGV[4] key TTL（毫秒）
	).Int64()
	if err != nil {
		slog.Warn("sliding_window: script failed, fail-open", "key", key, "err", err)
		return true
	}
	return count < max
}
