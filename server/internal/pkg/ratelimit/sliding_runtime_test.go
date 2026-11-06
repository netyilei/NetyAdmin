package ratelimit

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestSlidingWindowRuntime 真实 Redis 五场景。
// 服务不可达时自动 Skip（CI 无 Redis 即静默跳过，本地开发默认 localhost:6379），
// 地址可用环境变量 NETYADMIN_TEST_REDIS_ADDR 覆盖。
func TestSlidingWindowRuntime(t *testing.T) {
	ctx := context.Background()
	addr := os.Getenv("NETYADMIN_TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("redis 不可达: %v", err)
	}
	l := NewSlidingWindow(client, "swtest")
	clearKeys := func() {
		keys, _ := client.Keys(ctx, "swtest:*").Result()
		if len(keys) > 0 {
			client.Del(ctx, keys...)
		}
	}
	clearKeys()
	defer clearKeys()

	var allowed int
	for i := 0; i < 20; i++ {
		if l.Allow(ctx, "k1", time.Minute, 5) {
			allowed++
		}
	}
	if allowed != 5 {
		t.Errorf("T1 20次/限5 放行=%d 期望5", allowed)
	}

	for i := 0; i < 5; i++ {
		l.Allow(ctx, "k2", time.Millisecond, 5)
	}
	time.Sleep(50 * time.Millisecond)
	if !l.Allow(ctx, "k2", time.Millisecond, 5) {
		t.Error("T2 窗口过期后应恢复放行")
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var okCount int
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow(ctx, "k3", time.Minute, 10) {
				mu.Lock()
				okCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	card, _ := client.ZCard(ctx, "swtest:k3:60000000000:10").Result()
	if okCount != 10 || card != 100 {
		t.Errorf("T3 100并发/限10 放行=%d ZCARD=%d 期望10/100（member 去重会少计）", okCount, card)
	}

	// T6 同维度键、不同窗口/阈值属于两条规则，必须隔离计数。
	// 若共享 ZSET，第一条规则已写入 4 个成员，第二条规则 max=2 会误拒。
	for i := 0; i < 3; i++ {
		if !l.Allow(ctx, "k6", time.Minute, 3) {
			t.Fatalf("T6 第一条规则第 %d 次应放行", i+1)
		}
	}
	if l.Allow(ctx, "k6", time.Minute, 3) {
		t.Error("T6 第一条规则第 4 次应拒绝")
	}
	if !l.Allow(ctx, "k6", time.Hour, 2) {
		t.Error("T6 不同窗口/阈值的规则共享了 ZSET（计数污染）")
	}

	var nilL *SlidingWindow
	if !nilL.Allow(ctx, "k4", time.Minute, 1) {
		t.Error("T4 nil limiter 应放行")
	}
	broken := NewSlidingWindow(redis.NewClient(&redis.Options{Addr: "localhost:9999"}), "x")
	if !broken.Allow(ctx, "k4", time.Minute, 1) {
		t.Error("T5 故障应 fail-open")
	}
}
