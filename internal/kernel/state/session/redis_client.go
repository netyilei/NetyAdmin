// redis_client.go：go-redis → session.Client 适配。
package session

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient 创建适配了 Client 接口的 go-redis 客户端（bootstrap 注入）。
// MaxRetries=0：快速失败，降级决策归 Policy。
func NewRedisClient(addr string) Client {
	return redisAdapter{cli: redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  1 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		MaxRetries:   0,
	})}
}

type redisAdapter struct {
	cli *redis.Client
}

func (a redisAdapter) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	return a.cli.SetNX(ctx, key, value, ttl).Result()
}

func (a redisAdapter) Exists(ctx context.Context, key string) (bool, error) {
	n, err := a.cli.Exists(ctx, key).Result()
	return n > 0, err
}

func (a redisAdapter) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := a.cli.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (a redisAdapter) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return a.cli.Set(ctx, key, val, ttl).Err()
}

func (a redisAdapter) Del(ctx context.Context, keys ...string) error {
	return a.cli.Del(ctx, keys...).Err()
}

func (a redisAdapter) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	return a.cli.Eval(ctx, script, keys, args...).Result()
}

func (a redisAdapter) Ping(ctx context.Context) error {
	return a.cli.Ping(ctx).Err()
}
