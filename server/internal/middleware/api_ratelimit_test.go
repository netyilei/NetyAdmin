package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"NetyAdmin/internal/config"
)

// TestRuleScope 验证限流规则作用域：语义相同的规则稳定复用计数，
// 同 window/max 但 paths/name 不同的规则必须隔离（避免跨规则污染）。
func TestRuleScope(t *testing.T) {
	base := config.APIRateLimitRule{
		Name:      "rule-a",
		Dimension: "ip",
		Window:    config.Duration(time.Minute),
		Max:       10,
		Paths:     []string{"/a"},
	}
	otherPaths := base
	otherPaths.Paths = []string{"/b"}
	otherName := base
	otherName.Name = "rule-b"

	tests := []struct {
		name      string
		a, b      config.APIRateLimitRule
		wantEqual bool
	}{
		{"same rule", base, base, true},
		{"same window/max, different paths", base, otherPaths, false},
		{"same window/max, different name", base, otherName, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantEqual {
				assert.Equal(t, ruleScope(tt.a), ruleScope(tt.b))
				return
			}
			assert.NotEqual(t, ruleScope(tt.a), ruleScope(tt.b))
		})
	}
}
