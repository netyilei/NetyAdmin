package router

import (
	"github.com/gin-gonic/gin"

	handler "NetyAdmin/internal/interface/client/http/handler/v1"
	v1 "NetyAdmin/internal/interface/client/http/router/v1"
	"NetyAdmin/internal/middleware"
	ipacSvcPkg "NetyAdmin/internal/service/ipac"
	openSvcPkg "NetyAdmin/internal/service/open_platform"
)

type ClientRouter struct {
	appSvc  openSvcPkg.AppService
	apiSvc  openSvcPkg.OpenApiService
	logSvc  openSvcPkg.OpenLogService
	ipacSvc ipacSvcPkg.IPACService
	routers []v1.ClientModuleRouter
}

func NewClientRouter(
	echoH *handler.EchoHandler,
	userH *handler.UserHandler,
	authH *handler.AuthHandler,
	messageH *handler.MessageHandler,
	contentH *handler.ContentHandler,
	appSvc openSvcPkg.AppService,
	apiSvc openSvcPkg.OpenApiService,
	logSvc openSvcPkg.OpenLogService,
	ipacSvc ipacSvcPkg.IPACService,
) *ClientRouter {
	return &ClientRouter{
		appSvc:  appSvc,
		apiSvc:  apiSvc,
		logSvc:  logSvc,
		ipacSvc: ipacSvc,
		routers: []v1.ClientModuleRouter{
			v1.NewEchoRouter(echoH),
			v1.NewUserRouter(userH),
			v1.NewAuthRouter(authH),
			v1.NewMessageRouter(messageH),
			v1.NewContentRouter(contentH),
		},
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
	for _, module := range r.routers {
		module.RegisterAuth(authGroup)
	}
}
