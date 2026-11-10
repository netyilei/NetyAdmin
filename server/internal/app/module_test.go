package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"NetyAdmin/internal/config"
	clientRouterPkg "NetyAdmin/internal/interface/client/http/router"
	"NetyAdmin/internal/middleware"
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
				rl,
				chainMarker(&order, "permission"),
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

// recTypedAppModule 最小 clientRouterV1.ClientModuleRouter 实现（typed 注册委托测试用）。
type recTypedAppModule struct{ name string }

func (m *recTypedAppModule) Name() string { return m.name }

func (m *recTypedAppModule) RegisterPublic(g *gin.RouterGroup) {
	g.GET("/hello", func(c *gin.Context) { c.Status(http.StatusOK) })
}

func (m *recTypedAppModule) RegisterAuth(g *gin.RouterGroup) {
	g.GET("/profile", func(c *gin.Context) { c.Status(http.StatusOK) })
}

// TestRegisterTypedAuthModule_Guards 锁定 App.RegisterTypedAuthModule 的 fail-fast
// 守卫契约：Run 后 panic / client_router_enabled=false panic / clientRouter 未装配
// panic；happy path 委托给基座 clientRouter（typed 路由经 RegisterTypedRoutes 可挂载）。
func TestRegisterTypedAuthModule_Guards(t *testing.T) {
	mod := &recTypedAppModule{name: "tech"}
	accessor := (&middleware.AuthMiddleware{}).UserClaimsAccessorFor("tech")

	t.Run("Run后调用panic", func(t *testing.T) {
		a := &App{started: true, cfg: &config.Config{}}
		assert.Panics(t, func() { a.RegisterTypedAuthModule("tech", mod, accessor) })
	})

	t.Run("client_router关闭时panic", func(t *testing.T) {
		disabled := false
		a := &App{cfg: &config.Config{Base: config.BaseConfig{ClientRouterEnabled: &disabled}}}
		assert.Panics(t, func() { a.RegisterTypedAuthModule("tech", mod, accessor) })
	})

	t.Run("clientRouter未装配panic", func(t *testing.T) {
		a := &App{cfg: &config.Config{}}
		assert.Panics(t, func() { a.RegisterTypedAuthModule("tech", mod, accessor) })
	})

	t.Run("happy路径委托clientRouter", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		// NewClientRouter 各构造器纯存储，nil handler/service 安全（本测试不调 Register）；
		// authMW 必须非 nil——RegisterTypedRoutes 构造 TypedUserJWTAuth 闭包需解引用 receiver。
		cr := clientRouterPkg.NewClientRouter(nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, &middleware.AuthMiddleware{})
		a := &App{cfg: &config.Config{}, clientRouter: cr}
		a.RegisterTypedAuthModule("tech", mod, accessor)

		engine := gin.New()
		cr.RegisterTypedRoutes(engine) // Run 期挂载动作
		assertRouteMounted(t, engine, "/client/v1/tech/public/hello")
	})

	t.Run("nil_accessor复用users表校验链", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		cr := clientRouterPkg.NewClientRouter(nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, &middleware.AuthMiddleware{})
		a := &App{cfg: &config.Config{}, clientRouter: cr, authMW: &middleware.AuthMiddleware{}}
		a.RegisterTypedAuthModule("tech", mod, nil) // nil = UserClaimsAccessorFor("tech")，不 panic 即委托成功

		engine := gin.New()
		cr.RegisterTypedRoutes(engine)
		assertRouteMounted(t, engine, "/client/v1/tech/public/hello")
	})

	t.Run("authMW未装配且accessor为nil时panic", func(t *testing.T) {
		// nil accessor 依赖 a.authMW——未装配时必须立即 fail-fast，而非延迟到 Run 期解引用 panic
		a := &App{cfg: &config.Config{}, clientRouter: &clientRouterPkg.ClientRouter{}}
		assert.Panics(t, func() { a.RegisterTypedAuthModule("tech", mod, nil) })
	})
}

// TestRegisterTypedAuthModule_UserTypeValidation 锁定 userType 校验契约：
// 空串 / 路由元字符 / 非小写格式 / 基座内置路径段（保留字）都在注册期 fail-fast panic。
// 缺陷背景：无校验时空串使 typed 路由挂到 /client/v1 根（accessor 归一化后成为
// 无签名的默认用户路由）、":tech" 创建参数通配段；「同名必 panic」的文档承诺
// 对 gin 仅精确路径碰撞才 panic 的现实而言过强——保留字在调用点强制成立。
func TestRegisterTypedAuthModule_UserTypeValidation(t *testing.T) {
	mod := &recTypedAppModule{name: "probe"}
	accessor := (&middleware.AuthMiddleware{}).UserClaimsAccessorFor("tech")

	tests := []struct {
		name     string
		userType string
	}{
		{name: "空串", userType: ""},
		{name: "路由元字符_冒号", userType: ":tech"},
		{name: "路由元字符_斜杠", userType: "tech/x"},
		{name: "路由元字符_星号", userType: "*"},
		{name: "大写字母开头", userType: "Tech"},
		{name: "数字开头", userType: "1tech"},
		{name: "保留字_auth", userType: "auth"},
		{name: "保留字_user", userType: "user"},
		{name: "保留字_echo", userType: "echo"},
		{name: "保留字_message", userType: "message"},
		{name: "保留字_content", userType: "content"},
		{name: "保留字_storage", userType: "storage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &App{cfg: &config.Config{}, clientRouter: &clientRouterPkg.ClientRouter{},
				authMW: &middleware.AuthMiddleware{}}
			assert.Panics(t, func() { a.RegisterTypedAuthModule(tt.userType, mod, accessor) },
				"userType %q 必须在注册期 fail-fast", tt.userType)
		})
	}

	t.Run("合规userType通过校验", func(t *testing.T) {
		for _, valid := range []string{"tech", "tech-app", "tech2", "rider-x1"} {
			a := &App{cfg: &config.Config{}, clientRouter: &clientRouterPkg.ClientRouter{},
				authMW: &middleware.AuthMiddleware{}}
			assert.NotPanics(t, func() { a.RegisterTypedAuthModule(valid, mod, accessor) },
				"userType %q 应通过校验", valid)
		}
	})
}

// assertRouteMounted 断言路径已注册到 engine。
func assertRouteMounted(t *testing.T, e *gin.Engine, path string) {
	t.Helper()
	for _, ri := range e.Routes() {
		if ri.Path == path {
			return
		}
	}
	t.Fatalf("路由未挂载: %s", path)
}
