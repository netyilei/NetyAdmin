package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/middleware"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/database"
	jwtPkg "NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/pubsub"
	"NetyAdmin/internal/pkg/recovery"
	pkgSentry "NetyAdmin/internal/pkg/sentry"
	"NetyAdmin/internal/pkg/task"
	userRepoPkg "NetyAdmin/internal/repository/user"
	ipacService "NetyAdmin/internal/service/ipac"
	logService "NetyAdmin/internal/service/log"
	msgService "NetyAdmin/internal/service/message"
	userServicePkg "NetyAdmin/internal/service/user"
)

// drainTimeout 是 taskManager.Stop / logBus.Stop 的最大等待时长。
// 超过此时间仍在执行的内部 goroutine 会被放弃（进程退出时由 OS 回收），
// 避免 drain 卡死导致 SIGKILL 强杀。
//
// drainTimeout 与 ShutdownTimeout 的层级关系（重要）：
//   - ShutdownTimeout（默认 30s，env: NETYADMIN_SERVER_SHUTDOWN_TIMEOUT）：
//     srv.Shutdown 等待在途 HTTP 请求完成的最大时长，是整个关闭流程的总预算。
//   - drainTimeout（5s，本常量）：
//     taskManager.Stop / logBus.Stop 单步 drain 的最大时长，由 stopWithTimeout 包装执行。
//   - pkgSentry.Flush 自带 2s 内部超时（见 Run 末尾）。
//   - 最坏耗时（假设 srv.Shutdown 不超占预算，HTTP 请求一般 ms 级完成）：
//     srv.Shutdown       ≤ 30s （实际通常远小于此值）
//     taskManager.Stop   ≤ 5s  （stopWithTimeout 包装）
//     logBus.Stop        ≤ 5s  （stopWithTimeout 包装）
//     pkgSentry.Flush    ≤ 2s
//     drain 段合计 ≤ 12s（5s + 5s + 2s），远小于 30s 总预算，
//     留有 18s+ 缓冲应对 dbHealthChecker.Stop / eventBus.Close / sqlDB.Close 的偶发抖动。
//   - drainTimeout 故意远小于 ShutdownTimeout：保证 drain 步骤即便全部超时，
//     仍有充裕时间完成 sqlDB.Close 与 Sentry Flush，避免被 K8s/systemd SIGKILL 强杀
//     导致缓冲日志未刷盘 / Sentry 事件丢失。
//   - 注意：srv.Shutdown 与后续步骤共享同一个 ctx，因此 30s 是「整个关闭流程」
//     的上限而非 srv.Shutdown 单步的上限。当前 drain / sqlDB.Close / Sentry Flush
//     未显式 select ctx.Done()，但各自都有独立超时兜底（drainTimeout / 2s），
//     不会因 ctx 已超时而无限阻塞。
const drainTimeout = 5 * time.Second

// defaultShutdownTimeout 是 Server.ShutdownTimeout 配置为零值时的兜底。
const defaultShutdownTimeout = 30 * time.Second

// defaultHandlerTimeout 是 Server.HandlerTimeout 配置为零值时的兜底。
// 应略小于 Server.ReadTimeout / WriteTimeout（默认 120s），
// 确保超时时由 http.TimeoutHandler 返回 503 + JSON 错误体，
// 而非连接层超时断开（客户端收到空响应 / 连接重置）。
const defaultHandlerTimeout = 25 * time.Second

type App struct {
	cfg             *config.Config
	db              *gorm.DB
	engine          *gin.Engine
	tm              database.TxManager
	dbHealthChecker *database.HealthChecker
	taskManager     *task.Manager
	// lifecycleModules 是实现了 LifecycleModule 的下游模块（RegisterModule 收集）。
	// Run 时 OnStart / 优雅关闭时 OnStop（见 runLifecycleStart / runLifecycleStop）。
	lifecycleModules []LifecycleModule
	logBus           logService.LogBusService
	eventBus         pubsub.EventBus

	// Security infrastructure — exposed for downstream projects to compose
	// custom auth flows (SMS login, OAuth, biometric, etc.) without modifying
	// the base framework.
	jwt          *jwtPkg.JWT
	tokenStore   userServicePkg.TokenStore
	verifySvc    userServicePkg.VerificationService
	userRepo     userRepoPkg.UserRepository
	cacheSlow    cache.SecurityCache
	oauthBinding userServicePkg.OAuthBindingService

	// Module 装配扩展点（供 RegisterModule 使用，下游通过 App.RegisterModule 注入业务模块）
	started          bool // Run() 启动后置 true，RegisterModule 检测此标志拒绝启动后注册
	authMW           *middleware.AuthMiddleware
	authVerifier     middleware.AuthVerifier
	ipacSvc          ipacService.IPACService
	openPlatformAuth gin.HandlerFunc
	routerDeps       RouterDeps

	// Infrastructure — exposed via getters for downstream service construction
	// (repos, caches, message service, etc.) via App.DB() / App.CacheMgr() / ...
	redisClient   *redis.Client
	cacheMgr      *cache.LazyCacheManager
	userTokenRep  userRepoPkg.UserTokenRepository
	messageSvc    msgService.MessageService
	configWatcher configsync.ConfigWatcher
}

func NewApp(
	cfg *config.Config,
	db *gorm.DB,
	engine *gin.Engine,
	tm database.TxManager,
	dbHealthChecker *database.HealthChecker,
	taskManager *task.Manager,
	logBus logService.LogBusService,
	eventBus pubsub.EventBus,
	jwtInstance *jwtPkg.JWT,
	tokenStore userServicePkg.TokenStore,
	verifySvc userServicePkg.VerificationService,
	userRepo userRepoPkg.UserRepository,
	cacheSlow cache.SecurityCache,
	oauthBinding userServicePkg.OAuthBindingService,
	// Module 装配依赖（下游 RegisterModule 时使用）
	authMW *middleware.AuthMiddleware,
	authVerifier middleware.AuthVerifier,
	ipacSvc ipacService.IPACService,
	openPlatformAuth gin.HandlerFunc,
	// Infrastructure（下游 getter 暴露）
	redisClient *redis.Client,
	cacheMgr *cache.LazyCacheManager,
	userTokenRep userRepoPkg.UserTokenRepository,
	messageSvc msgService.MessageService,
	configWatcher configsync.ConfigWatcher,
) *App {
	return &App{
		cfg:              cfg,
		db:               db,
		engine:           engine,
		tm:               tm,
		dbHealthChecker:  dbHealthChecker,
		taskManager:      taskManager,
		logBus:           logBus,
		eventBus:         eventBus,
		jwt:              jwtInstance,
		tokenStore:       tokenStore,
		verifySvc:        verifySvc,
		userRepo:         userRepo,
		cacheSlow:        cacheSlow,
		oauthBinding:     oauthBinding,
		authMW:           authMW,
		authVerifier:     authVerifier,
		ipacSvc:          ipacSvc,
		openPlatformAuth: openPlatformAuth,
		redisClient:      redisClient,
		cacheMgr:         cacheMgr,
		userTokenRep:     userTokenRep,
		messageSvc:       messageSvc,
		configWatcher:    configWatcher,
		routerDeps: RouterDeps{
			AuthMiddleware:   authMW,
			OpenPlatformAuth: openPlatformAuth,
			JWT:              jwtInstance,
			Config:           cfg,
		},
	}
}

// --- Security infrastructure getters ---
//
// Downstream projects use these to build custom authentication flows
// (SMS code login, third-party OAuth, biometric, etc.) without forking
// the base framework or re-implementing JWT/tokenStore/verification.

// JWT returns the RS256 JWT instance for token signing and verification.
func (a *App) JWT() *jwtPkg.JWT { return a.jwt }

// TokenStore returns the token hash store for session management
// (access/refresh token storage, blacklist, multi-device session control).
func (a *App) TokenStore() userServicePkg.TokenStore { return a.tokenStore }

// VerificationService returns the verification code service (SMS/email captcha
// sending and validation). Downstream projects use this to implement
// phone-code login, password reset, etc.
func (a *App) VerificationService() userServicePkg.VerificationService { return a.verifySvc }

// UserRepository returns the user repository for account lookup
// (GetByID, GetByPhone, GetByEmail, etc.). Downstream projects use this
// to implement custom login flows that need to query the users table.
func (a *App) UserRepository() userRepoPkg.UserRepository { return a.userRepo }

// SecurityCache returns the L2 security cache (Redis-backed) for sensitive
// data like auth state, login rate limit counters, token blacklists.
func (a *App) SecurityCache() cache.SecurityCache { return a.cacheSlow }

// OAuthBindingService returns the third-party OAuth binding service for
// account binding lookup and management (WeChat, Alipay, GitHub, Apple, etc.).
// Downstream projects implement provider adapters and delegate storage to this service.
func (a *App) OAuthBindingService() userServicePkg.OAuthBindingService { return a.oauthBinding }

// --- Infrastructure getters ---
//
// 下游模块构造 Service 层时需要的基础设施依赖。
// 与上面的安全组件 getter 同模式——下游通过这些 getter 拿到 DB / 缓存 / Redis 等。

// DB returns the GORM DB connection for downstream Repository construction.
func (a *App) DB() *gorm.DB { return a.db }

// TxManager returns the transaction manager for downstream transaction orchestration.
func (a *App) TxManager() database.TxManager { return a.tm }

// CacheMgr returns the lazy cache manager (L1 BigCache + L2 Redis + configWatcher联动)
// for downstream cache usage.
func (a *App) CacheMgr() *cache.LazyCacheManager { return a.cacheMgr }

// RedisClient returns the Redis client for downstream direct use
// (rate limiting, distributed locks, slot management, etc.).
//
// **WARNING**: Returns nil if Redis is disabled in config. Downstream code MUST
// nil-check before use (e.g., `if r := app.RedisClient(); r != nil { ... }`),
// otherwise calling methods on nil will panic. The base framework internally
// follows this pattern at all Redis usage sites.
func (a *App) RedisClient() *redis.Client { return a.redisClient }

// UserTokenRepository returns the user_tokens repository for downstream
// multi-device session management (platform-scoped token version checks,
// SSE session kick-in, etc.).
func (a *App) UserTokenRepository() userRepoPkg.UserTokenRepository { return a.userTokenRep }

// MessageService returns the unified message service (SMS/Email/Internal)
// for downstream notification features (e.g., admin push notifications).
func (a *App) MessageService() msgService.MessageService { return a.messageSvc }

// ConfigWatcher returns the hot-reload config watcher (sys_configs snapshot).
//
// 下游模块读取动态配置（如验证码长度、短信尝试次数、业务开关）时应使用本
// getter，而不是每次直读 sys_configs：ConfigWatcher 由基座统一缓存并在
// 配置变更时刷新，与基座登录参数等配置的即时生效体验一致。
// RawConfig 返回配置文件的原始配置树副本（map 形态，嵌套段为 map[string]any）。
// 下游模块用它解析基座强类型结构未覆盖的自有配置段（如 [ws]）；
// 返回副本，修改不影响基座。
func (a *App) RawConfig() map[string]any { return a.cfg.RawConfig() }

func (a *App) ConfigWatcher() configsync.ConfigWatcher { return a.configWatcher }

func (a *App) Run() error {
	a.started = true // 标记已启动，RegisterModule 此后调用会 panic（路由不可变更）
	addr := fmt.Sprintf(":%d", a.cfg.Server.Port)
	// 超时值从配置读取，避免硬编码（config.yaml: read_timeout/write_timeout，单位秒）
	readTimeout := time.Duration(a.cfg.Server.ReadTimeout) * time.Second
	writeTimeout := time.Duration(a.cfg.Server.WriteTimeout) * time.Second
	if readTimeout <= 0 {
		readTimeout = 30 * time.Second
	}
	if writeTimeout <= 0 {
		writeTimeout = 30 * time.Second
	}

	// 请求处理超时：用 http.TimeoutHandler 包装 engine，
	// 超时返回 503 + JSON 错误体（详见 middleware.WrapWithTimeout）。
	// 零值兜底为 defaultHandlerTimeout（25s），应略小于 readTimeout/writeTimeout。
	// cfg.Server.HandlerTimeout 类型为 config.Duration，调用 .Duration() 转 time.Duration。
	handlerTimeout := a.cfg.Server.HandlerTimeout.Duration()
	if handlerTimeout <= 0 {
		handlerTimeout = defaultHandlerTimeout
	}
	wrappedHandler := middleware.WrapWithTimeout(a.engine, handlerTimeout)

	srv := &http.Server{
		Addr:         addr,
		Handler:      wrappedHandler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// 1. Start task manager (Execute startup tasks like DB migration)
	if a.taskManager != nil {
		a.taskManager.Start(context.Background())
	}

	// 1.1 启动健康检查：启动期主动探活一次（依赖未就绪仅告警，不阻断启动）
	if a.dbHealthChecker != nil {
		a.dbHealthChecker.Start()
	}

	// TLS 配置（可选）：
	// - 默认关闭（cfg.TLS.Enable=false），由前端 Nginx 终止 TLS，本服务只跑 HTTP
	// - 启用时启动 443 HTTPS + 80 HTTP→HTTPS 跳转 goroutine
	// - 运维在无 Nginx 场景启用；有 Nginx 时由 Nginx 终止 TLS
	var redirectSrv *http.Server
	if a.cfg.TLS.Enable {
		// 80 端口 HTTP→HTTPS 跳转：301 Moved Permanently，重写 Host + URI 为 https
		redirectSrv = &http.Server{
			Addr: ":80",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				u := *r.URL
				u.Scheme = "https"
				u.Host = r.Host
				http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
			}),
		}
		// 服务器主 goroutine 不使用 GoSafe：若 ListenAndServe panic，GoSafe 会 recover
		// 但不退出，导致进程变成「活着但不监听端口」的僵尸状态。服务器主 goroutine
		// 的语义是「失败必须退出」——panic 时由 Go runtime 直接 crash 进程（期望行为）。
		// 这不违反 RULES.md §8.3：§8.3 禁止「裸 go func + manual recover 吞错」，
		// 此处是「裸 go func 无 recover」——panic 不被吞，而是 crash 进程。
		go func() {
			slog.Info("HTTP→HTTPS 跳转服务启动", "addr", redirectSrv.Addr)
			if err := redirectSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("HTTP→HTTPS 跳转服务启动失败", "error", err)
				os.Exit(1)
			}
		}()
	}

	// 2. 先建立监听器再启动服务 goroutine：
	//    - 监听失败同步返回（fail-fast），避免"进程活着但不监听端口"的僵尸态；
	//    - LifecycleModule OnStart 在端口真实可接受连接之后调用（见注释 2.1）。
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("端口监听失败 %s: %w", addr, err)
	}

	// 2.1 LifecycleModule OnStart：端口已真实监听（OnStart 时序契约见 module.go）。
	//     失败仅记录日志并继续——子系统启动失败不阻断基座主服务，
	//     由模块自行重试或降级（与任务引擎/健康检查的失败哲学一致）。
	a.runLifecycleStart(context.Background())

	go func() {
		if a.cfg.TLS.Enable {
			slog.Info("服务器启动（TLS）", "addr", addr)
			if err := srv.ServeTLS(ln, a.cfg.TLS.CertFile, a.cfg.TLS.KeyFile); err != nil && err != http.ErrServerClosed {
				slog.Error("启动 TLS 服务器失败", "error", err)
				os.Exit(1)
			}
		} else {
			slog.Info("服务器启动", "addr", addr)
			if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				slog.Error("启动服务器失败", "error", err)
				os.Exit(1)
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("正在关闭服务器...")

	// 优雅关闭前置检查：若有未提交事务，slog.Error 告警。
	// 这些事务在 srv.Shutdown 等待在途请求退出时可能被强制中断，导致数据丢失。
	if a.tm != nil {
		if active := a.tm.ActiveTransactions(); active > 0 {
			slog.Error("优雅关闭时检测到未提交事务，可能丢失数据", "active_transactions", active)
		}
	}

	// 优雅关闭超时：从配置读取，零值兜底为 30s。
	// cfg.Server.ShutdownTimeout 类型为 config.Duration，调用 .Duration() 转 time.Duration。
	shutdownTimeout := a.cfg.Server.ShutdownTimeout.Duration()
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	shutdownErr := srv.Shutdown(ctx)
	if shutdownErr != nil {
		slog.Error("HTTP 优雅关闭超时/失败：监听已停止，在途请求可能被中断", "error", shutdownErr)
	}

	// LifecycleModule OnStop：HTTP 已停止监听（无论 Shutdown 是否超时），
	// 下游组件（如 WS Hub）仍需在此停止——放行到 OnStop 之后统一返回错误。
	a.runLifecycleStop()

	// 关闭 HTTP→HTTPS 跳转服务（若启用 TLS）
	if redirectSrv != nil {
		if err := redirectSrv.Shutdown(ctx); err != nil {
			slog.Warn("关闭 HTTP→HTTPS 跳转服务失败", "error", err)
		}
	}

	if shutdownErr != nil {
		return fmt.Errorf("服务器强制关闭: %v", shutdownErr)
	}

	// LifecycleModule OnStop：HTTP 优雅关闭完成后调用（逆序），带超时保护。
	// srv.Shutdown 失败也会走到这里——监听已停止，该停的下游组件仍要停。
	a.runLifecycleStop()

	// Stop DB health checker
	if a.dbHealthChecker != nil {
		a.dbHealthChecker.Stop()
	}

	// Stop task manager（带 5s drain 超时，避免 cron/interval 任务卡死阻塞退出）
	if a.taskManager != nil {
		a.stopWithTimeout("taskManager", a.taskManager.Stop)
	}

	// Stop LogBus (flush all buckets，带 5s drain 超时，避免刷盘卡死阻塞退出)
	if a.logBus != nil {
		a.stopWithTimeout("logBus", a.logBus.Stop)
	}

	// Stop PubSubBus (close Redis subscription goroutine)
	if a.eventBus != nil {
		// explicitly ignored: eventBus.Close 在进程退出路径调用，
		// 失败也仅意味着 Redis 订阅 goroutine 残留，由 OS 在进程退出时回收。
		// 此处仅 Warn 留痕，便于运维排查 Redis 连接异常。
		if err := a.eventBus.Close(); err != nil {
			slog.Warn("eventBus.Close 失败（进程退出时由 OS 回收 goroutine）", "error", err)
		}
	}

	// 关闭数据库连接池：必须在 taskManager / logBus drain 完成之后，
	// 否则仍在执行的任务 / 刷盘 worker 会因连接已关闭而失败。
	if sqlDB, err := a.db.DB(); err == nil && sqlDB != nil {
		if err := sqlDB.Close(); err != nil {
			slog.Warn("关闭数据库连接池失败", "error", err)
		}
	} else {
		slog.Warn("获取底层 *sql.DB 失败，跳过显式关闭", "error", err)
	}

	// Flush Sentry buffer (ensure all pending events are sent before exit)
	pkgSentry.Flush(2 * time.Second)

	slog.Info("服务器已安全关闭")
	return nil
}

// runLifecycleStart 顺序调用各 LifecycleModule 的 OnStart（端口已监听后）。
// 单模块失败仅记录错误日志并继续——子系统启动失败不阻断基座主服务，
// 由模块自行重试或降级（与任务引擎/健康检查的失败哲学一致）。
func (a *App) runLifecycleStart(ctx context.Context) {
	for _, m := range a.lifecycleModules {
		// per-module recover：单个模块 OnStart panic 不得拖垮基座启动
		//（module.go LifecycleModule 契约：panic 与 error 同样只记日志降级）
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("lifecycle module OnStart panicked (module continues degraded)",
						"module", m.Name(), "panic", r)
				}
			}()
			if err := m.OnStart(ctx); err != nil {
				slog.Error("lifecycle module OnStart failed (module continues degraded)",
					"module", m.Name(), "err", err)
			}
		}()
	}
}

// runLifecycleStop 逆序调用各 LifecycleModule 的 OnStop（HTTP 优雅关闭后）。
// 单模块 5s 超时（复用 stopWithTimeout），panic 由 GoSafe 捕获，不阻塞进程退出。
func (a *App) runLifecycleStop() {
	for i := len(a.lifecycleModules) - 1; i >= 0; i-- {
		m := a.lifecycleModules[i]
		a.stopWithTimeout("lifecycle-stop:"+m.Name(), func() { m.OnStop(context.Background()) })
	}
}

// stopWithTimeout 在独立 goroutine 中执行 stopFn，最多等待 drainTimeout。
// 超时后仅 slog.Warn 告警并立即返回，不阻塞进程退出（仍在执行的 goroutine 由 OS 回收）。
//
// 用于包装 taskManager.Stop / logBus.Stop 等可能阻塞的 drain 方法。
// 这些方法内部用 wg.Wait() 等待 worker 退出，若 worker 卡死会导致整个关闭流程卡死。
func (a *App) stopWithTimeout(name string, stopFn func()) {
	done := make(chan struct{})
	recovery.GoSafe("app:drain:"+name, func() {
		defer close(done)
		stopFn()
	})
	select {
	case <-done:
		slog.Info("drain 完成", "component", name)
	case <-time.After(drainTimeout):
		slog.Warn("drain 超时，放弃等待（进程退出时由 OS 回收 goroutine）", "component", name, "timeout", drainTimeout)
	}
}
