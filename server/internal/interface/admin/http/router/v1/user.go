package v1

import (
	"github.com/gin-gonic/gin"

	userHandler "NetyAdmin/internal/interface/admin/http/handler/v1/user"
)

type UserRouter struct {
	user *userHandler.UserHandler
}

func NewUserRouter(user *userHandler.UserHandler) *UserRouter {
	return &UserRouter{user: user}
}

func (r *UserRouter) RegisterPublic(group *gin.RouterGroup) {}

func (r *UserRouter) RegisterAuth(group *gin.RouterGroup) {}

func (r *UserRouter) RegisterPermission(group *gin.RouterGroup) {
	userGroup := group.Group("/systemManage/users")
	{
		userGroup.GET("/autocomplete", r.user.Autocomplete)
		userGroup.GET("", r.user.List)
		userGroup.POST("", r.user.Create)
		userGroup.PUT("/:id", r.user.Update)
		userGroup.PATCH("/:id/status", r.user.UpdateStatus)
		userGroup.POST("/:id/unlock", r.user.Unlock)
		userGroup.DELETE("/:id", r.user.Delete)
	}
}
