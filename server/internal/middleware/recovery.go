package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"netyadmin/internal/pkg/errorx"
	"netyadmin/internal/pkg/response"
	logService "netyadmin/internal/service/log"
)

func Recovery(errorLogSvc logService.ErrorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				requestID := c.GetString("requestID")
				if requestID == "" {
					requestID = uuid.New().String()
					c.Set("requestID", requestID)
					c.Header("X-Request-ID", requestID)
				}

				var userID uint
				if uid, exists := c.Get("userID"); exists {
					userID = uid.(uint)
				}

				errorLogSvc.LogPanic(
					c.Request.Context(),
					err,
					requestID,
					c.Request.URL.Path,
					c.Request.Method,
					c.ClientIP(),
					c.Request.UserAgent(),
					userID,
				)

				response.FailWithStatus(c, http.StatusInternalServerError, errorx.CodeInternalError, "服务器内部错误")
				c.Abort()
			}
		}()

		c.Next()
	}
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("requestID", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func ErrorLogger(errorLogSvc logService.ErrorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			requestID := c.GetString("requestID")

			var userID uint
			if uid, exists := c.Get("userID"); exists {
				userID = uid.(uint)
			}

			for _, err := range c.Errors {
				errorLogSvc.LogError(
					c.Request.Context(),
					err.Err,
					requestID,
					c.Request.URL.Path,
					c.Request.Method,
					c.ClientIP(),
					c.Request.UserAgent(),
					userID,
				)
			}
		}
	}
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		if status >= 400 {
			requestID := c.GetString("requestID")
			println("[%s] %s %s %d %v requestID=%s", time.Now().Format("2006-01-02 15:04:05"), method, path, status, latency, requestID)
		}
	}
}
