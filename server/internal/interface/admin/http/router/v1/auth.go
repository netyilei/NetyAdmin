package v1

import (
	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/interface/admin/http/handler/v1/auth"
	"NetyAdmin/internal/middleware"
	authPkg "NetyAdmin/internal/pkg/auth"
)

type AuthRouter struct {
	handler      *auth.AuthHandler
	loginLimiter authPkg.LoginLimiter
}

func NewAuthRouter(handler *auth.AuthHandler, loginLimiter authPkg.LoginLimiter) *AuthRouter {
	return &AuthRouter{handler: handler, loginLimiter: loginLimiter}
}

func (r *AuthRouter) RegisterPublic(group *gin.RouterGroup) {
	authGroup := group.Group("/auth")
	// 登录端点限流：仅在 /login + /refreshToken 上挂载，不全局注册。
	// limiter 为 noopLoginLimiter 时（Redis 未配置）等价于透传。
	loginRL := middleware.LoginRateLimit(r.loginLimiter)
	{
		authGroup.POST("/login", loginRL, r.handler.Login)
		authGroup.POST("/refreshToken", loginRL, r.handler.RefreshToken)
	}
}

func (r *AuthRouter) RegisterAuth(group *gin.RouterGroup) {
	authGroup := group.Group("/auth")
	{
		authGroup.GET("/getUserInfo", r.handler.GetUserInfo)
		authGroup.GET("/profile", r.handler.GetProfile)
		authGroup.PUT("/profile", r.handler.UpdateProfile)
		authGroup.POST("/changePassword", r.handler.ChangePassword)
		authGroup.POST("/logout", r.handler.Logout)
	}
}

func (r *AuthRouter) RegisterPermission(group *gin.RouterGroup) {}
