// middleware.go：固定顺序中间件链实现（顺序装配见 server.go）。
package transport

import (
	"log/slog"

	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
)

// mwRequestID 保证 request_id/trace_id（保留上游 X-Request-ID/traceparent）并回写响应头。
func (s *Server) mwRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, reqID, traceID := observability.EnsureIDs(c.Request.Context(), c.GetHeader("traceparent"))
		if incoming := c.GetHeader("X-Request-ID"); incoming != "" {
			reqID = incoming // 上游/网关已生成 → 保留
			ctx = observability.WithRequestID(ctx, reqID)
		}
		c.Header("X-Request-ID", reqID)
		c.Set("request_id", reqID)
		c.Request = c.Request.WithContext(observability.WithRequestID(observability.WithTraceID(ctx, traceID), reqID))
		c.Next()
	}
}

// mwMetrics 指标采集（在途数/时长直方图/计数；标签低基数）。
func (s *Server) mwMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsWebSocketUpgrade(c.Request) {
			c.Next() // 连接面时长不进 na_http_duration（会话级扭曲——realtime 指标独立承载）
			return
		}
		if s.metrics == nil {
			c.Next()
			return
		}
		s.metrics.InFlight.Inc()
		start := time.Now()
		c.Next()
		s.metrics.InFlight.Dec()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		module := c.GetString("route_module")
		if module == "" {
			module = "edge"
		}
		methodLabel := c.Request.Method
		if route == "unmatched" {
			methodLabel = "-" // 未匹配路由的任意 method 不进标签（防基数攻击）
		}
		s.metrics.HTTPDuration.WithLabelValues(module, "access", methodLabel, strconv.Itoa(c.Writer.Status())).
			Observe(time.Since(start).Seconds())
		s.metrics.RequestsTotal.WithLabelValues(module, "access", methodLabel, strconv.Itoa(c.Writer.Status())).Inc()
	}
}

// mwRecovery panic 隔离：转 10009 统一信封（不外泄堆栈），日志记堆栈。
func (s *Server) mwRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if p := recover(); p != nil {
				s.logger.Error("transport: panic recovered", "path", c.Request.URL.Path, "panic", fmt.Sprint(p), "stack", string(debug.Stack()))
				Fail(c, errorx.New(errorx.CodeInternal))
			}
		}()
		c.Next()
	}
}

// mwBodyLimit 请求体上限（Content-Length 预检 + http.MaxBytesReader；超限 413）。
// 仅 http.MaxBytesError（真超限）触发 413——绑定类错误走各自 handler 的错误映射。
func (s *Server) mwBodyLimit() gin.HandlerFunc {
	limit := s.opts.BodyLimitBytes
	return func(c *gin.Context) {
		if c.Request.ContentLength > limit {
			writeTooLarge(c)
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
		for _, e := range c.Errors {
			var mbe *http.MaxBytesError
			if errors.As(e.Err, &mbe) {
				writeTooLarge(c)
				return
			}
		}
	}
}

func writeTooLarge(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge,
		errorx.NewErrorEnvelope(errorx.New(errorx.CodeInvalidParams, "reason", "body too large"), c.GetString("request_id"), ""))
}

// mwTimeout 每请求超时（Timeout；30s 基线，超时取消 ctx——
// handler 需尊重 ctx；响应已写出则取消无副作用）。
// WS upgrade 豁免（ws-protocol §1「Upgrade 路径必须绕过 TimeoutHandler」）：
// 升级后连接生命周期归 Hub，30s 切流=长连接必断（code-audit P1 探针实证）。
func (s *Server) mwTimeout() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsWebSocketUpgrade(c.Request) {
			c.Next()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// IsWebSocketUpgrade 识别 WS 握手请求（RFC 6455 Upgrade: websocket）——
// 超时豁免（mwTimeout）与 access 审计旁路（auditmw）共用。
func IsWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// 安全响应头具名值（项 7——CSP/token 安全面收敛；G1-08 具名）。
const (
	// HeaderHSTS 是 HSTS 响应头名。
	HeaderHSTS = "Strict-Transport-Security"
	// hstsHeaderValue：一年期 + 含子域（prod env 才发——staging/dev 是 HTTP
	// 联调面，下发会锁死明文访问）。
	hstsHeaderValue = "max-age=31536000; includeSubDomains"
	// headerCSP / headerCSPReportOnly 是 W3C CSP 头名（enforce / report-only 双模）。
	headerCSP           = "Content-Security-Policy"
	headerCSPReportOnly = "Content-Security-Policy-Report-Only"
	// defaultCSPReportOnly 是 Report-Only 宽策略基线（admin SPA 同源静态 + PWA
	// + inline 脚本/样式兼容——不破坏；运维经 config.Server.CSP 显式收紧后转
	// enforce，渐进路径：Report-Only 观测 → 收紧策略 → enforce）。
	defaultCSPReportOnly = "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
		"style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; " +
		"connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
)

// mwSecurityHeaders 安全响应头（项 7；observability/安全基线）。
// 链位=引擎功能链最早（Recovery 兜底位之后）：任何短路路径（413 body limit/
// ReceivedHooks 拒绝/CORS preflight/NoRoute SPA 静态）均带安全头。
//   - 基线三头：nosniff / DENY / strict-origin-when-cross-origin（恒发）。
//   - HSTS：仅 Options.Env=prod 下发。
//   - CSP：opts.CSP 空=Report-Only 宽策略默认（不破坏 SPA/PWA）；非空=同名
//     enforce 头直发（运维显式收紧）。
func (s *Server) mwSecurityHeaders() gin.HandlerFunc {
	sendHSTS := s.opts.Env == "prod"
	csp := s.opts.CSP
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		if sendHSTS {
			c.Header(HeaderHSTS, hstsHeaderValue)
		}
		if csp != "" {
			c.Header(headerCSP, csp)
		} else {
			c.Header(headerCSPReportOnly, defaultCSPReportOnly)
		}
		c.Next()
	}
}

// mwCORS 白名单 CORS（禁通配符+凭证组合，security.md §5）。
func (s *Server) mwCORS() gin.HandlerFunc {
	allow := map[string]bool{}
	for _, o := range s.opts.CORSOrigins {
		allow[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		if allow[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// accessLogger 是访问流水出口（Options.AccessLogger 优先——分文件面；
// nil 回退主 logger）。
func (s *Server) accessLogger() *slog.Logger {
	if s.opts.AccessLogger != nil {
		return s.opts.AccessLogger
	}
	return s.logger
}

// mwAccessLog 访问日志（type=access；字段含 request_id/status/latency）。
func (s *Server) mwAccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsWebSocketUpgrade(c.Request) {
			c.Next() // 连接面日志由 realtime hooks/指标承载（会话时长不进访问日志）
			return
		}
		start := time.Now()
		c.Next()
		reqID, _ := c.Get("request_id")
		s.accessLogger().Info("access",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"request_id", reqID,
			"trace_id", observability.TraceIDFrom(c.Request.Context()),
			"bytes", c.Writer.Size(),
		)
	}
}

// HeaderTraceparent 是 W3C 传播头（键名由 OTel TraceContext propagator 定义，
// 此处具名供 middleware/测试引用）。
const HeaderTraceparent = "traceparent"

// mwTracing HTTP server span：extract 上游 traceparent → 开 span
// （名=METHOD + 路由模板，低基数；未匹配路由固定 /unmatched）→ 记录状态/错误 →
// end。Tracing 未接线时直通（零开销）。panic 经本中间件时仍 End（错误入 span）
// 后 re-panic 交 Recovery 兜底。
func (s *Server) mwTracing() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.tracing.Enabled() {
			c.Next()
			return
		}
		ctx := observability.ExtractTraceContext(c.Request.Context(),
			map[string]string{HeaderTraceparent: c.GetHeader(HeaderTraceparent)})
		route := c.FullPath()
		if route == "" {
			route = "/unmatched" // 未匹配路由路径不进 span 名（防基数）
		}
		ctx, span := s.tracing.StartSpan(ctx, "transport", c.Request.Method+" "+route,
			trace.SpanKindServer, attribute.String("http.request.method", c.Request.Method), attribute.String("http.route", route))
		c.Request = c.Request.WithContext(ctx)
		// panic 由链首 mwRecovery 兜底（位次在本 mw 之前），此处只记录响应结果；
		// handler panic 时 Writer.Status()=10009 响应已由 Recovery 写出。
		defer func() {
			span.SetAttributes(attribute.Int("http.response.status_code", c.Writer.Status()))
			if c.Writer.Status() >= http.StatusInternalServerError {
				span.RecordError(fmt.Errorf("http status %d", c.Writer.Status()))
				span.SetAttributes(attribute.Bool("error", true))
			}
			span.End()
		}()
		c.Next()
	}
}
