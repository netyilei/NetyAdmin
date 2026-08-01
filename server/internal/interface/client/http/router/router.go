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
	routers      []v1.ClientModuleRouter
	typedRouters []typedModuleRouter
}

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

// RegisterTypedAuthModule registers a ClientModuleRouter under a dedicated
// userType namespace. At registration time, two route groups are created:
//
//   - /client/v1/{userType}/public  — no auth, for RegisterPublic (OAuth callbacks,
//     login endpoints, etc.)
//   - /client/v1/{userType}         — TypedUserJWTAuth(accessor) applied, for RegisterAuth
//
// This preserves the public/auth split of ClientModuleRouter: public endpoints
// remain accessible without authentication, while authed endpoints are isolated
// by userType-specific JWT validation.
//
// This enables multi-role terminal routing: downstream projects register
// role-specific modules (e.g. "tech", "merchant") without modifying the base
// framework's ClientRouter.
func (r *ClientRouter) RegisterTypedAuthModule(userType string, module v1.ClientModuleRouter, accessor authPkg.ClaimsAccessor[*jwtPkg.UserClaims]) {
	r.typedRouters = append(r.typedRouters, typedModuleRouter{
		userType: userType,
		module:   module,
		accessor: accessor,
	})
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
	for _, module := range r.routers {
		module.RegisterAuth(authGroup)
	}

	// 3. Typed JWT auth route groups — one per userType
	// Each userType gets a public sub-group (no auth) and an authed group
	// with TypedUserJWTAuth(accessor) applied.
	for _, tm := range r.typedRouters {
		// Public endpoints under /client/v1/{userType}/public — no JWT required
		typedPublic := clientV1.Group(tm.userType + "/public")
		tm.module.RegisterPublic(typedPublic)

		// Authed endpoints under /client/v1/{userType} — JWT with matching type required
		typedAuthed := clientV1.Group(tm.userType)
		typedAuthed.Use(r.authMW.TypedUserJWTAuth(tm.accessor))
		tm.module.RegisterAuth(typedAuthed)
	}
}
