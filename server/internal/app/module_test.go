package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chainMarker 构造记录调用名的中间件（测试链序契约用）。
func chainMarker(order *[]string, name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		*order = append(*order, name)
		c.Next()
	}
}

// runChain 按序执行中间件链（gin 上下文外手动调用，c.Next() 对空 pending 队列是 no-op）。
func runChain(t *testing.T, chain []gin.HandlerFunc) {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	for _, mw := range chain {
		mw(c)
	}
}

// TestClientModuleAuthChain 锁定下游 ClientRouterModule auth 组中间件链契约：
// OpenPlatformAuth → api_ratelimit（可选），与基座 ClientRouter.Register 严格一致。
// 缺陷背景：RegisterModule 曾漏挂 api_ratelimit，下游 Module 认证组静默不受
// [api_ratelimit] 约束，与文档「全部认证组」承诺不符（2026-11-11 根修）。
func TestClientModuleAuthChain(t *testing.T) {
	tests := []struct {
		name string
		mkRL func(*[]string) gin.HandlerFunc // nil = 限流未配置
		want []string
	}{
		{name: "限流未配置_仅OpenPlatformAuth", mkRL: nil, want: []string{"openapi"}},
		{name: "限流已配置_链序_openapi_rl", mkRL: func(o *[]string) gin.HandlerFunc { return chainMarker(o, "rl") }, want: []string{"openapi", "rl"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var order []string
			var rl gin.HandlerFunc
			if tt.mkRL != nil {
				rl = tt.mkRL(&order)
			}
			chain := clientModuleAuthChain(chainMarker(&order, "openapi"), rl)
			require.Len(t, chain, len(tt.want))
			runChain(t, chain)
			assert.Equal(t, tt.want, order, "链序必须与基座 ClientRouter.Register 的 authGroup 一致")
		})
	}
}

// TestAdminModulePermissionChain 锁定下游 AdminRouterModule permission 组中间件链契约：
// IPAC → JWT → api_ratelimit（可选）→ Permission，与基座 admin registerV1 严格一致。
// 注：RegisterModule 的 admin 路径不做端到端行为测试——IPAC/JWT/Permission 真实
// 中间件需要完整依赖（IPAC 服务、JWT 密钥、RBAC verifier）才能放行请求，
// 链序契约由本测试在装配函数层锁定，调用点仅 4 个位置参数（diff 可审）。
func TestAdminModulePermissionChain(t *testing.T) {
	tests := []struct {
		name string
		mkRL func(*[]string) gin.HandlerFunc // nil = 限流未配置
		want []string
	}{
		{name: "限流未配置", mkRL: nil, want: []string{"ipac", "jwt", "permission"}},
		{name: "限流已配置_插在JWT与Permission之间", mkRL: func(o *[]string) gin.HandlerFunc { return chainMarker(o, "rl") }, want: []string{"ipac", "jwt", "rl", "permission"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var order []string
			var rl gin.HandlerFunc
			if tt.mkRL != nil {
				rl = tt.mkRL(&order)
			}
			chain := adminModulePermissionChain(
				chainMarker(&order, "ipac"),
				chainMarker(&order, "jwt"),
				chainMarker(&order, "permission"),
				rl,
			)
			require.Len(t, chain, len(tt.want))
			runChain(t, chain)
			assert.Equal(t, tt.want, order, "链序必须与基座 admin registerV1 的 permissionGroup 一致")
		})
	}
}

// recClientModule 客户端路由模块（测试用）：仅在 auth 组注册一个探针路由。
type recClientModule struct {
	name      string
	probeHits *int
}

func (m *recClientModule) Name() string                                          { return m.name }
func (m *recClientModule) RegisterClientPublic(_ *gin.RouterGroup, _ RouterDeps) {}

func (m *recClientModule) RegisterClientAuth(g *gin.RouterGroup, _ RouterDeps) {
	g.GET("/ping", func(c *gin.Context) {
		*m.probeHits++
		c.Status(http.StatusOK)
	})
}

// TestRegisterModule_ClientAuthAppliesRateLimit 端到端验证 RegisterModule
// 对下游 ClientRouterModule auth 组挂载完整中间件链（含 api_ratelimit 可选位）：
// 请求命中探针路由时，中间件按「openapi → rl(可选) → handler」顺序执行。
func TestRegisterModule_ClientAuthAppliesRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		withRL    bool
		wantOrder []string
	}{
		{name: "限流未配置_链仅openapi", withRL: false, wantOrder: []string{"openapi"}},
		{name: "限流已配置_链openapi_rl", withRL: true, wantOrder: []string{"openapi", "rl"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var order []string
			var rl gin.HandlerFunc
			if tt.withRL {
				rl = chainMarker(&order, "rl")
			}
			a := &App{
				engine:           gin.New(),
				openPlatformAuth: chainMarker(&order, "openapi"),
				apiRateLimit:     rl,
			}
			hits := 0
			a.RegisterModule(&recClientModule{name: "probe", probeHits: &hits})

			req := httptest.NewRequest(http.MethodGet, "/client/v1/ping", nil)
			w := httptest.NewRecorder()
			a.engine.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, 1, hits, "探针路由应恰好命中一次")
			assert.Equal(t, tt.wantOrder, order, "认证组中间件链序必须含 api_ratelimit 可选位")
		})
	}
}
