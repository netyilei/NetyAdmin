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
	client.Del(ctx, "swtest:k1", "swtest:k2", "swtest:k3")

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
	card, _ := client.ZCard(ctx, "swtest:k3").Result()
	if okCount != 10 || card != 100 {
		t.Errorf("T3 100并发/限10 放行=%d ZCARD=%d 期望10/100（member 去重会少计）", okCount, card)
	}

	var nilL *SlidingWindow
	if !nilL.Allow(ctx, "k4", time.Minute, 1) {
		t.Error("T4 nil limiter 应放行")
	}
	broken := NewSlidingWindow(redis.NewClient(&redis.Options{Addr: "localhost:9999"}), "x")
	if !broken.Allow(ctx, "k4", time.Minute, 1) {
		t.Error("T5 故障应 fail-open")
	}
	client.Del(ctx, "swtest:k1", "swtest:k2", "swtest:k3")
}
