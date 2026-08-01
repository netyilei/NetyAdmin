package v1

import (
	handler "NetyAdmin/internal/interface/client/http/handler/v1"
	"github.com/gin-gonic/gin"
)

type storageRouter struct {
	handler *handler.ClientStorageHandler
}

func NewStorageRouter(h *handler.ClientStorageHandler) ClientModuleRouter {
	return &storageRouter{handler: h}
}

func (r *storageRouter) RegisterPublic(publicGroup *gin.RouterGroup) {
}

func (r *storageRouter) RegisterAuth(authGroup *gin.RouterGroup) {
	authGroup.POST("/storage/credentials", r.handler.GetUploadCredentials)
	authGroup.POST("/storage/records", r.handler.CreateUploadRecord)
}
