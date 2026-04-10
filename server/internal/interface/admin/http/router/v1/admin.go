package v1

import (
	"github.com/gin-gonic/gin"

	"silentorder/internal/interface/admin/http/handler/v1/admin"
)

type AdminRouter struct {
	handler *admin.AdminHandler
}

func NewAdminRouter(handler *admin.AdminHandler) *AdminRouter {
	return &AdminRouter{handler: handler}
}

func (r *AdminRouter) RegisterPublic(group *gin.RouterGroup) {}

func (r *AdminRouter) RegisterAuth(group *gin.RouterGroup) {}

func (r *AdminRouter) RegisterPermission(group *gin.RouterGroup) {
	adminGroup := group.Group("/admins")
	{
		adminGroup.GET("", r.handler.List)
		adminGroup.POST("", r.handler.Create)
		adminGroup.GET("/:id", r.handler.GetByID)
		adminGroup.PUT("/:id", r.handler.Update)
		adminGroup.DELETE("/:id", r.handler.Delete)
	}
}
