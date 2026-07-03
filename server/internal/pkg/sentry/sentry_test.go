package sentry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildIgnoreTransactions(t *testing.T) {
	tests := []struct {
		name string
		user []string
		want []string
	}{
		{
			name: "用户未配置，仅默认清单",
			user: nil,
			want: []string{`/health`, `/favicon`, `/assets/`},
		},
		{
			name: "用户追加自定义事务",
			user: []string{`/api/v1/ping`, `/metrics`},
			want: []string{`/health`, `/favicon`, `/assets/`, `/api/v1/ping`, `/metrics`},
		},
		{
			name: "用户与默认重复时去重",
			user: []string{`/health`, `/metrics`, ``}, // 含空串
			want: []string{`/health`, `/favicon`, `/assets/`, `/metrics`},
		},
		{
			name: "用户全为空串时仅返回默认",
			user: []string{"", ""},
			want: []string{`/health`, `/favicon`, `/assets/`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildIgnoreTransactions(tt.user)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultIgnoreTransactions(t *testing.T) {
	// 默认清单必须覆盖 K8s 探针与前端静态资源噪声源
	defaults := defaultIgnoreTransactions()
	assert.Contains(t, defaults, `/health`)
	assert.Contains(t, defaults, `/favicon`)
	assert.Contains(t, defaults, `/assets/`)
}
