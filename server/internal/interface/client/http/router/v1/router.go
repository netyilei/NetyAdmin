package v1

import "github.com/gin-gonic/gin"

// ClientModuleRouter 客户端模块路由接口
type ClientModuleRouter interface {
	// RegisterPublic 注册无需签名的接口
	RegisterPublic(publicGroup *gin.RouterGroup)
	// RegisterAuth 注册需要签名的接口
	RegisterAuth(authGroup *gin.RouterGroup)
}
