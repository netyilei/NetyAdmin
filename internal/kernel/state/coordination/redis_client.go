// redis_client.go：go-redis 客户端 → Client 接口适配（接口保留假实现注入面）。
package coordination

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client 是协调原语依赖的最小 Redis 面（见 coordination.go）。
type Client interface {
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	SetTTL(ctx context.Context, key, value string, ttl time.Duration) error
	Exists(ctx context.Context, key string) (bool, error)
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
	IncrBy(ctx context.Context, key string, incr int64) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Del(ctx context.Context, keys ...string) error
	Ping(ctx context.Context) error
}

// NewRedisClient 创建适配了 Client 接口的 go-redis 客户端（bootstrap 注入）。
// MaxRetries=0：协调面快速失败，不可用时的行为由 Policy 决策（不做隐藏重试）。
func NewRedisClient(addr string) Client {
	return redisAdapter{cli: redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  1 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		MaxRetries:   0,
	})}
}

// redisAdapter 实现 Client（go-redis 命令对象 → 值/错误二元组）。
type redisAdapter struct {
	cli *redis.Client
}

func (a redisAdapter) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	return a.cli.SetNX(ctx, key, value, ttl).Result()
}

func (a redisAdapter) SetTTL(ctx context.Context, key, value string, ttl time.Duration) error {
	return a.cli.Set(ctx, key, value, ttl).Err()
}

func (a redisAdapter) Exists(ctx context.Context, key string) (bool, error) {
	n, err := a.cli.Exists(ctx, key).Result()
	return n > 0, err
}

func (a redisAdapter) Del(ctx context.Context, keys ...string) error {
	return a.cli.Del(ctx, keys...).Err()
}

func (a redisAdapter) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	return a.cli.Eval(ctx, script, keys, args...).Result()
}

func (a redisAdapter) IncrBy(ctx context.Context, key string, incr int64) (int64, error) {
	return a.cli.IncrBy(ctx, key, incr).Result()
}

func (a redisAdapter) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return a.cli.Expire(ctx, key, ttl).Result()
}

func (a redisAdapter) Ping(ctx context.Context) error {
	return a.cli.Ping(ctx).Err()
}
