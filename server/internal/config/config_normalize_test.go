package config

import (
	"testing"

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
