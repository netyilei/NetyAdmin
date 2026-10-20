package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIncr_LocalFallback 验证 Redis 未启用时 Incr 走进程内计数：
// 计数递进、Delete 重置、TTL 过期后重新从 1 开始。
func TestIncr_LocalFallback(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	v, err := m.Incr(ctx, "k", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), v)

	v, err = m.Incr(ctx, "k", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(2), v)

	// Delete 同时清理本地计数与缓存项
	require.NoError(t, m.Delete(ctx, "k"))
	v, err = m.Incr(ctx, "k", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), v)
}

// TestIncr_LocalFallback_Expires 验证 TTL 到期后计数归零（与 Redis EXPIRE 语义一致）。
func TestIncr_LocalFallback_Expires(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	v, err := m.Incr(ctx, "k-expire", 30*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, int64(1), v)

	v, err = m.Incr(ctx, "k-expire", 30*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, int64(2), v)

	time.Sleep(80 * time.Millisecond)

	v, err = m.Incr(ctx, "k-expire", 30*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, int64(1), v, "TTL 过期后应从 1 重新计数")
}

// TestIncr_LocalFallback_Concurrent 验证并发自增无丢失。
func TestIncr_LocalFallback_Concurrent(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	const workers = 100
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, err := m.Incr(ctx, "k-concurrent", time.Minute)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	v, err := m.Incr(ctx, "k-concurrent", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(workers+1), v)
}
