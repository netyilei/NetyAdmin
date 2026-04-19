package v1

import (
	"github.com/gin-gonic/gin"

	ipacHandler "NetyAdmin/internal/interface/admin/http/handler/v1/ipac"
	openHandler "NetyAdmin/internal/interface/admin/http/handler/v1/open_platform"
)

type OpsRouter struct {
	ipac    *ipacHandler.IPACHandler
	app     *openHandler.AppHandler
	openApi *openHandler.OpenApiHandler
	openLog *openHandler.OpenLogHandler
}

func NewOpsRouter(ipac *ipacHandler.IPACHandler, app *openHandler.AppHandler, openApi *openHandler.OpenApiHandler, openLog *openHandler.OpenLogHandler) *OpsRouter {
	return &OpsRouter{
		ipac:    ipac,
		app:     app,
		openApi: openApi,
		openLog: openLog,
	}
}

func (r *OpsRouter) RegisterPublic(group *gin.RouterGroup) {}

func (r *OpsRouter) RegisterAuth(group *gin.RouterGroup) {}

func (r *OpsRouter) RegisterPermission(group *gin.RouterGroup) {
	r.registerIPAC(group)
	r.registerOpenPlatform(group)
	r.registerOpenApi(group)
	r.registerOpenLog(group)
}

func (r *OpsRouter) registerIPAC(group *gin.RouterGroup) {
	ipacGroup := group.Group("/ops/ip-access")
	{
		ipacGroup.GET("", r.ipac.List)
		ipacGroup.POST("", r.ipac.Create)
		ipacGroup.PUT("", r.ipac.Update)
		ipacGroup.DELETE("/:id", r.ipac.Delete)
		ipacGroup.DELETE("/batch", r.ipac.DeleteBatch)
	}
}

func (r *OpsRouter) registerOpenPlatform(group *gin.RouterGroup) {
	appGroup := group.Group("/open/apps")
	{
		appGroup.GET("", r.app.List)
		appGroup.POST("", r.app.Create)
		appGroup.PUT("", r.app.Update)
		appGroup.DELETE("/:id", r.app.Delete)
		appGroup.PUT("/reset-secret", r.app.ResetSecret)
		appGroup.PUT("/ip-rules", r.app.LinkIPRules)
		appGroup.GET("/scopes", r.app.GetScopes)
		appGroup.GET("/available-scopes", r.app.ListAvailableScopes)
	}

	scopeGroup := group.Group("/open/scopes")
	{
		scopeGroup.GET("", r.app.ListScopeGroups)
		scopeGroup.POST("", r.app.CreateScopeGroup)
		scopeGroup.PUT("", r.app.UpdateScopeGroup)
		scopeGroup.DELETE("/:id", r.app.DeleteScopeGroup)
	}
}

func (r *OpsRouter) registerOpenApi(group *gin.RouterGroup) {
	apiGroup := group.Group("/open/apis")
	{
		apiGroup.GET("", r.openApi.List)
		apiGroup.POST("", r.openApi.Create)
		apiGroup.PUT("", r.openApi.Update)
		apiGroup.DELETE("/:id", r.openApi.Delete)
		apiGroup.GET("/grouped", r.openApi.ListGrouped)
		apiGroup.GET("/scope-apis", r.openApi.GetScopeApis)
		apiGroup.PUT("/scope-apis", r.openApi.UpdateScopeApis)
	}
}

func (r *OpsRouter) registerOpenLog(group *gin.RouterGroup) {
	logGroup := group.Group("/ops/open-platform-log")
	{
		logGroup.GET("", r.openLog.List)
	}
}
