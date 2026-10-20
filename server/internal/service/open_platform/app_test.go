package open_platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestAppCacheTTL 验证 App 信息缓存 TTL 的兜底规则：
// 显式 >0 按秒换算；0/负数使用默认 5 分钟（避免 L2 永久陈旧）。
func TestAppCacheTTL(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		want    time.Duration
	}{
		{"explicit ttl", 300, 5 * time.Minute},
		{"zero falls back to default", 0, defaultAppCacheTTL},
		{"negative falls back to default", -1, defaultAppCacheTTL},
		{"one second", 1, time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, appCacheTTL(tt.seconds))
		})
	}
}
