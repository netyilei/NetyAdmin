package v1

import (
	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/interface/admin/http/handler/v1/auth"
)

type AuthRouter struct {
	handler *auth.AuthHandler
}

func NewAuthRouter(handler *auth.AuthHandler) *AuthRouter {
	return &AuthRouter{handler: handler}
}

func (r *AuthRouter) RegisterPublic(group *gin.RouterGroup) {
	authGroup := group.Group("/auth")
	{
		authGroup.POST("/login", r.handler.Login)
		authGroup.POST("/refreshToken", r.handler.RefreshToken)
	}
}

func (r *AuthRouter) RegisterAuth(group *gin.RouterGroup) {
	authGroup := group.Group("/auth")
	{
		authGroup.GET("/getUserInfo", r.handler.GetUserInfo)
		authGroup.GET("/profile", r.handler.GetProfile)
		authGroup.PUT("/profile", r.handler.UpdateProfile)
		authGroup.POST("/changePassword", r.handler.ChangePassword)
	}
}

func (r *AuthRouter) RegisterPermission(group *gin.RouterGroup) {}
