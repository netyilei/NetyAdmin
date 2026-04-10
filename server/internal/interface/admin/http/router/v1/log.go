package v1

import (
	"github.com/gin-gonic/gin"

	"silentorder/internal/interface/admin/http/handler/v1/error_log"
	"silentorder/internal/interface/admin/http/handler/v1/operation_log"
)

type LogRouter struct {
	operationLog *operation_log.OperationLogHandler
	errorLog     *error_log.ErrorLogHandler
}

func NewLogRouter(
	operationLog *operation_log.OperationLogHandler,
	errorLog *error_log.ErrorLogHandler,
) *LogRouter {
	return &LogRouter{
		operationLog: operationLog,
		errorLog:     errorLog,
	}
}

func (r *LogRouter) RegisterPublic(group *gin.RouterGroup) {}

func (r *LogRouter) RegisterAuth(group *gin.RouterGroup) {}

func (r *LogRouter) RegisterPermission(group *gin.RouterGroup) {
	r.registerOperationLog(group)
	r.registerErrorLog(group)
}

func (r *LogRouter) registerOperationLog(group *gin.RouterGroup) {
	operationLogGroup := group.Group("/operation-logs")
	{
		operationLogGroup.GET("", r.operationLog.List)
		operationLogGroup.DELETE("/:id", r.operationLog.Delete)
		operationLogGroup.POST("/batch-delete", r.operationLog.DeleteBatch)
	}
}

func (r *LogRouter) registerErrorLog(group *gin.RouterGroup) {
	errorLogGroup := group.Group("/error-logs")
	{
		errorLogGroup.GET("", r.errorLog.List)
		errorLogGroup.PUT("/:id/resolve", r.errorLog.Resolve)
		errorLogGroup.DELETE("/:id", r.errorLog.Delete)
		errorLogGroup.POST("/batch-delete", r.errorLog.DeleteBatch)
	}
}
