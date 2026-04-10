package router

import (
	"github.com/gin-gonic/gin"

	"netyadmin/internal/interface/admin/http/handler/v1/admin"
	"netyadmin/internal/interface/admin/http/handler/v1/auth"
	"netyadmin/internal/interface/admin/http/handler/v1/content"
	"netyadmin/internal/interface/admin/http/handler/v1/error_log"
	"netyadmin/internal/interface/admin/http/handler/v1/operation_log"
	"netyadmin/internal/interface/admin/http/handler/v1/route"
	storageHandler "netyadmin/internal/interface/admin/http/handler/v1/storage"
	"netyadmin/internal/interface/admin/http/handler/v1/system"
	"netyadmin/internal/middleware"
	v1 "netyadmin/internal/interface/admin/http/router/v1"
)

type Router struct {
	authVerifier middleware.AuthVerifier
	routers      []v1.ModuleRouter
}

func NewRouter(
	authH *auth.AuthHandler,
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
	authVerifier middleware.AuthVerifier,
) *Router {
	return &Router{
		authVerifier: authVerifier,
		routers: []v1.ModuleRouter{
			v1.NewAuthRouter(authH),
			v1.NewAdminRouter(adminH),
			v1.NewSystemRouter(systemH),
			v1.NewStorageRouter(storageH),
			v1.NewContentRouter(categoryH, articleH, bannerGroupH, bannerItemH),
			v1.NewLogRouter(operationLogH, errorLogH),
			v1.NewRouteRouter(routeH),
		},
	}
}

func (r *Router) Register(engine *gin.Engine) {
	// 注册全局中间件
	engine.Use(middleware.TraceID())

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
