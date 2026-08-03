package auth_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	userEntity "NetyAdmin/internal/domain/entity/user"
	authPkg "NetyAdmin/internal/pkg/auth"
)

// mockCacheManager 是 authPkg.CacheManager 的内存实现，用于单测。
// Incr 通过 mutex 模拟 Redis 的 INCR + EXPIRE 行为，保证并发安全
// （用于验证 HandlePasswordWrong 在并发场景下的正确性）。
type mockCacheManager struct {
	mu       sync.Mutex
	counters map[string]int64
	values   map[string]string
	ttl      map[string]time.Duration
	// incrErr 控制下次 Incr 调用是否返回错误（用于测试 fail-closed 分支）
	incrErr error
}

func newMockCacheManager() *mockCacheManager {
	return &mockCacheManager{
		counters: make(map[string]int64),
		values:   make(map[string]string),
		ttl:      make(map[string]time.Duration),
	}
}

func (m *mockCacheManager) Get(_ context.Context, key string, dest interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	val, ok := m.values[key]
	if !ok {
		return errors.New("not found")
	}
	if s, ok := dest.(*string); ok {
		*s = val
	}
	return nil
}

func (m *mockCacheManager) Set(_ context.Context, key string, value interface{}, ttl time.Duration, _ ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := value.(string); ok {
		m.values[key] = s
	}
	m.ttl[key] = ttl
	return nil
}

func (m *mockCacheManager) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, key)
	delete(m.counters, key)
	delete(m.ttl, key)
	return nil
}

func (m *mockCacheManager) Exists(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.values[key]
	if !ok {
		_, ok = m.counters[key]
	}
	return ok, nil
}

// Incr 模拟 Redis 的 INCR + EXPIRE：原子自增并在首次设置时记录 TTL。
// 通过 mutex 保证并发安全，多 goroutine 同时调用不会丢失计数。
func (m *mockCacheManager) Incr(_ context.Context, key string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.incrErr != nil {
		err := m.incrErr
		m.incrErr = nil
		return 0, err
	}
	m.counters[key]++
	val := m.counters[key]
	// 仅在首次设置（值为 1）时记录 TTL（与 Redis 实现一致）
	if val == 1 && ttl > 0 {
		m.ttl[key] = ttl
	}
	return val, nil
}

// getCounter 用于测试断言
func (m *mockCacheManager) getCounter(key string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counters[key]
}

// lockValue 获取 lockKey 的值
func (m *mockCacheManager) lockValue(key string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.values[key]
	return v, ok
}

// ensure mockCacheManager 实现 authPkg.CacheManager 接口（编译期检查）
var _ authPkg.CacheManager = (*mockCacheManager)(nil)

// ============== tokenStore mock 实现 ==============

// inMemoryTokenStore 完整实现 authPkg.UserServiceTokenStore，记录调用次数
type inMemoryTokenStore struct {
	deleteAllCalls int
	createCalls    int
}

func (s *inMemoryTokenStore) Create(_ context.Context, _ *userEntity.AdminToken) error {
	s.createCalls++
	return nil
}

func (s *inMemoryTokenStore) Get(_ context.Context, _, _ string) (*userEntity.AdminToken, error) {
	return nil, errors.New("not implemented")
}

func (s *inMemoryTokenStore) Delete(_ context.Context, _, _ string) error {
	return nil
}

func (s *inMemoryTokenStore) DeleteAll(_ context.Context, _ string) error {
	s.deleteAllCalls++
	return nil
}

var _ authPkg.UserServiceTokenStore = (*inMemoryTokenStore)(nil)

// ============== HandlePasswordWrong 单测 ==============

// TestHandlePasswordWrong_NotLocked 验证：失败次数未达阈值时不锁定，返回剩余次数
func TestHandlePasswordWrong_NotLocked(t *testing.T) {
	tests := []struct {
		name      string
		maxRetry  int
		calls     int
		wantCount int64
	}{
		{"first wrong", 5, 1, 1},
		{"second wrong", 5, 2, 2},
		{"near threshold", 5, 4, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newMockCacheManager()
			cfg := authPkg.LoginLockConfig{
				MaxRetry:     tt.maxRetry,
				LockDuration: 15 * time.Minute,
				RetryTTL:     10 * time.Minute,
			}
			retryKey := "retry:test"
			lockKey := "lock:test"

			var locked bool
			var msg string
			for i := 0; i < tt.calls; i++ {
				locked, msg = authPkg.HandlePasswordWrong(context.Background(), mgr, lockKey, retryKey, cfg)
			}

			assert.False(t, locked, "未达阈值不应锁定")
			assert.Contains(t, msg, "剩余尝试次数")
			assert.Equal(t, tt.wantCount, mgr.getCounter(retryKey), "计数器应等于调用次数")
			_, isLocked := mgr.lockValue(lockKey)
			assert.False(t, isLocked, "不应写入 lockKey")
		})
	}
}

// TestHandlePasswordWrong_Locked 验证：失败次数达阈值时锁定并清理计数
func TestHandlePasswordWrong_Locked(t *testing.T) {
	mgr := newMockCacheManager()
	cfg := authPkg.LoginLockConfig{
		MaxRetry:     3,
		LockDuration: 15 * time.Minute,
		RetryTTL:     10 * time.Minute,
	}
	retryKey := "retry:lock"
	lockKey := "lock:lock"

	var locked bool
	var msg string
	for i := 0; i < cfg.MaxRetry; i++ {
		locked, msg = authPkg.HandlePasswordWrong(context.Background(), mgr, lockKey, retryKey, cfg)
	}
	assert.True(t, locked, "达到阈值应锁定")
	assert.Contains(t, msg, "已被锁定")
	assert.Contains(t, msg, "15m0s")

	// 锁定后 retryKey 应被清理（Delete 在 mock 中删除 counters）
	assert.Equal(t, int64(0), mgr.getCounter(retryKey), "锁定后 retryKey 计数应被清理")

	// lockKey 应被设置
	val, ok := mgr.lockValue(lockKey)
	assert.True(t, ok, "lockKey 应被写入")
	assert.Equal(t, "1", val)
}

// TestHandlePasswordWrong_IncrFailsFailClosed 验证：Incr 失败时 fail-closed 直接锁定
func TestHandlePasswordWrong_IncrFailsFailClosed(t *testing.T) {
	mgr := newMockCacheManager()
	mgr.incrErr = errors.New("redis unavailable")

	cfg := authPkg.LoginLockConfig{
		MaxRetry:     5,
		LockDuration: 15 * time.Minute,
		RetryTTL:     10 * time.Minute,
	}
	retryKey := "retry:fail"
	lockKey := "lock:fail"

	locked, msg := authPkg.HandlePasswordWrong(context.Background(), mgr, lockKey, retryKey, cfg)

	assert.True(t, locked, "Incr 失败应 fail-closed 锁定")
	assert.Contains(t, msg, "已被锁定")

	// lockKey 应被设置
	val, ok := mgr.lockValue(lockKey)
	assert.True(t, ok)
	assert.Equal(t, "1", val)
}

// TestHandlePasswordWrong_ConcurrentAtomicity 验证：并发调用 HandlePasswordWrong 时计数原子性
// 模拟场景：多个 goroutine 同时密码错误，应正确递增计数，不应丢失或重复
func TestHandlePasswordWrong_ConcurrentAtomicity(t *testing.T) {
	mgr := newMockCacheManager()
	cfg := authPkg.LoginLockConfig{
		MaxRetry:     100, // 设高一些，避免在并发中触发锁定
		LockDuration: 15 * time.Minute,
		RetryTTL:     10 * time.Minute,
	}
	retryKey := "retry:conc"
	lockKey := "lock:conc"

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	var lockedCount int64
	var successCount int64
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			locked, _ := authPkg.HandlePasswordWrong(context.Background(), mgr, lockKey, retryKey, cfg)
			if locked {
				atomic.AddInt64(&lockedCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	// 由于 MaxRetry=100，goroutines=50，全部都不会锁定
	assert.Equal(t, int64(0), lockedCount, "未达阈值不应有锁定")
	assert.Equal(t, int64(goroutines), successCount, "全部应成功计数")

	// 计数器最终值应等于 goroutine 数（原子性验证）
	finalCount := mgr.getCounter(retryKey)
	assert.Equal(t, int64(goroutines), finalCount, "并发下计数应等于 goroutine 总数，无丢失无重复")
}

// TestHandlePasswordWrong_ConcurrentLockBoundary 验证：并发下达到阈值边界时的锁定行为
// 模拟场景：goroutines 数 >= MaxRetry，验证触发锁定后 lockKey 必然被写入
func TestHandlePasswordWrong_ConcurrentLockBoundary(t *testing.T) {
	mgr := newMockCacheManager()
	cfg := authPkg.LoginLockConfig{
		MaxRetry:     10,
		LockDuration: 15 * time.Minute,
		RetryTTL:     10 * time.Minute,
	}
	retryKey := "retry:boundary"
	lockKey := "lock:boundary"

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})

	var lockedCount int64
	startTime := time.Now()

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			locked, _ := authPkg.HandlePasswordWrong(context.Background(), mgr, lockKey, retryKey, cfg)
			if locked {
				atomic.AddInt64(&lockedCount, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	elapsed := time.Since(startTime)
	t.Logf("goroutines=%d, MaxRetry=%d, lockedCount=%d, elapsed=%v",
		goroutines, cfg.MaxRetry, atomic.LoadInt64(&lockedCount), elapsed)

	// 验证至少有一个 goroutine 触发锁定（具体数量取决于调度顺序，
	// 但计数器最终值必然 >= MaxRetry，因此必有锁定发生）
	assert.GreaterOrEqual(t, atomic.LoadInt64(&lockedCount), int64(1),
		"达到阈值后应有锁定发生")

	// lockKey 必然被设置
	val, ok := mgr.lockValue(lockKey)
	assert.True(t, ok, "lockKey 应被写入")
	assert.Equal(t, "1", val)
}
