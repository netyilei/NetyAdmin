package utils

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// NewULID 生成一个新的 ULID 字符串
func NewULID() string {
	t := time.Now()
	entropy := ulid.Monotonic(rand.Reader, 0)
	id := ulid.MustNew(ulid.Timestamp(t), entropy)
	return id.String()
}

// ULIDToTime 从 ULID 字符串中提取时间戳
func ULIDToTime(idStr string) (time.Time, error) {
	id, err := ulid.Parse(idStr)
	if err != nil {
		return time.Time{}, err
	}
	return ulid.Time(id.Time()), nil
}
