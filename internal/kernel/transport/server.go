// server.go：HTTP Server 组装——Gin 引擎 + 固定顺序中间件链 + 健康端点 + /metrics。
// 中间件固定顺序（2026-11-21 audit D 组根修）：
// Recovery → SecurityHeaders → RequestID → Tracing → Metrics → Timeout
// → BodyLimit → RequestReceivedHooks → CORS → AccessLog。
// Received 段=引擎链（鉴权前）；Authenticated/Before/After/Sent/Error 五段=
// 组级 stage mw（Group 挂载，AuthN 之后——principal 注入后语义；Register 路径
// 由 wrapHandlerStages 同体夹点）。业务路由一律经 Group/Register 注册（TR-05）。
package transport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/hooks"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/state/coordination"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Options 是 Server 构造参数（来自 config.Server 组装， 注入）。
type Options struct {
	// TrustedProxies 是反向代理 CIDR 白名单（X-Forwarded-For 解析信任面；
	// nil/空=不信任任何代理头，ClientIP 取直连地址——默认即现状）。
	TrustedProxies []string
	// AccessLogger 是访问流水出口（nil=用 Logger——分文件面：logs/<date>/
	// transport_access.log——2026-11-19访问/错误/调试日志全部文件化不进 DB）。
	AccessLogger   *slog.Logger
	Addr           string   // 监听地址（host:port）
	Mode           string   // gin 模式：release（生产默认）| debug
	BodyLimitBytes int64    // 请求体上限（0=默认 10MB）
	CORSOrigins    []string // CORS 白名单（空=不启用 CORS；禁通配符+凭证组合）
	ReadHeaderTO   time.Duration
	// Hooks 是请求 Hook 注册表（TR-05 六段接线；nil = 不接阶段，零开销）。
	Hooks *hooks.Registry
	// Env 是部署环境（dev|staging|prod）。prod 才发 HSTS（安全响应头项 7
	// ——非 prod 为 HTTP 联调面，下发会锁死明文访问）。
	Env string
	// CSP 是 Content-Security-Policy 策略串（项 7）。空=默认发
	// Content-Security-Policy-Report-Only 宽策略（SPA/PWA 渐进收紧不破坏）；
	// 非空=发 Content-Security-Policy 强制 enforce。
	CSP string
	// Tracing 是 OTel 接线句柄（nil/未接线 = 不开 span，零开销）。
	Tracing *observability.Tracing
	// Idempotency 是幂等键协调面（TR-01；nil = Idempotency-Key 中间件直通）。
	Idempotency *coordination.Coordination
}

// ReadinessFunc 是 /health/ready 的就绪探针（聚合器注入）。
type ReadinessFunc func(ctx context.Context) (bool, map[string]string)

// Server 是 HTTP 传输面。
type Server struct {
	engine      *gin.Engine
	logger      *slog.Logger
	metrics     *observability.Metrics
	hooks       *hooks.Registry            // nil = 请求 Hook 阶段直通（TR-05）
	tracing     *observability.Tracing     // nil/未接线 = 无 span
	idempotency *coordination.Coordination // nil = Idempotency-Key 直通（TR-01）
	stageMWName string                     // stage mw 方法值运行时名（链内同类实例识别）
	opts        Options
	readyFn     ReadinessFunc
	server      *http.Server
}

// New 创建 Server 并装配固定中间件链与健康/指标端点。
func New(opts Options, logger *slog.Logger, metrics *observability.Metrics, ready ReadinessFunc) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if opts.Mode == "" {
		opts.Mode = gin.ReleaseMode
	}
	if metrics == nil {
		return nil, errors.New("transport: metrics must not be nil")
	}
	gin.SetMode(opts.Mode)
	if opts.BodyLimitBytes <= 0 {
		opts.BodyLimitBytes = 10 << 20
	}
	if opts.ReadHeaderTO <= 0 {
		opts.ReadHeaderTO = 10 * time.Second
	}

	s := &Server{engine: gin.New(), logger: logger, metrics: metrics, hooks: opts.Hooks, tracing: opts.Tracing, idempotency: opts.Idempotency, opts: opts, readyFn: ready}
	s.stageMWName = stageMWNameFunc(s.handlerStageMW) // 嵌套组同类实例识别（request_hooks.go）

	// 代理白名单配置化（触发点=部署形态已定：反代部署
	// 显式登记代理 CIDR——X-Forwarded-For 解析；nil/空=不信任任何代理头，
	// ClientIP 取直连地址——X-Forwarded-For 可伪造（安全基线））
	if err := s.engine.SetTrustedProxies(opts.TrustedProxies); err != nil {
		return nil, fmt.Errorf("transport: trusted proxies: %w", err)
	}

	// —— 固定顺序中间件链（+ §7.3 钩子段位） ——
	// 幂等键占位不在引擎链：组级挂载于 authn 之后（Server.Idempotency，
	// L48 根修——未认证请求不得造键）。
	// 安全响应头=引擎功能链最早位（Recovery 兜底位之后——项 7）：任何短路
	// 路径（413 body limit/ReceivedHooks 拒绝/CORS preflight/NoRoute SPA）均带
	// 安全头。Authenticated+四段不在引擎链：组级 stage mw 挂载（AuthN 之后——
	// audit D①根修：引擎链位先于组级 AuthN 时 principal 永缺席）。
	s.engine.Use(
		s.mwRecovery(), // 链首兜底：任何后续 mw/handler panic 均转 10009 信封
		s.mwSecurityHeaders(),
		s.mwRequestID(),
		s.mwTracing(), // ：extract 上游 traceparent → server span（包全链路延迟）
		s.mwMetrics(),
		s.mwTimeout(),
		s.mwBodyLimit(),
		s.mwRequestReceivedHooks(), // §7.3：body limit 后、鉴权前（可短路）
		s.mwCORS(),
		s.mwAccessLog(),
	)

	// 健康与指标端点（ops 面）
	s.engine.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	s.engine.GET("/health/ready", func(c *gin.Context) {
		if s.readyFn == nil {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "checks": map[string]string{}})
			return
		}
		ok, details := s.readyFn(c.Request.Context())
		code := http.StatusOK
		if !ok {
			code = http.StatusServiceUnavailable
		}
		c.JSON(code, gin.H{"status": map[bool]string{true: "ok", false: "unready"}[ok], "checks": details})
	})
	s.engine.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metrics.Registry(), promhttp.HandlerOpts{})))

	// 未匹配路由统一 404 信封（错误一律 errorx Envelope——gin 默认文本 404
	// 违反契约面；模块开关关闭后端点「不存在」也走此语义，如 users.enabled=false
	// 的 /client/v1/users/*）。组合根 SPA 面可用 Engine().NoRoute 覆盖（gin
	// 语义=替换，admin/dist 存在时静态 fallback 优先；API 前缀仍回信封）。
	s.engine.NoRoute(func(c *gin.Context) {
		FailOf(c, errorx.New(errorx.CodeNotFound))
	})

	return s, nil
}

// Engine 暴露 gin engine（组合根 static/SPA NoRoute 挂载专用——业务
// 路由仍一律走 Register/Group 管线；第三方滥用面=组合根唯一）。
func (s *Server) Engine() *gin.Engine { return s.engine }

// Register 注册业务路由（module 写入 route_module 供指标标签；模块经装配调用，
// handler 夹 request hook 的 Before/After/ResponseSent/RequestError 段）。
func (s *Server) Register(method, path, module string, handler gin.HandlerFunc) error {
	s.logger.Info("transport: register route", "module", module, "method", method, "path", path)
	staged := s.wrapHandlerStages(handler)
	wrapped := func(c *gin.Context) {
		c.Set("route_module", module)
		staged(c)
	}
	switch method {
	case http.MethodGet:
		s.engine.GET(path, wrapped)
	case http.MethodPost:
		s.engine.POST(path, wrapped)
	case http.MethodPut:
		s.engine.PUT(path, wrapped)
	case http.MethodPatch:
		s.engine.PATCH(path, wrapped)
	case http.MethodDelete:
		s.engine.DELETE(path, wrapped)
	default:
		return fmt.Errorf("transport: unsupported method %q", method)
	}
	return nil
}

// stagedGroup 是组路由 stage 包装器（TR-05 audit D③根修：Group 注册路径与
// Register 同获五段夹点）。内嵌 *gin.RouterGroup——IRoutes 方法全部透传
// （handler 注册走 gin 原生，stages 由组链尾 stage mw 承载）；仅影子 Group：
// 嵌套组同样追加 stage mw 实例（子组 AuthN 之后），请求期由链内最深实例唯一
// 触发。Group 签名与 *RouterGroup 一致 → 满足 gin.IRouter/IRoutes——装配与
// 模块侧签名零变更。
type stagedGroup struct {
	*gin.RouterGroup
	srv *Server
}

// Group 影子方法：嵌套组追加自持 stage mw（gin 自动继承父组链——父组实例在
// 子组 AuthN 之前，让位给本实例；hasDeeperStageMW 保证唯一触发）。
func (g stagedGroup) Group(relativePath string, middlewares ...gin.HandlerFunc) *gin.RouterGroup {
	sub := g.RouterGroup.Group(relativePath, middlewares...)
	if g.srv.hooks != nil {
		sub.Use(g.srv.mwHandlerStageHooks())
	}
	return sub
}

// Group 创建版本路由组（版本化挂载；组级中间件先于组内路由执行；
// gin 允许 Serve 前随时加组——bootstrap AfterBootstrap 段内注册）。
// 组链尾追加 stage mw（Authenticated/Before/After/Sent/Error 五段夹点，TR-05；
// AuthN 组级中间件由装配方挂在本 mw 之前——§7.3「principal 注入后」语义）。
func (s *Server) Group(relativePath string, middlewares ...gin.HandlerFunc) gin.IRouter {
	g := s.engine.Group(relativePath, middlewares...)
	if s.hooks != nil {
		g.Use(s.mwHandlerStageHooks())
	}
	return stagedGroup{RouterGroup: g, srv: s}
}

// HTTP server 超时基线（慢速攻击防护——stdlib 默认 0=永不超时，Slowloris
// 可无限占连接）。选择理由：
//   - ReadHeaderTimeout：10s（Options.ReadHeaderTO 可覆盖）——只约束 header 读取；
//   - ReadTimeout：60s——约束「header+body」整体读取。本进程无 SSE；WS upgrade
//     走 hijack（net/http 交接后不受 ReadTimeout 约束 ✓）；client 大文件上传是
//     presigned 直传对象存储（不经本进程 body ✓），body 上限 10MB（BodyLimit），
//     60s 对 10MB≈170KB/s 下限，公网/内网均足够；
//   - IdleTimeout：120s——keep-alive 空闲连接回收（负载均衡器默认空闲阈通常 ≥60s）。
//
// 不设 ReadTimeout=0：慢速 body 攻击面（chunked 逐字节）必须封堵。
const (
	defaultIdleTimeout = 120 * time.Second
	defaultReadTimeout = 60 * time.Second
)

// Build 构造 *http.Server（把 ListenAndServe 交给 bootstrap.ServeFunc）。
func (s *Server) Build() *http.Server {
	s.server = &http.Server{
		Addr:              s.opts.Addr,
		Handler:           s.engine,
		ReadHeaderTimeout: s.opts.ReadHeaderTO,
		ReadTimeout:       defaultReadTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}
	return s.server
}

// ListenAndServe 阻塞运行（bootstrap.ServeFunc 契约）。
func (s *Server) ListenAndServe() error {
	if s.server == nil {
		s.Build()
	}
	return s.server.ListenAndServe()
}

// Shutdown 优雅关闭。
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}
