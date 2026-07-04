package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/eko/gocache/lib/v4/cache"
	bigcacheStore "github.com/eko/gocache/store/bigcache/v4"
)

// testPayload 是测试用的回源数据结构
type testPayload struct {
	Message string `json:"message"`
}

// newTestManager 构造一个仅含 L1 (BigCache)、不依赖 Redis 的 lazyCacheManager。
// l1Enabled=false 让 FetchFast 委托给 Fetch，集中测试 Fetch 的 singleflight 路径。
// "缓存不存在的 key" 等价于 "热 key 失效瞬间的 cache miss"，是缓存击穿的经典场景。
func newTestManager(t *testing.T) *lazyCacheManager {
	t.Helper()
	bcConfig := bigcache.DefaultConfig(5 * time.Minute)
	bcConfig.Shards = 64
	bigcacheClient, err := bigcache.New(context.Background(), bcConfig)
	if err != nil {
		t.Fatalf("init bigcache: %v", err)
	}
	l1Store := bigcacheStore.NewBigcache(bigcacheClient)
	l1Cache := cache.New[any](l1Store)
	return &lazyCacheManager{
		cacheManager: l1Cache,
		l1Cache:      l1Cache,
		l1Enabled:    false,
		switches:     &DefaultSwitchChecker{},
		prefix:       "test",
		localNX:      sync.Map{},
	}
}

// TestFetch_Singleflight_Concurrent 验证 100 个 goroutine 并发 Fetch 同一个
// 缓存不存在的 key（等价于热 key 失效瞬间），loader 仅被调用 1 次，
// 且所有 caller 都拿到相同的回源值。
func TestFetch_Singleflight_Concurrent(t *testing.T) {
	mgr := newTestManager(t)

	var callCount int64
	// loader 内部 sleep，确保所有 100 个 goroutine 都已进入 flightGroup.Do 等待队列，
	// 而不是后到的 caller 命中 leader 已写入的缓存从而绕过 singleflight 验证路径。
	loader := func() (interface{}, error) {
		atomic.AddInt64(&callCount, 1)
		time.Sleep(100 * time.Millisecond)
		return &testPayload{Message: "hello-from-db"}, nil
	}

	const n = 100
	results := make([]string, n)
	errs := make([]error, n)

	var ready sync.WaitGroup
	var done sync.WaitGroup
	start := make(chan struct{})

	ready.Add(n)
	done.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer done.Done()
			ready.Done()
			<-start
			var result testPayload
			errs[idx] = mgr.Fetch(context.Background(), "hot-key", "test-module", nil, time.Minute, &result, loader)
			results[idx] = result.Message
		}(i)
	}

	// 等待所有 goroutine 就绪后统一放行，最大化并发竞争窗口
	ready.Wait()
	close(start)
	done.Wait()

	if got := atomic.LoadInt64(&callCount); got != 1 {
		t.Errorf("loader called %d times, want 1 (singleflight should dedup concurrent calls)", got)
	}

	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Errorf("goroutine %d got unexpected error: %v", i, errs[i])
			continue
		}
		if results[i] != "hello-from-db" {
			t.Errorf("goroutine %d got %q, want %q", i, results[i], "hello-from-db")
		}
	}
}

// TestFetch_Singleflight_LoaderError 验证 loader 返回 error 时，
// singleflight 把同一个 error 传播给所有 100 个等待中的 caller（不吞错）。
func TestFetch_Singleflight_LoaderError(t *testing.T) {
	mgr := newTestManager(t)

	var callCount int64
	sentinelErr := errors.New("db connection refused")
	loader := func() (interface{}, error) {
		atomic.AddInt64(&callCount, 1)
		time.Sleep(100 * time.Millisecond)
		return nil, sentinelErr
	}

	const n = 100
	errs := make([]error, n)

	var ready sync.WaitGroup
	var done sync.WaitGroup
	start := make(chan struct{})

	ready.Add(n)
	done.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer done.Done()
			ready.Done()
			<-start
			var result testPayload
			errs[idx] = mgr.Fetch(context.Background(), "err-key", "test-module", nil, time.Minute, &result, loader)
		}(i)
	}

	ready.Wait()
	close(start)
	done.Wait()

	if got := atomic.LoadInt64(&callCount); got != 1 {
		t.Errorf("loader called %d times, want 1", got)
	}

	for i := 0; i < n; i++ {
		if errs[i] == nil {
			t.Errorf("goroutine %d expected error, got nil", i)
			continue
		}
		if !errors.Is(errs[i], sentinelErr) {
			t.Errorf("goroutine %d got %v, want %v", i, errs[i], sentinelErr)
		}
	}
}
