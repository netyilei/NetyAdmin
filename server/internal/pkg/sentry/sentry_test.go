package sentry

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// compileIgnorePatterns 将 pattern 列表编译为 regex，模拟 sentry-go 的 IgnoreTransactions 匹配。
// sentry-go integrations.go:206 的逻辑是 regex.Match || strings.Contains(pattern.String())，
// 即先尝试 regex 全匹配，失败再退化为子串包含。这里复刻该语义用于测试断言。
func compileIgnorePatterns(t *testing.T, patterns []string) []*regexp.Regexp {
	t.Helper()
	regs := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			t.Fatalf("invalid regex pattern %q: %v", p, err)
		}
		regs = append(regs, re)
	}
	return regs
}

// matchesIgnore 复刻 sentry-go IgnoreTransactionsIntegration.processor 的匹配语义。
func matchesIgnore(t *testing.T, patterns []string, suspect string) bool {
	t.Helper()
	regs := compileIgnorePatterns(t, patterns)
	for _, re := range regs {
		if re.MatchString(suspect) || strings.Contains(suspect, re.String()) {
			return true
		}
	}
	return false
}

func TestBuildIgnoreTransactions(t *testing.T) {
	tests := []struct {
		name string
		user []string
		want []string
	}{
		{
			name: "用户未配置，仅默认清单",
			user: nil,
			want: []string{` /health`, ` /favicon`, ` /assets/`},
		},
		{
			name: "用户追加自定义事务",
			user: []string{` /api/v1/ping`, ` /metrics`},
			want: []string{` /health`, ` /favicon`, ` /assets/`, ` /api/v1/ping`, ` /metrics`},
		},
		{
			name: "用户与默认重复时去重",
			user: []string{` /health`, ` /metrics`, ``}, // 含空串
			want: []string{` /health`, ` /favicon`, ` /assets/`, ` /metrics`},
		},
		{
			name: "用户全为空串时仅返回默认",
			user: []string{"", ""},
			want: []string{` /health`, ` /favicon`, ` /assets/`},
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
	// 默认清单必须覆盖 K8s 探针与前端静态资源噪声源（带前导空格锚定 method/path 分隔符）
	defaults := defaultIgnoreTransactions()
	assert.Contains(t, defaults, ` /health`)
	assert.Contains(t, defaults, ` /favicon`)
	assert.Contains(t, defaults, ` /assets/`)
}

// TestIgnoreTransactions_NoFalsePositive 验证默认清单不会误伤含 health/assets/favicon
// 子串的业务路由事务名。这是 BUG-1 修复的核心回归测试。
func TestIgnoreTransactions_NoFalsePositive(t *testing.T) {
	patterns := buildIgnoreTransactions(nil)

	// 真实噪声事务应被过滤（sentrygin 事务名格式 "<METHOD> <FullPath>"")
	shouldFilter := []string{
		"GET /health",
		"GET /favicon.svg",
		"GET /assets/index-abc123.js",
		"GET /assets/css/main.css",
	}
	for _, txn := range shouldFilter {
		t.Run("过滤:"+txn, func(t *testing.T) {
			assert.True(t, matchesIgnore(t, patterns, txn), "应过滤噪声事务: %s", txn)
		})
	}

	// 业务路由事务名即使含 health/assets/favicon 子串也不应被误过滤
	shouldNotFilter := []string{
		"GET /admin/v1/assets-management/list",   // assets 子串在 path 中间
		"GET /admin/v1/health-check",             // health 子串在 path 中间
		"POST /admin/v1/favicon-configs",         // favicon 子串在 path 中间
		"GET /admin/v1/users/health-records/123", // 嵌套业务路由
		"GET /api/v1/articles",                   // 无关业务路由
	}
	for _, txn := range shouldNotFilter {
		t.Run("不误伤:"+txn, func(t *testing.T) {
			assert.False(t, matchesIgnore(t, patterns, txn), "不应误伤业务事务: %s", txn)
		})
	}
}
