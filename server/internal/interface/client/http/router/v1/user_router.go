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

func (r *userRouter) RegisterPublic(publicGroup *gin.RouterGroup) {}

func (r *userRouter) RegisterAuth(authGroup *gin.RouterGroup) {
	group := authGroup.Group("/user")
	{
		// 需要 App 签名但不需要 User JWT 的接口
		group.POST("/register", r.handler.Register)
		group.POST("/login", r.handler.Login)
		group.POST("/refresh-token", r.handler.RefreshToken)
		group.POST("/reset-password", r.handler.ResetPassword)

		// 需要 App 签名 + User JWT 的接口
		userAuth := group.Group("")
		userAuth.Use(middleware.UserJWTAuth())
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
