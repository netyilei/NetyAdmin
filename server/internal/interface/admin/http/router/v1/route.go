package v1

import (
	"github.com/gin-gonic/gin"

	"netyadmin/internal/interface/admin/http/handler/v1/route"
)

type RouteRouter struct {
	handler *route.RouteHandler
}

func NewRouteRouter(handler *route.RouteHandler) *RouteRouter {
	return &RouteRouter{handler: handler}
}

func (r *RouteRouter) RegisterPublic(group *gin.RouterGroup) {}

func (r *RouteRouter) RegisterAuth(group *gin.RouterGroup) {
	routeGroup := group.Group("/route")
	{
		routeGroup.GET("/getUserRoutes", r.handler.GetUserRoutes)
		routeGroup.GET("/isRouteExist", r.handler.IsRouteExist)
	}
}

func (r *RouteRouter) RegisterPermission(group *gin.RouterGroup) {}
