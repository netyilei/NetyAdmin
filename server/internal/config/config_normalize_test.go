package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNormalize_CacheAndMultiNode 验证缓存/多节点约束的自动归一化：
//   - multi_node=true 且 Redis 关闭 → 自动启用 L2；
//   - 非多节点且 L1/L2 双关 → 自动启用 L1；
//   - 显式配置不被覆盖；重复调用幂等。
func TestNormalize_CacheAndMultiNode(t *testing.T) {
	tests := []struct {
		name        string
		multiNode   bool
		redisOn     bool
		l1On        bool
		wantRedisOn bool
		wantL1On    bool
	}{
		{"multi-node forces L2 and both-off rule forces L1", true, false, false, true, true},
		{"multi-node with explicit L1 off stays L2-only", true, false, true, true, true},
		{"single node both off forces L1", false, false, false, false, true},
		{"single node explicit L1 off with Redis on stays L2-only", false, true, false, true, false},
		{"single node L1 on with Redis off unchanged", false, false, true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.Server.MultiNode = tt.multiNode
			cfg.Redis.Enabled = tt.redisOn
			cfg.Redis.L1Enabled = tt.l1On

			cfg.normalize()
			assert.Equal(t, tt.wantRedisOn, cfg.Redis.Enabled, "redis.enabled")
			assert.Equal(t, tt.wantL1On, cfg.Redis.L1Enabled, "l1_enabled")

			// 幂等：再次归一化不得改变结果
			cfg.normalize()
			assert.Equal(t, tt.wantRedisOn, cfg.Redis.Enabled)
			assert.Equal(t, tt.wantL1On, cfg.Redis.L1Enabled)
		})
	}
}
