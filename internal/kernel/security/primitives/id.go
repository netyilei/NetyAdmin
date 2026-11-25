package primitives

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// Clock 是可注入时钟（测试可替换；禁止包级 time.Now 散落）。
type Clock interface {
	Now() time.Time
}

// SystemClock 系统时钟（生产默认）。
type SystemClock struct{}

// Now 实现 Clock。
func (SystemClock) Now() time.Time { return time.Now() }

// —— 测试/业务可用的固定时钟 ——

// FixedClock 固定时间时钟。
type FixedClock struct{ T time.Time }

// Now 实现 Clock。
func (c FixedClock) Now() time.Time { return c.T }

// ulid 熵源惰性初始化（避免 import 期 rand 失败即 panic；Monotonic 非并发安全，串行队列化）。
var (
	ulidMu   sync.Mutex
	ulidOnce sync.Once
	ulidEnt  *ulid.MonotonicEntropy
)

func ulidEntropy() *ulid.MonotonicEntropy {
	ulidOnce.Do(func() { ulidEnt = ulid.Monotonic(rand.Reader, 0) })
	return ulidEnt
}

// NewULID 生成时间有序 ULID（26 字符 Crockford Base32，架构设计全景表行 34 默认 ID）。
// 单调队列化保证同毫秒内有序；clock 用于注入（测试可替换）。
func NewULID(clock Clock) string {
	ulidMu.Lock()
	defer ulidMu.Unlock()
	ts := ulid.Timestamp(clock.Now().UTC())
	return ulid.MustNew(ts, ulidEntropy()).String()
}

// ParseULID 校验并解析 ULID 字符串。
func ParseULID(s string) (ulid.ULID, error) {
	return ulid.Parse(s)
}
