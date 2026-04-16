package v1

import (
	"github.com/gin-gonic/gin"
	handler "NetyAdmin/internal/interface/client/http/handler/v1"
)

type echoRouter struct {
	handler *handler.EchoHandler
}

func NewEchoRouter(h *handler.EchoHandler) ClientModuleRouter {
	return &echoRouter{handler: h}
}

func (r *echoRouter) RegisterPublic(publicGroup *gin.RouterGroup) {
	// 这里可以放不需要签名的接口
}

func (r *echoRouter) RegisterAuth(authGroup *gin.RouterGroup) {
	// 注册需要签名的 Echo 接口
	authGroup.POST("/echo", r.handler.Echo)
}
