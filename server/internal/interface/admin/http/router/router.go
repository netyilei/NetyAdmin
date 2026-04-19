package router

import (
	"github.com/gin-gonic/gin"

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
) *Router {
	return &Router{
		authVerifier: authVerifier,
		ipacSvc:      ipacSvc,
		routers: []v1.ModuleRouter{
			v1.NewAuthRouter(authH),
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
	// 注册全局中间件
	engine.Use(middleware.TraceID())
	engine.Use(middleware.IPACAuth(r.ipacSvc))

	r.registerV1(engine)
}

func (r *Router) registerV1(engine *gin.Engine) {
	adminV1 := engine.Group("/admin/v1")

	// 1. 不需要认证的接口 (如登录、获取上传凭证)
	publicGroup := adminV1.Group("")
	for _, module := range r.routers {
		module.RegisterPublic(publicGroup)
	}

	// 2. 需要认证，但不需要特定权限的接口 (如获取个人信息)
	authGroup := adminV1.Group("")
	authGroup.Use(middleware.JWTAuth())
	for _, module := range r.routers {
		module.RegisterAuth(authGroup)
	}

	// 3. 需要认证且需要特定权限的接口 (RBAC)
	permissionGroup := adminV1.Group("")
	permissionGroup.Use(middleware.JWTAuth())
	permissionGroup.Use(middleware.PermissionAuth(r.authVerifier))
	for _, module := range r.routers {
		module.RegisterPermission(permissionGroup)
	}
}
