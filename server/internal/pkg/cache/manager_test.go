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

// newTestManager 构造一个仅含 L1 (BigCache)、不依赖 Redis 的 LazyCacheManager。
// l1Enabled=false 让 FetchFast 委托给 Fetch，集中测试 Fetch 的 singleflight 路径。
// "缓存不存在的 key" 等价于 "热 key 失效瞬间的 cache miss"，是缓存击穿的经典场景。
func newTestManager(t *testing.T) *LazyCacheManager {
	t.Helper()
	bcConfig := bigcache.DefaultConfig(5 * time.Minute)
	bcConfig.Shards = 64
	bigcacheClient, err := bigcache.New(context.Background(), bcConfig)
	if err != nil {
		t.Fatalf("init bigcache: %v", err)
	}
	l1Store := bigcacheStore.NewBigcache(bigcacheClient)
	l1Cache := cache.New[any](l1Store)
	return &LazyCacheManager{
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

// TestL2Helper_FallbackToL1 验证铁律降级链：
// 当 l2Cache 为 nil（Redis 未启用）时，非 Fast 方法（Set/Get/Delete）降级用 l1Cache 本地兜底。
// 这保证无 Redis 环境下 token 等安全数据仍能缓存，避免直接打 DB。
func TestL2Helper_FallbackToL1(t *testing.T) {
	mgr := newTestManager(t) // l2Cache=nil, l1Cache=BigCache
	ctx := context.Background()

	// l2() 应返回 l1Cache（降级兜底）
	if got := mgr.l2(); got != mgr.l1Cache {
		t.Fatalf("l2() with nil l2Cache should return l1Cache for fallback, got different instance")
	}

	// Set（非 Fast）应写入 l1Cache（降级兜底）
	if err := mgr.Set(ctx, "token:user1", "1", time.Minute); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get（非 Fast）应从 l1Cache 读出（降级兜底）
	var val string
	if err := mgr.Get(ctx, "token:user1", &val); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "1" {
		t.Errorf("Get got %q, want %q", val, "1")
	}

	// Delete（非 Fast）应从 l1Cache 删除
	if err := mgr.Delete(ctx, "token:user1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if err := mgr.Get(ctx, "token:user1", &val); err == nil {
		t.Errorf("Get after Delete should fail (not found), got nil error")
	}
}

// TestL2Helper_PrefersL2 验证铁律核心：
// 当 l2Cache 非 nil 时，l2() 返回 l2Cache（绝不碰 L1）。
// 由于测试环境无真实 Redis，这里只验证 l2() 的选择逻辑，不验证实际读写。
func TestL2Helper_PrefersL2(t *testing.T) {
	mgr := newTestManager(t)
	// 注入一个假的 l2Cache（用另一个 BigCache 实例模拟）
	bcConfig := bigcache.DefaultConfig(5 * time.Minute)
	bcConfig.Shards = 64
	bcClient, _ := bigcache.New(context.Background(), bcConfig)
	fakeL2 := cache.New[any](bigcacheStore.NewBigcache(bcClient))
	mgr.l2Cache = fakeL2

	// l2() 应返回 l2Cache，不是 l1Cache
	if got := mgr.l2(); got != fakeL2 {
		t.Fatalf("l2() with non-nil l2Cache should return l2Cache (never L1), got different instance")
	}
	if got := mgr.l2(); got == mgr.l1Cache {
		t.Fatalf("l2() must NOT return l1Cache when l2Cache is available (violates 铁律)")
	}
}

// TestFetch_L2Unavailable_FallbackL1ThenDB 验证完整降级链：
// L2 不可用（l2Cache=nil）→ 写入/读取降级到 L1 → L1 miss → DB 回源（loader）→ 回填 L1。
// 这是铁律降级链的端到端验证。
func TestFetch_L2Unavailable_FallbackL1ThenDB(t *testing.T) {
	mgr := newTestManager(t) // l2Cache=nil, l1Cache=BigCache
	ctx := context.Background()

	var loaderCalls int32
	loader := func() (interface{}, error) {
		atomic.AddInt32(&loaderCalls, 1)
		return &testPayload{Message: "from-db"}, nil
	}

	// 第一次 Fetch：L1 miss（空缓存）→ loader 回源 DB → 回填 L1
	var result testPayload
	if err := mgr.Fetch(ctx, "cfg:app1", "test", nil, time.Minute, &result, loader); err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if result.Message != "from-db" {
		t.Errorf("got %q, want %q", result.Message, "from-db")
	}
	if atomic.LoadInt32(&loaderCalls) != 1 {
		t.Errorf("loader should be called once on cold miss, got %d", loaderCalls)
	}

	// 第二次 Fetch：L1 hit（第一次回填的）→ 不调 loader
	var result2 testPayload
	if err := mgr.Fetch(ctx, "cfg:app1", "test", nil, time.Minute, &result2, loader); err != nil {
		t.Fatalf("second Fetch failed: %v", err)
	}
	if result2.Message != "from-db" {
		t.Errorf("got %q, want %q", result2.Message, "from-db")
	}
	if atomic.LoadInt32(&loaderCalls) != 1 {
		t.Errorf("loader should NOT be called again on L1 hit, got %d calls", loaderCalls)
	}

	// 验证数据确实在 L1（l2Cache=nil 时 l2() 返回 l1Cache）
	if got := mgr.l2(); got != mgr.l1Cache {
		t.Fatalf("l2() should return l1Cache when l2Cache=nil (fallback)")
	}
}
