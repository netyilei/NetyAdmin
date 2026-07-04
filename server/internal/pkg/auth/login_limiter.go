// Package auth 提供登录端点 IP 维度限流器（与 pkg/ratelimit 令牌桶相互独立）。
//
// 设计目标（fix-fundamental-design-flaws Task 3）：
//   - 仅作用于 admin /auth/login + /auth/refreshToken 与 client /user/login + /user/refresh-token
//     登录相关路由，不影响其他接口（不全局注册中间件）。
//   - 算法：Redis ZSET 滑动窗口（ZADD + ZREMRANGEBYSCORE + ZCARD）。
//     相比固定窗口，滑动窗口可更平滑地限制突发流量，避免窗口边界处的双倍流量问题。
//   - 失败开放（fail-open）：Redis 未配置或不可用时，限流器降级为 no-op，
//     Check 返回 true、Record 返回 nil，不阻断登录关键路径。
//     原因：登录是用户进入系统的关键入口，限流器故障导致用户无法登录比放宽限流更严重。
//
// 与 pkg/ratelimit 的关系：
//   - pkg/ratelimit 基于 ulule/limiter 提供通用固定窗口限流（用于开放平台 AppKey 维度）。
//   - 本包专门针对登录端点，使用滑动窗口算法，语义独立、不共享状态。
//   - 两套限流器互不影响，可独立调优。
package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"NetyAdmin/internal/pkg/utils"
)

// LoginLimiter 登录端点 IP 维度限流器接口。
//
// 调用契约：
//   - Check：滑动窗口内已记录的尝试次数 < Max 时返回 (true, nil)，否则返回 (false, nil)。
//     仅读取计数，不写入新记录。Redis 不可用时返回 (true, nil) — fail-open。
//   - Record：在滑动窗口内追加一条记录（ZADD，score=now，member=唯一 ID）。
//     Redis 不可用时返回 nil — fail-open。
//
// 典型用法（middleware/login_ratelimit.go）：
//
//	ok, err := limiter.Check(ctx, ip)
//	if err != nil || !ok { return CodeTooManyRequests }
//	c.Next()                      // 执行 handler
//	_ = limiter.Record(ctx, ip)   // 不论成功失败都计数
type LoginLimiter interface {
	Check(ctx context.Context, ip string) (bool, error)
	Record(ctx context.Context, ip string) error
}

// noopLoginLimiter 是 Redis 未配置时的空实现（fail-open）。
// Check 永远返回 true、Record 永远返回 nil，等同于无限流。
type noopLoginLimiter struct{}

func (noopLoginLimiter) Check(ctx context.Context, ip string) (bool, error) {
	return true, nil
}

func (noopLoginLimiter) Record(ctx context.Context, ip string) error {
	return nil
}

// redisLoginLimiter 基于 Redis ZSET 的滑动窗口实现。
//
// Key 结构：<prefix>:login_ratelimit:<ip>
// ZSET member：唯一 ID（ULID），保证同一毫秒内的多次 ZADD 不会被去重
// ZSET score：unix 毫秒时间戳，用于 ZREMRANGEBYSCORE 滑动窗口淘汰
//
// 算法流程：
//   - Check：ZREMRANGEBYSCORE(0, now-window) 淘汰过期 → ZCARD 统计当前窗口内计数 → count < Max
//   - Record：ZADD(now, ulid)
//
// 并发说明：Check 与 Record 不在同一原子事务内，存在「Check 间并发窗口」：
//
//	多个并发请求可能同时通过 Check 后再各自 Record，导致窗口内瞬时计数略超 Max。
//	这对登录端点是可接受的（限流器是「尽力而为」的防爆破机制，而非严格的配额管控）。
//	若需严格原子性，可改用 Lua 脚本，但会引入额外复杂度，本任务范围内不做。
type redisLoginLimiter struct {
	client *redis.Client
	prefix string
	window time.Duration
	max    int
}

// NewLoginLimiter 根据是否提供 Redis 客户端选择具体实现。
//
// 参数：
//   - redisClient: Redis 客户端，nil 时返回 noopLoginLimiter（fail-open）
//   - prefix: Key 前缀（与缓存层 prefix 一致，多实例隔离）
//   - window: 滑动窗口时长；零值兜底为 1 分钟
//   - max: 窗口内最大尝试次数；零值兜底为 10
func NewLoginLimiter(redisClient *redis.Client, prefix string, window time.Duration, max int) LoginLimiter {
	if redisClient == nil {
		return noopLoginLimiter{}
	}
	if window <= 0 {
		window = time.Minute
	}
	if max <= 0 {
		max = 10
	}
	return &redisLoginLimiter{
		client: redisClient,
		prefix: prefix,
		window: window,
		max:    max,
	}
}

// key 拼接限流 Key：<prefix>:login_ratelimit:<ip>
func (l *redisLoginLimiter) key(ip string) string {
	if l.prefix == "" {
		return fmt.Sprintf("login_ratelimit:%s", ip)
	}
	return fmt.Sprintf("%s:login_ratelimit:%s", l.prefix, ip)
}

// Check 读取当前窗口内已记录的尝试次数，未达阈值时返回 true。
// 任何 Redis 错误均视为 fail-open（返回 true），避免登录关键路径被 Redis 故障阻断。
func (l *redisLoginLimiter) Check(ctx context.Context, ip string) (bool, error) {
	now := time.Now()
	cutoff := now.Add(-l.window).UnixMilli()

	key := l.key(ip)

	// 1. 淘汰窗口外的过期记录（score < cutoff）
	//    min="0", max=cutoff（含），保证旧记录被清理
	if err := l.client.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", cutoff)).Err(); err != nil {
		// Redis 故障 → fail-open
		slog.Warn("login_limiter: ZRemRangeByScore failed, fail-open", "ip", ip, "err", err)
		return true, nil
	}

	// 2. 统计当前窗口内计数
	count, err := l.client.ZCard(ctx, key).Result()
	if err != nil {
		slog.Warn("login_limiter: ZCard failed, fail-open", "ip", ip, "err", err)
		return true, nil
	}

	return int(count) < l.max, nil
}

// Record 在当前时间戳追加一条记录。
// 使用 ULID 作为 ZSET member，保证同一 IP 同一毫秒内的多次 ZADD 不会被去重丢失。
// 任何 Redis 错误均视为 fail-open（返回 nil），不阻断后续登录流程。
//
// 同时为 ZSET 设置 TTL=window，避免冷门 IP 的 ZSET 长期驻留 Redis。
// TTL 设置失败仅 Warn，不影响主流程（ZSET 仍可正常工作，仅内存回收稍延迟）。
func (l *redisLoginLimiter) Record(ctx context.Context, ip string) error {
	now := time.Now()
	nowMS := now.UnixMilli()

	key := l.key(ip)
	member := utils.NewULID()

	// ZADD 记录本次尝试
	if err := l.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(nowMS),
		Member: member,
	}).Err(); err != nil {
		slog.Warn("login_limiter: ZAdd failed, fail-open", "ip", ip, "err", err)
		return nil
	}

	// 设置/刷新 ZSET 过期时间，避免冷门 IP 残留
	// 用 window + 一点缓冲（10%），确保窗口内的最新记录不会被提前过期
	ttl := l.window + l.window/10
	if ttl < time.Second {
		ttl = time.Second
	}
	if err := l.client.Expire(ctx, key, ttl).Err(); err != nil {
		slog.Warn("login_limiter: Expire failed", "ip", ip, "err", err)
	}
	return nil
}
