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

// TestGetAndDeleteRuntime 真 Redis 验证 Lua GET+DEL：
// 原子消费（并发仅一个成功）、二次取值为 redis.Nil、缺失返回 redis.Nil。
// 覆盖 Redis 6.0（无 GETDEL）兼容路径。
func TestGetAndDeleteRuntime(t *testing.T) {
	mgr, client, prefix := newRuntimeManager(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if keys, err := client.Keys(ctx, prefix+":*").Result(); err == nil && len(keys) > 0 {
			client.Del(ctx, keys...)
		}
	})

	key := prefix + ":gad"
	if err := client.Set(ctx, key, `"secret"`, time.Minute).Err(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var got string
	if err := mgr.GetAndDelete(ctx, "gad", &got); err != nil {
		t.Fatalf("GetAndDelete: %v", err)
	}
	if got != "secret" {
		t.Fatalf("got = %q, want secret", got)
	}
	if err := mgr.GetAndDelete(ctx, "gad", new(string)); !errors.Is(err, redis.Nil) {
		t.Fatalf("second GetAndDelete: err = %v, want redis.Nil", err)
	}

	// 并发消费同一凭证：Lua 脚本原子执行，必须恰好一个成功。
	key2 := prefix + ":gad-concurrent"
	if err := client.Set(ctx, key2, `"once"`, time.Minute).Err(); err != nil {
		t.Fatalf("seed concurrent: %v", err)
	}
	const workers = 32
	var wg sync.WaitGroup
	var mu sync.Mutex
	var ok, nilErrs, otherErrs int
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var v string
			err := mgr.GetAndDelete(ctx, "gad-concurrent", &v)
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
	if ok != 1 || nilErrs != workers-1 || otherErrs != 0 {
		t.Fatalf("concurrent consume: ok=%d nil=%d other=%d, want 1/%d/0", ok, nilErrs, otherErrs, workers-1)
	}
}

// TestIncrRuntimeTTL 真 Redis 验证单条 Lua 的 INCR + TTL 补挂：
// 首次自增即挂 TTL；已有 TTL 不续期；历史遗留的无 TTL key 在下一次自增时补挂。
func TestIncrRuntimeTTL(t *testing.T) {
	mgr, client, prefix := newRuntimeManager(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if keys, err := client.Keys(ctx, prefix+":*").Result(); err == nil && len(keys) > 0 {
			client.Del(ctx, keys...)
		}
	})

	v, err := mgr.Incr(ctx, "incr", time.Minute)
	if err != nil || v != 1 {
		t.Fatalf("Incr = %d, %v; want 1, nil", v, err)
	}
	key := prefix + ":incr"
	ttl := client.PTTL(ctx, key).Val()
	if ttl <= 0 || ttl > time.Minute {
		t.Fatalf("TTL = %v, want (0, 1m]", ttl)
	}

	// 后续自增不续期：TTL 只会变小。
	time.Sleep(20 * time.Millisecond)
	v, err = mgr.Incr(ctx, "incr", time.Minute)
	if err != nil || v != 2 {
		t.Fatalf("second Incr = %d, %v; want 2, nil", v, err)
	}
	if ttl2 := client.PTTL(ctx, key).Val(); ttl2 > ttl {
		t.Fatalf("TTL renewed: %v -> %v", ttl, ttl2)
	}

	// 模拟旧版本遗留的无 TTL 计数 key：下一次 Incr 必须补挂 TTL。
	legacy := prefix + ":incr-legacy"
	if err := client.Set(ctx, legacy, 5, 0).Err(); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	v, err = mgr.Incr(ctx, "incr-legacy", time.Minute)
	if err != nil || v != 6 {
		t.Fatalf("legacy Incr = %d, %v; want 6, nil", v, err)
	}
	if ttl := client.PTTL(ctx, legacy).Val(); ttl <= 0 {
		t.Fatalf("legacy key TTL not attached: %v", ttl)
	}
}
