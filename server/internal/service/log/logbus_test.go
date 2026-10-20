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
}

func (e testEntry) GetLogType() logEntity.LogType { return logEntity.LogTypeOpen }
func (e testEntry) GetCreatedAt() time.Time       { return time.Now() }
func (e testEntry) GetRequestID() string          { return "" }
func (e testEntry) GetSizeBytes() int             { return e.size }

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
