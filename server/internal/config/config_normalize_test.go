package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNormalize_CacheAndMultiNode 验证缓存/多节点约束的自动归一化：
//   - multi_node=true 且 Redis 关闭 → 自动启用 L2；
//   - L1/L2 双关 → 自动启用 L1；
//   - multi_node=true 且显式 bus=memory → 自动切换 redis；
//   - 显式配置不被误覆盖；重复调用幂等。
func TestNormalize_CacheAndMultiNode(t *testing.T) {
	tests := []struct {
		name        string
		multiNode   bool
		redisOn     bool
		l1On        bool
		busDriver   string
		wantRedisOn bool
		wantL1On    bool
		wantBus     string
	}{
		{"multi-node forces L2 and both-off rule forces L1", true, false, false, "", true, true, ""},
		{"multi-node with explicit L1 off and Redis on stays L2-only", true, true, false, "", true, false, ""},
		{"single node both off forces L1", false, false, false, "", false, true, ""},
		{"single node explicit L1 off with Redis on stays L2-only", false, true, false, "", true, false, ""},
		{"single node L1 on with Redis off unchanged", false, false, true, "", false, true, ""},
		{"multi-node memory bus switches to redis", true, true, true, "memory", true, true, "redis"},
		{"single node memory bus kept", false, true, true, "memory", true, true, "memory"},
		{"multi-node explicit redis bus kept", true, true, true, "redis", true, true, "redis"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.Server.MultiNode = tt.multiNode
			cfg.Redis.Enabled = tt.redisOn
			cfg.Redis.L1Enabled = tt.l1On
			cfg.Bus.Driver = tt.busDriver

			cfg.normalize()
			assert.Equal(t, tt.wantRedisOn, cfg.Redis.Enabled, "redis.enabled")
			assert.Equal(t, tt.wantL1On, cfg.Redis.L1Enabled, "l1_enabled")
			assert.Equal(t, tt.wantBus, cfg.Bus.Driver, "bus.driver")

			// 幂等：再次归一化不得改变结果
			cfg.normalize()
			assert.Equal(t, tt.wantRedisOn, cfg.Redis.Enabled)
			assert.Equal(t, tt.wantL1On, cfg.Redis.L1Enabled)
			assert.Equal(t, tt.wantBus, cfg.Bus.Driver)
		})
	}
}

// TestNormalize_APIRateLimitRules 验证非法限流规则在加载边界被丢弃并保留合法规则：
// max<=0 或 window<=0 的规则无法表达限流语义（max=0 整组 429、window=0 恒放行）。
func TestNormalize_APIRateLimitRules(t *testing.T) {
	tests := []struct {
		name      string
		rules     []APIRateLimitRule
		wantNames []string
	}{
		{"empty rules untouched", nil, nil},
		{"valid rules kept", []APIRateLimitRule{
			{Name: "a", Dimension: "ip", Window: Duration(time.Minute), Max: 10},
			{Name: "b", Dimension: "ip", Window: Duration(time.Hour), Max: 100},
		}, []string{"a", "b"}},
		{"zero max dropped", []APIRateLimitRule{
			{Name: "bad", Dimension: "ip", Window: Duration(time.Minute), Max: 0},
			{Name: "good", Dimension: "ip", Window: Duration(time.Minute), Max: 5},
		}, []string{"good"}},
		{"negative max dropped", []APIRateLimitRule{
			{Name: "bad", Dimension: "ip", Window: Duration(time.Minute), Max: -1},
		}, nil},
		{"zero window dropped", []APIRateLimitRule{
			{Name: "bad", Dimension: "ip", Window: Duration(0), Max: 5},
		}, nil},
		{"negative window dropped", []APIRateLimitRule{
			{Name: "bad", Dimension: "ip", Window: Duration(-time.Second), Max: 5},
		}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.APIRateLimit.Rules = tt.rules

			cfg.normalize()

			var got []string
			for _, r := range cfg.APIRateLimit.Rules {
				got = append(got, r.Name)
			}
			assert.Equal(t, tt.wantNames, got)

			// 幂等：再次归一化不得继续改变结果
			cfg.normalize()
			assert.Len(t, cfg.APIRateLimit.Rules, len(tt.wantNames))
		})
	}
}
