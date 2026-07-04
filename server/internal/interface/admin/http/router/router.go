package router

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"NetyAdmin/internal/interface/admin/http/handler/v1/admin"
	"NetyAdmin/internal/interface/admin/http/handler/v1/auth"
	"NetyAdmin/internal/interface/admin/http/handler/v1/common"
	"NetyAdmin/internal/interface/admin/http/handler/v1/content"
	dictHandler "NetyAdmin/internal/interface/admin/http/handler/v1/dict"
	"NetyAdmin/internal/interface/admin/http/handler/v1/error_log"
	ipacHandler "NetyAdmin/internal/interface/admin/http/handler/v1/ipac"
	msgHandler "NetyAdmin/internal/interface/admin/http/handler/v1/message"
	openHandler "NetyAdmin/internal/interface/admin/http/handler/v1/open_platform"
	"NetyAdmin/internal/interface/admin/http/handler/v1/operation_log"
	"NetyAdmin/internal/interface/admin/http/handler/v1/route"
	storageHandler "NetyAdmin/internal/interface/admin/http/handler/v1/storage"
	"NetyAdmin/internal/interface/admin/http/handler/v1/system"
	taskHandler "NetyAdmin/internal/interface/admin/http/handler/v1/task"
	userAdminHandler "NetyAdmin/internal/interface/admin/http/handler/v1/user"
	v1 "NetyAdmin/internal/interface/admin/http/router/v1"
	"NetyAdmin/internal/middleware"
	authPkg "NetyAdmin/internal/pkg/auth"
	ipacService "NetyAdmin/internal/service/ipac"
)

type Router struct {
	authVerifier middleware.AuthVerifier
	ipacSvc      ipacService.IPACService
	routers      []v1.ModuleRouter
}

func NewRouter(
	authH *auth.AuthHandler,
	commonH *common.CommonHandler,
	adminH *admin.AdminHandler,
	systemH *system.SystemHandler,
	storageH *storageHandler.StorageHandler,
	categoryH *content.ContentCategoryHandler,
	articleH *content.ContentArticleHandler,
	bannerGroupH *content.ContentBannerGroupHandler,
	bannerItemH *content.ContentBannerItemHandler,
	operationLogH *operation_log.OperationLogHandler,
	errorLogH *error_log.ErrorLogHandler,
	routeH *route.RouteHandler,
	ipacH *ipacHandler.IPACHandler,
	appH *openHandler.AppHandler,
	openApiH *openHandler.OpenApiHandler,
	openLogH *openHandler.OpenLogHandler,
	msgH *msgHandler.MessageHandler,
	dictH *dictHandler.DictHandler,
	taskH *taskHandler.TaskHandler,
	userAdminH *userAdminHandler.UserHandler,
	ipacSvc ipacService.IPACService,
	authVerifier middleware.AuthVerifier,
	loginLimiter authPkg.LoginLimiter,
) *Router {
	return &Router{
		authVerifier: authVerifier,
		ipacSvc:      ipacSvc,
		routers: []v1.ModuleRouter{
			v1.NewAuthRouter(authH, loginLimiter),
			v1.NewCommonRouter(commonH),
			v1.NewAdminRouter(adminH),
			v1.NewStorageRouter(storageH),
			v1.NewContentRouter(categoryH, articleH, bannerGroupH, bannerItemH),
			v1.NewLogRouter(operationLogH, errorLogH),
			v1.NewRouteRouter(routeH),
			v1.NewOpsRouter(ipacH, appH, openApiH, openLogH),
			v1.NewMessageRouter(msgH),
			v1.NewSystemRouter(systemH),
			v1.NewDictRouter(dictH),
			v1.NewTaskRouter(taskH),
			v1.NewUserRouter(userAdminH),
		},
	}
}

func (r *Router) Register(engine *gin.Engine) {
	// 全局中间件（RequestID / CORS / SecurityHeaders / Recovery / Sentry / ErrorLogger /
	// Timeout / Logger / OperationLogger）已在 wire.go 中通过 engine.Use 注册。
	//
	// IPACAuth 不再全局注册：原 engine.Use(middleware.IPACAuth(...)) 会应用到所有路由，
	// 包括 /admin/v1/auth/login、/auth/refreshToken、/common/captcha 等公开路由。
	// 当 ipacSvc.CheckIP 出错（fail-closed）时返回 CodeIPBlocked，会导致所有人都无法登录修复问题
	// ——系统自锁风险违反"基座程序可用性"原则。
	// 现改为在 authGroup / permissionGroup 路由组上注册，公开路由豁免 IPAC 检查。
	// /health 端点在 wire.go:305 已先于 router.Register 注册，保持豁免。

	r.registerV1(engine)

	// Swagger UI - 仅 debug 模式下开放
	if gin.Mode() == gin.DebugMode {
		engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
}

func (r *Router) registerV1(engine *gin.Engine) {
	adminV1 := engine.Group("/admin/v1")

	// 1. 不需要认证的接口 (如登录、获取上传凭证)
	//    公开路由豁免 IPAC 检查，确保 IPAC 服务故障或误配 deny 规则时仍可登录修复问题。
	publicGroup := adminV1.Group("")
	for _, module := range r.routers {
		module.RegisterPublic(publicGroup)
	}

	// 2. 需要认证，但不需要特定权限的接口 (如获取个人信息)
	//    IPACAuth 在 JWTAuth 之前：IP 被拒绝的请求不浪费 JWT 验证时间。
	authGroup := adminV1.Group("")
	authGroup.Use(middleware.IPACAuth(r.ipacSvc))
	authGroup.Use(middleware.JWTAuth())
	for _, module := range r.routers {
		module.RegisterAuth(authGroup)
	}

	// 3. 需要认证且需要特定权限的接口 (RBAC)
	//    IPACAuth 在 JWTAuth 之前，与 authGroup 保持一致。
	permissionGroup := adminV1.Group("")
	permissionGroup.Use(middleware.IPACAuth(r.ipacSvc))
	permissionGroup.Use(middleware.JWTAuth())
	permissionGroup.Use(middleware.PermissionAuth(r.authVerifier))
	for _, module := range r.routers {
		module.RegisterPermission(permissionGroup)
	}
}
