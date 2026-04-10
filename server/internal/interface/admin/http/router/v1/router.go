package v1

import "github.com/gin-gonic/gin"

// ModuleRouter is the interface that all V1 routers must implement
type ModuleRouter interface {
	// RegisterPublic registers routes that do not require any authentication
	RegisterPublic(publicGroup *gin.RouterGroup)
	// RegisterAuth registers routes that require authentication but no specific permissions
	RegisterAuth(authGroup *gin.RouterGroup)
	// RegisterPermission registers routes that require both authentication and specific permissions
	RegisterPermission(permissionGroup *gin.RouterGroup)
}
