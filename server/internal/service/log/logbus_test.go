package log

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	logEntity "NetyAdmin/internal/domain/entity/log"
)

// testEntry 是容量控制测试用的最小 LogEntry 实现。
type testEntry struct {
	size int
	lt   logEntity.LogType
}

func (e testEntry) GetLogType() logEntity.LogType {
	if e.lt == "" {
		return logEntity.LogTypeOpen
	}
	return e.lt
}
func (e testEntry) GetCreatedAt() time.Time { return time.Now() }
func (e testEntry) GetRequestID() string    { return "" }
func (e testEntry) GetSizeBytes() int       { return e.size }

func newTestLogBus(maxEntries int, maxBytes int64) *logBusService {
	return &logBusService{
		configs: map[logEntity.LogType]BucketConfig{
			logEntity.LogTypeOpen: {Priority: logEntity.PriorityP2, SizeThreshold: 100},
		},
		buffers: map[logEntity.LogType][]logEntity.LogEntry{
			logEntity.LogTypeOpen: {},
		},
		globalMaxEntries: maxEntries,
		globalMaxBytes:   maxBytes,
	}
}

// TestLogBusByteCapacity 验证 global_max_bytes_mb 真正生效：
// 超过字节上限时拒绝追加；驱逐旧条目后恢复容量；flush 后计数归零。
func TestLogBusByteCapacity(t *testing.T) {
	b := newTestLogBus(1000, 1000)

	assert.True(t, b.tryAppend(testEntry{size: 600}))
	assert.Equal(t, int64(600), b.totalBytes)

	// 600 + 500 > 1000：字节上限拒绝
	assert.False(t, b.tryAppend(testEntry{size: 500}))
	assert.Equal(t, 1, b.totalEntries)

	// 驱逐最旧条目后重新可容纳
	b.evictOldest(logEntity.PriorityP2)
	assert.Equal(t, 0, b.totalEntries)
	assert.Equal(t, int64(0), b.totalBytes)
	assert.True(t, b.tryAppend(testEntry{size: 500}))
	assert.Equal(t, int64(500), b.totalBytes)

	// flush 后计数与字节数归零
	b.flushAll()
	assert.Equal(t, 0, b.totalEntries)
	assert.Equal(t, int64(0), b.totalBytes)
}

// TestLogBusEntryCapacity 验证条数上限仍然生效。
func TestLogBusEntryCapacity(t *testing.T) {
	b := newTestLogBus(1, 1<<20)

	assert.True(t, b.tryAppend(testEntry{size: 10}))
	assert.False(t, b.tryAppend(testEntry{size: 10}))

	b.evictOldest(logEntity.PriorityP2)
	assert.True(t, b.tryAppend(testEntry{size: 10}))
}

// TestLogBusEvictsOnlyTargetPriority 验证驱逐是精确优先级：
// 驱逐 P2 时绝不触碰 P0（错误日志），P0 只能在同步写路径处理。
func TestLogBusEvictsOnlyTargetPriority(t *testing.T) {
	b := &logBusService{
		configs: map[logEntity.LogType]BucketConfig{
			logEntity.LogTypeError: {Priority: logEntity.PriorityP0, SizeThreshold: 100},
			logEntity.LogTypeOpen:  {Priority: logEntity.PriorityP2, SizeThreshold: 100},
		},
		buffers: map[logEntity.LogType][]logEntity.LogEntry{
			logEntity.LogTypeError: {},
			logEntity.LogTypeOpen:  {},
		},
		globalMaxEntries: 1000,
		globalMaxBytes:   1 << 20,
	}

	assert.True(t, b.tryAppend(testEntry{size: 10, lt: logEntity.LogTypeError}))
	assert.True(t, b.tryAppend(testEntry{size: 20, lt: logEntity.LogTypeOpen}))

	// 驱逐 P2：只移除开放平台日志，错误日志保留
	b.evictOldest(logEntity.PriorityP2)
	assert.Equal(t, 1, len(b.buffers[logEntity.LogTypeError]), "P0 不得被 P2 驱逐")
	assert.Equal(t, 0, len(b.buffers[logEntity.LogTypeOpen]))
	assert.Equal(t, int64(10), b.totalBytes)

	// 再驱逐 P1：无 P1 条目，P0 仍保留
	b.evictOldest(logEntity.PriorityP1)
	assert.Equal(t, 1, len(b.buffers[logEntity.LogTypeError]))
	assert.Equal(t, int64(10), b.totalBytes)
}
