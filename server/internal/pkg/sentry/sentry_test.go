package sentry

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/getsentry/sentry-go"
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

// --- BeforeSend PII 脱敏测试（Task 6） ---
//
// sentry-go v0.47.0 的 Event 已移除 Extra 字段，自定义附加数据写入 event.Contexts
// （map[string]Context，Context = map[string]any）。以下测试用 event.Contexts["extra"]
// 承载"Extra map"语义，验证 scrubEvent 行为。

// TestScrubEvent_ExtraMap 验证 Contexts["extra"] 中命中敏感 key 的 value 被替换为
// [REDACTED]，其余 value 保留。对应 SubTask 6.4 Test 1。
func TestScrubEvent_ExtraMap(t *testing.T) {
	event := &sentry.Event{
		Contexts: map[string]sentry.Context{
			"extra": {
				"password": "abc",
				"other":    "keep",
			},
		},
	}
	scrubEvent(event)
	ctx := event.Contexts["extra"]
	assert.Equal(t, "[REDACTED]", ctx["password"])
	assert.Equal(t, "keep", ctx["other"])
}

// TestScrubEvent_NestedMap 验证递归 scrub 嵌套 map[string]any。对应 SubTask 6.4 Test 2。
func TestScrubEvent_NestedMap(t *testing.T) {
	event := &sentry.Event{
		Contexts: map[string]sentry.Context{
			"extra": {
				"user": map[string]any{
					"app_secret": "xyz",
					"name":       "alice",
				},
			},
		},
	}
	scrubEvent(event)
	userMap := event.Contexts["extra"]["user"].(map[string]any)
	assert.Equal(t, "[REDACTED]", userMap["app_secret"])
	assert.Equal(t, "alice", userMap["name"])
}

// TestScrubEvent_UserEmail 验证 event.User.Email 与 User.IPAddress 直接替换为 [REDACTED]。
// 对应 SubTask 6.4 Test 3。
func TestScrubEvent_UserEmail(t *testing.T) {
	event := &sentry.Event{
		User: sentry.User{
			Email:     "test@example.com",
			IPAddress: "1.2.3.4",
			ID:        "01H8XKJG1Z2Y3W4V5S6T7Q8PA9",
		},
	}
	scrubEvent(event)
	assert.Equal(t, "[REDACTED]", event.User.Email)
	assert.Equal(t, "[REDACTED]", event.User.IPAddress)
	// ULID 等 ID 不应被脱敏（仅 Email/IPAddress 直接清空）
	assert.Equal(t, "01H8XKJG1Z2Y3W4V5S6T7Q8PA9", event.User.ID)
}

// TestScrubEvent_RequestData 验证 Request.Data（JSON 字符串）中的敏感字段被脱敏。
// 对应 SubTask 6.4 Test 4。
func TestScrubEvent_RequestData(t *testing.T) {
	event := &sentry.Event{
		Request: &sentry.Request{
			Data: `{"refresh_token":"xxx","user_id":"u123"}`,
		},
	}
	scrubEvent(event)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(event.Request.Data), &parsed); err != nil {
		t.Fatalf("Request.Data scrub 后非合法 JSON: %v", err)
	}
	assert.Equal(t, "[REDACTED]", parsed["refresh_token"])
	assert.Equal(t, "u123", parsed["user_id"])
}

// TestIsSensitiveKey 验证 case-insensitive 子串匹配覆盖任务要求的全部关键词。
func TestIsSensitiveKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"password", true},
		{"PASSWORD", true},      // case-insensitive
		{"user_password", true}, // 子串
		{"App_Secret", true},    // 命中 secret（app_secret 含 secret 子串）
		{"appsecret", true},     // 任务明示关键词
		{"app_key", true},
		{"access_key", true},
		{"refresh_token", true},
		{"REFRESH_TOKEN", true},
		{"token", true},
		{"authTokenV2", true}, // 命中 token 子串，case-insensitive
		{"username", false},   // 不含任何敏感关键词
		{"email", false},
		{"name", false},
		{"user_id", false},
		{"id", false},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			assert.Equal(t, c.want, isSensitiveKey(c.key))
		})
	}
}

// TestScrubEvent_NilEvent 验证 nil event 不 panic（防御性）。
func TestScrubEvent_NilEvent(t *testing.T) {
	assert.Nil(t, scrubEvent(nil))
}

// TestScrubEvent_RequestData_NonJSON 验证非合法 JSON 的 Request.Data 原样返回。
func TestScrubEvent_RequestData_NonJSON(t *testing.T) {
	raw := "not-a-json-body&refresh_token=xxx"
	event := &sentry.Event{
		Request: &sentry.Request{Data: raw},
	}
	scrubEvent(event)
	assert.Equal(t, raw, event.Request.Data)
}
