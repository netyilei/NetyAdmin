package app

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/middleware"
	jwtPkg "NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/task"
)

// Module 是下游业务项目装配到基座的扩展点基接口。
//
// 下游实现一个或多个子接口（ClientRouterModule / AdminRouterModule / JobModule /
// EngineModule），通过 App.RegisterModule 注入——零基座代码改动。
// 基接口仅要求 Name()，用于日志/诊断标识模块。
type Module interface {
	// Name 返回模块名（日志/诊断用，如 "fitness"）。
	Name() string
}

// ClientRouterModule 扩展客户端路由树（/client/v1/*）。
//
// 基座在 Register 时已对 authGroup 挂载 OpenPlatformAuth（应用签名校验）——
// 下游在 RegisterClientAuth 内注册业务路由即可；如需 JWT 用户鉴权，
// 用 deps.AuthMiddleware 在子组挂载 UserJWTAuth / TypedUserJWTAuth。
type ClientRouterModule interface {
	Module
	// RegisterClientAuth 在已挂 OpenPlatformAuth 的 authGroup 内注册需签名的业务路由。
	RegisterClientAuth(authGroup *gin.RouterGroup, deps RouterDeps)
	// RegisterClientPublic 在无签名的 publicGroup 注册公开路由。
	// 无公开路由的模块空实现即可。
	RegisterClientPublic(publicGroup *gin.RouterGroup, deps RouterDeps)
}

// AdminRouterModule 扩展管理端路由树（/admin/v1/*）。
//
// 基座在 Register 时已对 permissionGroup 挂载完整权限链
// （IPACAuth → JWTAuth → PermissionAuth）——下游在
// RegisterAdminPermission 内注册业务路由即可。
type AdminRouterModule interface {
	Module
	// RegisterAdminPermission 在已挂完整权限链的 permissionGroup 内注册业务路由。
	RegisterAdminPermission(permissionGroup *gin.RouterGroup, deps RouterDeps)
}

// JobModule 提供定时任务注册。
type JobModule interface {
	Module
	// RegisterJobs 返回任务列表，基座调 taskManager.Register 注册。
	RegisterJobs() []task.Task
}

// EngineModule 提供直接访问 engine 的能力（内部回调端点等无中间件链场景）。
//
// 注册到 engine 的路由不带任何基座中间件（无 IPAC/JWT/签名校验），
// 下游需自行处理鉴权。仅用于内部端点（如 /internal/callback/task）。
type EngineModule interface {
	Module
	// RegisterEngine 在 engine 上直接注册路由。
	RegisterEngine(engine *gin.Engine, deps RouterDeps)
}

// RouterDeps 聚合下游装配所需的全部基座依赖。
//
// 设计原则：用一个结构体传递依赖，避免 App 上 getter 蔓延。
// 下游通过 RegisterXxx(group, deps) 拿到全部所需组件。
type RouterDeps struct {
	// AuthMiddleware 提供 UserJWTAuth / JWTAuth / TypedUserJWTAuth 等鉴权中间件。
	// 下游在子路由组上按需挂载（如 client 业务路由需 JWT 用户鉴权）。
	AuthMiddleware *middleware.AuthMiddleware
	// OpenPlatformAuth 是预构造的应用签名校验中间件（gin.HandlerFunc）。
	// 通常下游无需自行挂载（authGroup 已挂），仅用于 EngineModule 等需独立挂载的场景。
	OpenPlatformAuth gin.HandlerFunc
	// JWT 是 RS256 JWT 实例，供下游签发/解析自定义 token。
	JWT *jwtPkg.JWT
	// Config 是基座全局配置，供下游读取运行参数。
	Config *config.Config
}

// RegisterModule 将下游模块注入基座。
//
// 在 App.Run() 之前调用（路由注册必须在 engine 启动前）。
// m 实现哪些子接口（ClientRouterModule / AdminRouterModule / JobModule / EngineModule），
// 基座就调用哪些注册方法——下游按需实现，不强制全实现。
//
// 调用示例（下游 main.go）：
//
//	application, err := app.Bootstrap(cfg, db)
//	application.RegisterModule(fitnessModule)
//	application.Run()
//
// RegisterModule 幂等：重复注册同一模块的相同路由会 panic（gin 路由冲突），
// 下游应确保只调一次。
func (a *App) RegisterModule(m Module) {
	if m == nil {
		return
	}
	slog.Info("注册下游模块", "module", m.Name())

	// 1. 客户端路由模块
	if cm, ok := m.(ClientRouterModule); ok {
		clientV1 := a.engine.Group("/client/v1")
		publicGroup := clientV1.Group("")
		authGroup := clientV1.Group("")
		authGroup.Use(a.openPlatformAuth)
		cm.RegisterClientPublic(publicGroup, a.routerDeps)
		cm.RegisterClientAuth(authGroup, a.routerDeps)
	}

	// 2. 管理端路由模块
	if am, ok := m.(AdminRouterModule); ok {
		adminV1 := a.engine.Group("/admin/v1")
		permissionGroup := adminV1.Group("")
		permissionGroup.Use(middleware.IPACAuth(a.ipacSvc))
		permissionGroup.Use(a.authMW.JWTAuth())
		permissionGroup.Use(middleware.PermissionAuth(a.authVerifier))
		am.RegisterAdminPermission(permissionGroup, a.routerDeps)
	}

	// 3. 定时任务模块
	if jm, ok := m.(JobModule); ok {
		jobs := jm.RegisterJobs()
		if len(jobs) > 0 {
			a.taskManager.Register(jobs...)
		}
	}

	// 4. Engine 直挂模块
	if em, ok := m.(EngineModule); ok {
		em.RegisterEngine(a.engine, a.routerDeps)
	}
}
