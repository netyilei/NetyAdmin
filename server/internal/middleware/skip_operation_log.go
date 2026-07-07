package middleware

import "github.com/gin-gonic/gin"

// skipOperationLogKey 是标记「跳过操作日志」的 gin.Context key。
// 由路由级 marker SkipOperationLog 设置，OperationLogger 开头检查。
const skipOperationLogKey = "__skip_operation_log"

// SkipOperationLog 是路由级 marker 中间件，挂在需要跳过 OperationLogger 的路由上。
//
// 使用场景：operation-logs 自身的删除路由（DELETE /:id、POST /batch-delete）若被记录，
// 会产生「删日志产生新日志」的噪音（删 N 条产生 N 条记录，净增长无法收敛）。
//
// 用法：
//
//	operationLogGroup.DELETE("/:id", middleware.SkipOperationLog(), r.handler.Delete)
//
// 替代原 OperationLogger 内硬编码字符串前缀匹配的脆弱实现（路由改名会静默失效）。
func SkipOperationLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(skipOperationLogKey, true)
		c.Next()
	}
}
