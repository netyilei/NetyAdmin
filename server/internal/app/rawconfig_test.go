package app

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"NetyAdmin/internal/config"
)

// TestAppRawConfig 验证 App.RawConfig 暴露原始配置树（含基座强类型结构外的
// 下游自有段，如 [ws]）且为深拷贝副本：下游修改自己的段不得污染基座配置
// （LoadFromRaw 场景）。
func TestAppRawConfig(t *testing.T) {
	content := `
base:
  client_router_enabled: true
server:
  port: 9090
ws:
  listen: ":9501"
`
	fDir := t.TempDir()
	require.NoError(t, os.WriteFile(fDir+"/config.yaml", []byte(content), 0o600))

	// Load("config.yaml") 为相对路径——切到临时目录工作
	pwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(fDir))
	defer func() { require.NoError(t, os.Chdir(pwd)) }()

	cfg, err := config.Load("config.yaml")
	require.NoError(t, err)
	a := &App{cfg: cfg}

	raw := a.RawConfig()
	require.NotNil(t, raw)
	require.Contains(t, raw, "ws")

	// 修改下游拿到的副本嵌套段
	raw["ws"].(map[string]any)["listen"] = ":hacked"

	// 重取：内部不受影响
	raw2 := a.RawConfig()
	assert.NotEqual(t, ":hacked", raw2["ws"].(map[string]any)["listen"], "副本修改不得污染基座配置")
}
