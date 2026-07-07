package v1

import (
	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/interface/admin/http/handler/v1/error_log"
	"NetyAdmin/internal/interface/admin/http/handler/v1/operation_log"
	"NetyAdmin/internal/middleware"
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
		// DELETE /:id 与 POST /batch-delete 挂 SkipOperationLog marker，
		// 避免操作日志中间件记录「删除操作日志」自身（产生噪音 + 净增长无法收敛）。
		// GET（List）天然被 OperationLogger 的 method 过滤跳过，无需 marker。
		operationLogGroup.GET("", r.operationLog.List)
		operationLogGroup.DELETE("/:id", middleware.SkipOperationLog(), r.operationLog.Delete)
		operationLogGroup.POST("/batch-delete", middleware.SkipOperationLog(), r.operationLog.DeleteBatch)
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
