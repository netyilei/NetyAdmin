package ipac

import (
	"testing"
	"time"
)

// TestParseExpiredAt 覆盖过期时间两级解析：
// RFC3339（带时区，前端主路径）精确解析；裸 DateTime 按服务器本地时区；
// 非法格式返回错误（原先静默吞错导致规则永不过期语义漂移）。
func TestParseExpiredAt(t *testing.T) {
	t.Run("rfc3339 with offset", func(t *testing.T) {
		got, err := parseExpiredAt("2026-09-18T08:30:00+08:00")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		want := time.Date(2026, 9, 18, 0, 30, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("got %v, want utc-instant %v", got, want)
		}
	})

	t.Run("legacy datetime interpreted as local", func(t *testing.T) {
		got, err := parseExpiredAt("2026-09-18 08:30:00")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		want := time.Date(2026, 9, 18, 8, 30, 0, 0, time.Local)
		if !got.Equal(want) {
			t.Fatalf("got %v, want local %v", got, want)
		}
	})

	t.Run("invalid rejected", func(t *testing.T) {
		if _, err := parseExpiredAt("tomorrow"); err == nil {
			t.Fatal("invalid input should be rejected")
		}
		if _, err := parseExpiredAt(""); err == nil {
			t.Fatal("empty input should be rejected")
		}
	})
}
