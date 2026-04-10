package v1

import (
	"github.com/gin-gonic/gin"

	storageHandler "NetyAdmin/internal/interface/admin/http/handler/v1/storage"
)

type StorageRouter struct {
	handler *storageHandler.StorageHandler
}

func NewStorageRouter(handler *storageHandler.StorageHandler) *StorageRouter {
	return &StorageRouter{handler: handler}
}

func (r *StorageRouter) RegisterPublic(group *gin.RouterGroup) {}

func (r *StorageRouter) RegisterAuth(group *gin.RouterGroup) {
	storageGroup := group.Group("/storage")
	{
		storageGroup.POST("/upload-credentials", r.handler.GetUploadCredentials)
		storageGroup.POST("/upload-record", r.handler.CreateUploadRecord)
	}
}

func (r *StorageRouter) RegisterPermission(group *gin.RouterGroup) {
	storageConfigGroup := group.Group("/storage-configs")
	{
		storageConfigGroup.GET("", r.handler.GetStorageConfigList)
		storageConfigGroup.GET("/all-enabled", r.handler.GetAllEnabledStorageConfigs)
		storageConfigGroup.GET("/:id", r.handler.GetStorageConfig)
		storageConfigGroup.POST("", r.handler.CreateStorageConfig)
		storageConfigGroup.PUT("", r.handler.UpdateStorageConfig)
		storageConfigGroup.DELETE("/:id", r.handler.DeleteStorageConfig)
		storageConfigGroup.PUT("/:id/default", r.handler.SetDefaultStorageConfig)
		storageConfigGroup.POST("/test-upload", r.handler.TestStorageUpload)
	}

	uploadRecordGroup := group.Group("/upload-records")
	{
		uploadRecordGroup.GET("", r.handler.GetUploadRecordList)
		uploadRecordGroup.GET("/:id", r.handler.GetUploadRecord)
		uploadRecordGroup.DELETE("/:id", r.handler.DeleteUploadRecord)
		uploadRecordGroup.POST("/batch-delete", r.handler.DeleteUploadRecords)
	}
}
