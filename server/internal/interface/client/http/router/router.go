package router

import (
	"github.com/gin-gonic/gin"

	handler "NetyAdmin/internal/interface/client/http/handler/v1"
	v1 "NetyAdmin/internal/interface/client/http/router/v1"
	"NetyAdmin/internal/middleware"
	authPkg "NetyAdmin/internal/pkg/auth"
	jwtPkg "NetyAdmin/internal/pkg/jwt"
	ipacSvcPkg "NetyAdmin/internal/service/ipac"
	openSvcPkg "NetyAdmin/internal/service/open_platform"
)

type ClientRouter struct {
	appSvc       openSvcPkg.AppService
	apiSvc       openSvcPkg.OpenApiService
	logSvc       openSvcPkg.OpenLogService
	ipacSvc      ipacSvcPkg.IPACService
	authMW       *middleware.AuthMiddleware
	apiRateLimit gin.HandlerFunc // 通用业务限流（可选，nil = 未启用）
	routers      []v1.ClientModuleRouter
	typedRouters []typedModuleRouter
}

// SetAPIRateLimit 注入通用限流中间件（可选能力，wire 在配置了 rules 且 Redis 可用时注入）。
func (r *ClientRouter) SetAPIRateLimit(mw gin.HandlerFunc) { r.apiRateLimit = mw }

// typedModuleRouter associates a ClientModuleRouter with a specific userType
// and its corresponding ClaimsAccessor for JWT authentication.
type typedModuleRouter struct {
	userType string
	module   v1.ClientModuleRouter
	accessor authPkg.ClaimsAccessor[*jwtPkg.UserClaims]
}

func NewClientRouter(
	echoH *handler.EchoHandler,
	userH *handler.UserHandler,
	authH *handler.AuthHandler,
	messageH *handler.MessageHandler,
	contentH *handler.ContentHandler,
	storageH *handler.ClientStorageHandler,
	appSvc openSvcPkg.AppService,
	apiSvc openSvcPkg.OpenApiService,
	logSvc openSvcPkg.OpenLogService,
	ipacSvc ipacSvcPkg.IPACService,
	loginLimiter authPkg.LoginLimiter,
	authMW *middleware.AuthMiddleware,
) *ClientRouter {
	return &ClientRouter{
		appSvc:  appSvc,
		apiSvc:  apiSvc,
		logSvc:  logSvc,
		ipacSvc: ipacSvc,
		authMW:  authMW,
		routers: []v1.ClientModuleRouter{
			v1.NewEchoRouter(echoH),
			v1.NewUserRouter(userH, loginLimiter, authMW),
			v1.NewAuthRouter(authH),
			v1.NewMessageRouter(messageH, authMW),
			v1.NewContentRouter(contentH),
			v1.NewStorageRouter(storageH),
		},
	}
}

// RegisterTypedAuthModule 登记 typed 用户路由模块（注册后两个路由组在
// RegisterTypedRoutes 挂载时创建）：
//
//   - /client/v1/{userType}/public  — no auth, for RegisterPublic (OAuth callbacks,
//     login endpoints, etc.)
//   - /client/v1/{userType}         — TypedUserJWTAuth(accessor) applied, for RegisterAuth
//
// 内部方法——下游唯一入口是 App.RegisterTypedAuthModule（带时机/配置守卫；
// 基座 ClientRouter 实例不暴露给下游）。
func (r *ClientRouter) RegisterTypedAuthModule(userType string, module v1.ClientModuleRouter, accessor authPkg.ClaimsAccessor[*jwtPkg.UserClaims]) {
	r.typedRouters = append(r.typedRouters, typedModuleRouter{
		userType: userType,
		module:   module,
		accessor: accessor,
	})
}

// RegisterTypedRoutes 注册 typed 用户路由组（/client/v1/{userType} 与 /public 子组）。
//
// 与 Register 分离的原因（时序契约）：typed 模块只能由下游在 Bootstrap 之后提供，
// 注册时机必然晚于 Bootstrap 期执行的 Register——捆绑会让晚到的注册静默无效。
// 调用时机：App.Run() 启动监听前、全部 RegisterTypedAuthModule 调用之后；
// [base].client_router_enabled 门控在 app.Run 处（该 flag=false 时下游完全接管
// /client/v1/*，且 App.RegisterTypedAuthModule 已 fail-fast，typedRouters 必为空）。
//
// 中间件链：public 组无鉴权；authed 组 TypedUserJWTAuth(accessor) → api_ratelimit
// （可选），与基座 Register 的 authGroup 链保持一致。
func (r *ClientRouter) RegisterTypedRoutes(engine *gin.Engine) {
	clientV1 := engine.Group("/client/v1")
	for _, tm := range r.typedRouters {
		// Public endpoints under /client/v1/{userType}/public — no JWT required
		typedPublic := clientV1.Group(tm.userType + "/public")
		tm.module.RegisterPublic(typedPublic)

		// Authed endpoints under /client/v1/{userType} — JWT with matching type required
		typedAuthed := clientV1.Group(tm.userType)
		typedAuthed.Use(r.authMW.TypedUserJWTAuth(tm.accessor))
		if r.apiRateLimit != nil {
			typedAuthed.Use(r.apiRateLimit)
		}
		tm.module.RegisterAuth(typedAuthed)
	}
}

func (r *ClientRouter) Register(engine *gin.Engine) {
	clientV1 := engine.Group("/client/v1")

	// 1. 无需签名的接口
	publicGroup := clientV1.Group("")
	for _, module := range r.routers {
		module.RegisterPublic(publicGroup)
	}

	// 2. 需要开放平台签名验证的接口
	authGroup := clientV1.Group("")
	authGroup.Use(middleware.OpenPlatformAuth(r.appSvc, r.apiSvc, r.logSvc, r.ipacSvc))
	if r.apiRateLimit != nil {
		authGroup.Use(r.apiRateLimit)
	}
	for _, module := range r.routers {
		module.RegisterAuth(authGroup)
	}

	// 3. Typed 用户路由组已拆分到 RegisterTypedRoutes（不在本方法注册）：
	//    typed 模块只能由下游在拿到 App 之后提供（Bootstrap 期不存在），若捆绑在
	//    本方法（Bootstrap 内执行），晚到的注册只会追加 slice、静默无效。
	//    改由 App.Run() 在监听启动前统一挂载——时序契约见 RegisterTypedRoutes 注释。
}
