package utils

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

var entropy = ulid.Monotonic(rand.Reader, 0)

// NewULID 生成一个新的 ULID 字符串
func NewULID() string {
	t := time.Now()
	id := ulid.MustNew(ulid.Timestamp(t), entropy)
	return id.String()
}
