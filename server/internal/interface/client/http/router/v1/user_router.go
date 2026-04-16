package v1

import (
	handler "NetyAdmin/internal/interface/client/http/handler/v1"
	"NetyAdmin/internal/middleware"

	"github.com/gin-gonic/gin"
)

type userRouter struct {
	handler *handler.UserHandler
}

func NewUserRouter(h *handler.UserHandler) ClientModuleRouter {
	return &userRouter{handler: h}
}

func (r *userRouter) RegisterPublic(publicGroup *gin.RouterGroup) {
	group := publicGroup.Group("/user")
	{
		group.POST("/register", r.handler.Register)
		group.POST("/login", r.handler.Login)
		group.POST("/refresh-token", r.handler.RefreshToken)
		group.POST("/reset-password", r.handler.ResetPassword)
	}
}

func (r *userRouter) RegisterAuth(authGroup *gin.RouterGroup) {
	group := authGroup.Group("/user")
	group.Use(middleware.UserJWTAuth())
	{
		group.GET("/profile", r.handler.GetProfile)
		group.PUT("/profile", r.handler.UpdateProfile)
		group.POST("/change-password", r.handler.ChangePassword)
		group.POST("/logout", r.handler.Logout)
	}
}
