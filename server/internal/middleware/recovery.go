package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	logService "NetyAdmin/internal/service/log"
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

				var userID interface{}
				if uid, exists := c.Get("adminID"); exists {
					userID = uid
				} else if uid, exists := c.Get("userID"); exists {
					userID = uid
				}

				var adminIDUint uint
				switch v := userID.(type) {
				case uint:
					adminIDUint = v
				}

				errorLogSvc.LogPanic(
					c.Request.Context(),
					err,
					requestID,
					c.Request.URL.Path,
					c.Request.Method,
					c.ClientIP(),
					c.Request.UserAgent(),
					adminIDUint,
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

// SentryTagSetter 在 sentrygin 之后运行，将 requestID、path、method 注入 Sentry Scope。
// 注意：此中间件作为全局中间件运行，早于 JWTAuth（authGroup/permissionGroup 内），
// 因此 adminID/userID 尚未设置。用户 ID 的注入在 ErrorLogger 中完成（c.Next() 之后 adminID 已可用）。
func SentryTagSetter() gin.HandlerFunc {
	return func(c *gin.Context) {
		if hub := sentrygin.GetHubFromContext(c); hub != nil {
			requestID := c.GetString("requestID")
			if requestID != "" {
				hub.Scope().SetTag("request_id", requestID)
			}
			hub.Scope().SetTag("path", c.Request.URL.Path)
			hub.Scope().SetTag("method", c.Request.Method)
		}
		c.Next()
	}
}

// ErrorLogger 捕获 Gin 上下文错误，写入 DB error_log 表并上报 Sentry。
// c.Next() 之后 JWTAuth 已执行，adminID 已可用，此处注入用户上下文到 Sentry Scope。
func ErrorLogger(errorLogSvc logService.ErrorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			requestID := c.GetString("requestID")

			var userID interface{}
			if uid, exists := c.Get("adminID"); exists {
				userID = uid
			} else if uid, exists := c.Get("userID"); exists {
				userID = uid
			}

			var adminIDUint uint
			switch v := userID.(type) {
			case uint:
				adminIDUint = v
			}

			hub := sentrygin.GetHubFromContext(c)

			// 注入用户上下文到 Sentry Scope（此处 adminID 已由 JWTAuth 设置）
			if hub != nil && adminIDUint > 0 {
				hub.Scope().SetUser(sentry.User{
					ID: strconv.FormatUint(uint64(adminIDUint), 10),
				})
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
					adminIDUint,
				)

				// 同步上报到 Sentry（若 hub 存在则带请求上下文，否则用全局 hub）
				if hub != nil {
					hub.CaptureException(err.Err)
				} else {
					sentry.CaptureException(err.Err)
				}
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
			slog.Warn("HTTP 4xx/5xx",
				"method", method,
				"path", path,
				"status", status,
				"latency", latency.String(),
				"requestID", requestID,
			)
		}
	}
}
