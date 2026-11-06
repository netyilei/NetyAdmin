package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRuntimeManager 构造只接 Redis 的 LazyCacheManager，用于验证依赖 Lua/Redis
// 语义的方法（GetAndDelete / Incr）。Redis 不可达时自动 Skip。
func newRuntimeManager(t *testing.T) (*LazyCacheManager, *redis.Client, string) {
	t.Helper()
	addr := os.Getenv("NETYADMIN_TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		t.Skipf("redis 不可达: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	prefix := fmt.Sprintf("netyadmin:cachetest:%d", time.Now().UnixNano())
	return &LazyCacheManager{redisClient: client, prefix: prefix}, client, prefix
}

func clearRuntimeKeys(t *testing.T, client *redis.Client, ctx context.Context, prefix string) {
	t.Helper()
	keys, err := client.Keys(ctx, prefix+":*").Result()
	require.NoError(t, err)
	if len(keys) > 0 {
		require.NoError(t, client.Del(ctx, keys...).Err())
	}
}

// TestGetAndDeleteRuntime 真 Redis 验证原生 GETDEL 单命令原子消费：
// 缺失返回 redis.Nil、命中原子取出并删除、并发消费恰好一个成功。
func TestGetAndDeleteRuntime(t *testing.T) {
	mgr, client, prefix := newRuntimeManager(t)
	ctx := context.Background()
	t.Cleanup(func() { clearRuntimeKeys(t, client, ctx, prefix) })

	t.Run("missing returns redis.Nil", func(t *testing.T) {
		err := mgr.GetAndDelete(ctx, "missing", new(string))
		assert.ErrorIs(t, err, redis.Nil)
	})

	t.Run("hit consumes value", func(t *testing.T) {
		require.NoError(t, client.Set(ctx, prefix+":hit", `"secret"`, time.Minute).Err())
		var got string
		require.NoError(t, mgr.GetAndDelete(ctx, "hit", &got))
		assert.Equal(t, "secret", got)
		assert.ErrorIs(t, mgr.GetAndDelete(ctx, "hit", new(string)), redis.Nil, "second read must be empty")
	})

	t.Run("concurrent consume has exactly one winner", func(t *testing.T) {
		require.NoError(t, client.Set(ctx, prefix+":once", `"once"`, time.Minute).Err())
		const workers = 32
		var wg sync.WaitGroup
		var mu sync.Mutex
		var ok, nilErrs, otherErrs int
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var v string
				err := mgr.GetAndDelete(ctx, "once", &v)
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					ok++
				case errors.Is(err, redis.Nil):
					nilErrs++
				default:
					otherErrs++
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, 1, ok, "only one consumer may win")
		assert.Equal(t, workers-1, nilErrs)
		assert.Equal(t, 0, otherErrs)
	})
}

// TestIncrRuntimeTTL 真 Redis 验证单条 Lua 的 INCR + TTL 补挂：
// 首次自增即挂 TTL、已有 TTL 不续期、历史遗留的无 TTL key 在下一次自增时补挂。
func TestIncrRuntimeTTL(t *testing.T) {
	mgr, client, prefix := newRuntimeManager(t)
	ctx := context.Background()
	t.Cleanup(func() { clearRuntimeKeys(t, client, ctx, prefix) })

	t.Run("first incr attaches ttl", func(t *testing.T) {
		v, err := mgr.Incr(ctx, "first", time.Minute)
		require.NoError(t, err)
		assert.Equal(t, int64(1), v)
		ttl := client.PTTL(ctx, prefix+":first").Val()
		assert.Positive(t, ttl, "TTL must be attached on first increment")
		assert.LessOrEqual(t, ttl, time.Minute)
	})

	t.Run("existing ttl is not renewed", func(t *testing.T) {
		key := prefix + ":existing"
		require.NoError(t, client.Set(ctx, key, 1, 2*time.Second).Err())
		time.Sleep(20 * time.Millisecond)

		v, err := mgr.Incr(ctx, "existing", time.Minute)
		require.NoError(t, err)
		assert.Equal(t, int64(2), v)
		assert.LessOrEqual(t, client.PTTL(ctx, key).Val(), 2*time.Second,
			"an existing TTL must not be extended to the caller's 1m")
	})

	t.Run("legacy key without ttl is repaired", func(t *testing.T) {
		key := prefix + ":legacy"
		require.NoError(t, client.Set(ctx, key, 5, 0).Err())

		v, err := mgr.Incr(ctx, "legacy", time.Minute)
		require.NoError(t, err)
		assert.Equal(t, int64(6), v)
		assert.Positive(t, client.PTTL(ctx, key).Val(), "legacy key must get a TTL on next increment")
	})
}
