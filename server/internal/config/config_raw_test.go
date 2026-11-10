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

// TestBaseRouterSwitches 锁定 [base] 双开关契约：
// nil（未配置）= true 向后兼容，显式 false 必须生效。
// client_router_enabled 与 admin_router_enabled 语义对称、互不影响。
func TestBaseRouterSwitches(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantClient bool
		wantAdmin  bool
	}{
		{name: "全部缺省_向后兼容", yaml: "server:\n  port: 9090\n", wantClient: true, wantAdmin: true},
		{name: "两开关显式false", yaml: "base:\n  client_router_enabled: false\n  admin_router_enabled: false\n", wantClient: false, wantAdmin: false},
		{name: "两开关显式true", yaml: "base:\n  client_router_enabled: true\n  admin_router_enabled: true\n", wantClient: true, wantAdmin: true},
		{name: "仅admin关闭", yaml: "base:\n  admin_router_enabled: false\n", wantClient: true, wantAdmin: false},
		{name: "仅client关闭", yaml: "base:\n  client_router_enabled: false\n", wantClient: false, wantAdmin: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := t.TempDir() + "/cfg.yaml"
			require.NoError(t, os.WriteFile(f, []byte(tt.yaml), 0o600))
			cfg, err := Load(f)
			require.NoError(t, err)
			assert.Equal(t, tt.wantClient, cfg.Base.IsClientRouterEnabled(), "client_router_enabled 语义偏差")
			assert.Equal(t, tt.wantAdmin, cfg.Base.IsAdminRouterEnabled(), "admin_router_enabled 语义偏差")
		})
	}
}
