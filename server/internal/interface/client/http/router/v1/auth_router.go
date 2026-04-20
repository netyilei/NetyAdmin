package v1

import (
	handler "NetyAdmin/internal/interface/client/http/handler/v1"

	"github.com/gin-gonic/gin"
)

type authRouter struct {
	handler *handler.AuthHandler
}

func NewAuthRouter(h *handler.AuthHandler) ClientModuleRouter {
	return &authRouter{handler: h}
}

func (r *authRouter) RegisterPublic(publicGroup *gin.RouterGroup) {}

func (r *authRouter) RegisterAuth(authGroup *gin.RouterGroup) {
	group := authGroup.Group("/auth")
	{
		group.GET("/captcha", r.handler.Captcha)
		group.GET("/captcha-status", r.handler.CaptchaStatus)
		group.GET("/verify-config", r.handler.VerifyConfig)
		group.POST("/send-code", r.handler.SendCode)
	}
}
