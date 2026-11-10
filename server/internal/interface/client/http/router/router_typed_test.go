package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"NetyAdmin/internal/middleware"
	"NetyAdmin/internal/pkg/auth"
	jwtPkg "NetyAdmin/internal/pkg/jwt"
)

// recTypedModule typed 用户路由模块（测试用）：public/auth 各注册一个探针路由。
type recTypedModule struct{ name string }

func (m *recTypedModule) Name() string { return m.name }

func (m *recTypedModule) RegisterPublic(g *gin.RouterGroup) {
	g.GET("/hello", func(c *gin.Context) { c.Status(http.StatusOK) })
}

func (m *recTypedModule) RegisterAuth(g *gin.RouterGroup) {
	g.GET("/profile", func(c *gin.Context) { c.Status(http.StatusOK) })
}

// routePaths 收集 engine 已注册路由的路径集合。
func routePaths(e *gin.Engine) map[string]bool {
	paths := make(map[string]bool)
	for _, ri := range e.Routes() {
		paths[ri.Path] = true
	}
	return paths
}

// TestRegisterTypedRoutes_SplitFromRegister 锁定 typed 路由注册的时序契约：
// Register（Bootstrap 期执行）不注册 typed 路由；typed 路由由 RegisterTypedRoutes
// （Run 监听前）单独挂载。缺陷背景：拆分前 typed 注册捆绑在 Register 内
// （Bootstrap 期 typedRouters 必为空），下游晚到的 RegisterTypedAuthModule 只追加
// slice、静默无效——扩展点悬空（2026-11-11 根修）。
func TestRegisterTypedRoutes_SplitFromRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 零值 AuthMiddleware：TypedUserJWTAuth 仅构造闭包不解引用 receiver，
	// 注册期安全；authed 路由的调用期行为需完整 JWT 依赖，不在本测试范围。
	r := &ClientRouter{authMW: &middleware.AuthMiddleware{}}
	accessor := r.authMW.UserClaimsAccessorFor("tech")
	r.RegisterTypedAuthModule("tech", &recTypedModule{name: "tech"}, accessor)

	engine := gin.New()
	r.Register(engine)

	paths := routePaths(engine)
	assert.False(t, paths["/client/v1/tech/public/hello"], "Register 不得注册 typed 路由（时序契约）")
	assert.False(t, paths["/client/v1/tech/profile"], "Register 不得注册 typed 路由（时序契约）")

	r.RegisterTypedRoutes(engine)

	paths = routePaths(engine)
	assert.True(t, paths["/client/v1/tech/public/hello"], "RegisterTypedRoutes 应挂载 typed public 路由")
	assert.True(t, paths["/client/v1/tech/profile"], "RegisterTypedRoutes 应挂载 typed authed 路由")

	// public 组无鉴权中间件 → 探针路由直接可达
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/client/v1/tech/public/hello", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

// stubAccessor 独立账户体系角色的 accessor 样例（验证 RegisterTypedAuthModule
// 接受任意 ClaimsAccessor 实现——自有账户体系角色不走 UserClaimsAccessorFor）。
type stubAccessor struct{}

func (stubAccessor) NewClaims() *jwtPkg.UserClaims { return &jwtPkg.UserClaims{} }
func (stubAccessor) TokenStoreKey(c *jwtPkg.UserClaims) string {
	return c.UID
}

func (stubAccessor) SkipLegacyTokenStoreCheck() bool { return true }

func (stubAccessor) LookupAccount(context.Context, *jwtPkg.UserClaims, string) (*auth.AccountCheckResult, error) {
	return &auth.AccountCheckResult{}, nil
}

// TestRegisterTypedAuthModule_AcceptsCustomAccessor 验证下游可直传自实现 accessor
// （编译期接口满足 + 注册路径可用）。
func TestRegisterTypedAuthModule_AcceptsCustomAccessor(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := &ClientRouter{authMW: &middleware.AuthMiddleware{}}
	r.RegisterTypedAuthModule("merchant", &recTypedModule{name: "merchant"}, stubAccessor{})

	engine := gin.New()
	r.RegisterTypedRoutes(engine)

	paths := routePaths(engine)
	assert.True(t, paths["/client/v1/merchant/public/hello"])
	assert.True(t, paths["/client/v1/merchant/profile"])
}
