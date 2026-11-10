package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRawConfig 真实 YAML 验证：raw 保留完整原始配置树（含基座未定义的下游段），
// RawConfig() 返回副本（修改不影响内部状态），强类型解析不受影响。
func TestRawConfig(t *testing.T) {
	content := `
server:
  port: 9090
base:
  client_router_enabled: true
biz:
  listen: ":9501"
  origins:
    - "https://a.example.com"
    - "https://b.example.com"
`
	f := t.TempDir() + "/cfg.yaml"
	require.NoError(t, os.WriteFile(f, []byte(content), 0o600))

	cfg, err := Load(f)
	require.NoError(t, err)

	raw := cfg.RawConfig()
	require.NotNil(t, raw)
	assert.InDelta(t, 9090, raw["server"].(map[string]any)["port"], 0.001, "强类型段在 raw 中同样可见")

	bizSeg := raw["biz"].(map[string]any)
	assert.Equal(t, ":9501", bizSeg["listen"], "下游自有段必须可读（LoadFromRaw 场景）")
	assert.Len(t, bizSeg["origins"], 2)

	// 副本语义（深拷贝）：修改返回值的嵌套段不影响内部
	raw["biz"].(map[string]any)["listen"] = ":hacked"
	ws2 := cfg.RawConfig()["biz"].(map[string]any)
	assert.NotEqual(t, ":hacked", ws2["listen"], "RawConfig 必须深拷贝嵌套段")

	// 强类型解析不受 raw 填充影响
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.True(t, cfg.Base.IsClientRouterEnabled())
}

// TestRawConfig_MissingFile 直接验证 parse_cfg 场景：Load 失败时不 panic。
func TestRawConfig_Load(t *testing.T) {
	f := t.TempDir() + "/cfg.yaml"
	require.NoError(t, os.WriteFile(f, []byte("server:\n  port: 9091\nbase:\n  client_router_enabled: false\n"), 0o600))

	cfg, err := Load(f)
	require.NoError(t, err)
	// client_router_enabled 显式 false 必须生效（B1 开关语义）
	assert.False(t, cfg.Base.IsClientRouterEnabled(), "显式 false 必须关闭 client 路由")
	assert.NotNil(t, cfg.RawConfig())
}
