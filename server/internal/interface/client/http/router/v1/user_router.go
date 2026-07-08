package v1

import (
	handler "NetyAdmin/internal/interface/client/http/handler/v1"
	"NetyAdmin/internal/middleware"
	authPkg "NetyAdmin/internal/pkg/auth"

	"github.com/gin-gonic/gin"
)

type userRouter struct {
	handler      *handler.UserHandler
	loginLimiter authPkg.LoginLimiter
	authMW       *middleware.AuthMiddleware
}

func NewUserRouter(h *handler.UserHandler, loginLimiter authPkg.LoginLimiter, authMW *middleware.AuthMiddleware) ClientModuleRouter {
	return &userRouter{handler: h, loginLimiter: loginLimiter, authMW: authMW}
}

func (r *userRouter) RegisterPublic(publicGroup *gin.RouterGroup) {}

func (r *userRouter) RegisterAuth(authGroup *gin.RouterGroup) {
	group := authGroup.Group("/user")
	// 登录端点限流：仅在 /login + /refresh-token 上挂载，不全局注册。
	// limiter 为 noopLoginLimiter 时（Redis 未配置）等价于透传。
	loginRL := middleware.LoginRateLimit(r.loginLimiter)
	{
		// 需要 App 签名但不需要 User JWT 的接口
		group.POST("/register", r.handler.Register)
		group.POST("/login", loginRL, r.handler.Login)
		group.POST("/refresh-token", loginRL, r.handler.RefreshToken)
		group.POST("/reset-password", r.handler.ResetPassword)

		// 需要 App 签名 + User JWT 的接口
		userAuth := group.Group("")
		userAuth.Use(r.authMW.UserJWTAuth())
		{
			userAuth.GET("/profile", r.handler.GetProfile)
			userAuth.PUT("/profile", r.handler.UpdateProfile)
			userAuth.PUT("/password", r.handler.ChangePassword)
			userAuth.DELETE("/account", r.handler.DeleteAccount)
			userAuth.GET("/upload-token", r.handler.GetUploadToken)
			userAuth.POST("/upload-record", r.handler.RecordUpload)
			userAuth.POST("/logout", r.handler.Logout)
		}
	}
}
