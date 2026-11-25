// l2_redis.go：go-redis → L2Client 适配（Cache 平面 L2 面）。
package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewL2RedisClient 创建适配了 L2Client 的 go-redis 客户端（bootstrap 注入）。
// MaxRetries=0：L2 失败按一致性模式决策（event/bounded bypass、strong fail），不做隐藏重试。
func NewL2RedisClient(addr string) L2Client {
	return l2Redis{cli: redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  1 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		MaxRetries:   0,
	})}
}

type l2Redis struct {
	cli *redis.Client
}

func (a l2Redis) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := a.cli.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (a l2Redis) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return a.cli.Set(ctx, key, val, ttl).Err()
}

func (a l2Redis) Del(ctx context.Context, keys ...string) error {
	return a.cli.Del(ctx, keys...).Err()
}

func (a l2Redis) Incr(ctx context.Context, key string) (int64, error) {
	return a.cli.Incr(ctx, key).Result()
}
