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
	"NetyAdmin/internal/pkg/requestid"
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

				adminIDUint := extractAdminIDUint(c)

				// LogPanic 调用包裹内层 recover：LogPanic 内部若 panic（如 DB 卡死、
				// log_bus 阻塞 panic）不会导致进程崩溃。内层 recover 仅记录到 slog.Error，
				// 不影响外层 Recovery 中间件返回 500 给客户端。
				func() {
					defer func() {
						if e := recover(); e != nil {
							slog.Error("Recovery: LogPanic panicked",
								"panic", e, "requestID", requestID, "path", c.Request.URL.Path)
						}
					}()
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
				}()

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
		// 1. 保留 gin.Context 中的 requestID 字符串，向后兼容 c.GetString("requestID")
		//    （recovery / sentry_tag_setter / response.Fail / Logger 等老路径仍读此值）
		c.Set("requestID", requestID)
		c.Header("X-Request-ID", requestID)
		// 2. 注入到 c.Request.Context()，让 service / repository 通过 ctx 读取
		//    （requestid.FromContext(ctx) / slogutil.LoggerFromContext(ctx)）
		//    c.Request.WithContext 必须赋值回 c.Request，否则 ctx 不会生效
		c.Request = c.Request.WithContext(requestid.WithRequestID(c.Request.Context(), requestID))
		c.Next()
	}
}

// extractAdminIDUint 提取当前请求的用户 ID 并转换为 uint（供 DB error_log.AdminID 字段）。
// Admin 端 JWTAuth 设置 adminID(uint)；Client 端 UserJWTAuth 设置 userID(string 业务ID)。
// 由于 DB 字段为 uint 且语义为"管理员 ID"，Client 端的字符串业务 ID 不强转（保留为 0），
// 避免溢出/语义混淆；Client 端的用户标识通过 Sentry 上下文单独保留（见 setSentryUserFromContext）。
func extractAdminIDUint(c *gin.Context) uint {
	if uid, exists := c.Get("adminID"); exists {
		switch v := uid.(type) {
		case uint:
			return v
		}
	}
	return 0
}

// userIdentifier 是注入 Sentry Scope 的用户标识，兼容 Admin(uint) 与 Client(string) 两端。
type userIdentifier struct {
	id       string // Sentry User.ID（字符串形式）
	username string // 可选，Admin 端可从 username context 取
	hasUser  bool   // 是否识别到用户
}

// extractSentryUser 提取 Sentry 用户上下文标识，兼容两端：
//   - Admin 端：adminID(uint) + username(string)
//   - Client 端：userID(string 业务ID)
//
// 关键修复（H1）：Client 端 userID 为 string 类型，旧代码 type switch 仅匹配 uint，
// 导致 Sentry User.ID 丢失；此处显式处理 string 分支。
func extractSentryUser(c *gin.Context) userIdentifier {
	// Admin 端：adminID (uint)
	if uid, exists := c.Get("adminID"); exists {
		if v, ok := uid.(uint); ok && v > 0 {
			u := userIdentifier{id: strconv.FormatUint(uint64(v), 10), hasUser: true}
			if uname, ok := c.Get("username"); ok {
				if s, ok := uname.(string); ok && s != "" {
					u.username = s
				}
			}
			return u
		}
	}
	// Client 端：userID (string 业务ID)
	if uid, exists := c.Get("userID"); exists {
		if s, ok := uid.(string); ok && s != "" {
			return userIdentifier{id: s, hasUser: true}
		}
	}
	return userIdentifier{}
}

// applySentryUser 将用户标识注入 Sentry Scope（hub 为 nil 时为空操作）。
// 同时设置 ID 与 username（如有），符合 ADR-008 用户上下文规范。
func applySentryUser(hub *sentry.Hub, u userIdentifier) {
	if hub == nil || !u.hasUser {
		return
	}
	user := sentry.User{ID: u.id}
	if u.username != "" {
		user.Username = u.username
	}
	hub.Scope().SetUser(user)
}

// SentryTagSetter 在 sentrygin 之后运行，将 requestID、path、method 注入 Sentry Scope。
// 注意：此中间件作为全局中间件运行，早于 JWTAuth（authGroup/permissionGroup 内），
// 因此 adminID/userID 尚未设置。用户 ID 的注入在 ErrorLogger/Recovery 中完成（c.Next() 之后）。
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
// c.Next() 之后 JWTAuth 已执行，adminID/userID 已可用，此处注入用户上下文到 Sentry Scope。
func ErrorLogger(errorLogSvc logService.ErrorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		requestID := c.GetString("requestID")
		adminIDUint := extractAdminIDUint(c)
		hub := sentrygin.GetHubFromContext(c)

		// 注入用户上下文到 Sentry Scope（H1+H2：兼容 Client string userID + 补 username）
		applySentryUser(hub, extractSentryUser(c))

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
