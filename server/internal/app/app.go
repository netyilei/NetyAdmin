package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/middleware"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/pubsub"
	"NetyAdmin/internal/pkg/recovery"
	pkgSentry "NetyAdmin/internal/pkg/sentry"
	"NetyAdmin/internal/pkg/task"
	logService "NetyAdmin/internal/service/log"
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
//       srv.Shutdown       ≤ 30s （实际通常远小于此值）
//       taskManager.Stop   ≤ 5s  （stopWithTimeout 包装）
//       logBus.Stop        ≤ 5s  （stopWithTimeout 包装）
//       pkgSentry.Flush    ≤ 2s
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
	tm              *database.TransactionManager
	dbHealthChecker *database.HealthChecker
	taskManager     *task.Manager
	logBus          logService.LogBusService
	eventBus        pubsub.EventBus
}

func NewApp(cfg *config.Config, db *gorm.DB, engine *gin.Engine, tm *database.TransactionManager, dbHealthChecker *database.HealthChecker, taskManager *task.Manager, logBus logService.LogBusService, eventBus pubsub.EventBus) *App {
	return &App{
		cfg:             cfg,
		db:              db,
		engine:          engine,
		tm:              tm,
		dbHealthChecker: dbHealthChecker,
		taskManager:     taskManager,
		logBus:          logBus,
		eventBus:        eventBus,
	}
}

func (a *App) Run() error {
	addr := fmt.Sprintf(":%d", a.cfg.Server.Port)
	// 超时值从配置读取，避免硬编码（config.toml: read_timeout/write_timeout，单位秒）
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
		recovery.GoSafe("app:http_redirect", func() {
			slog.Info("HTTP→HTTPS 跳转服务启动", "addr", redirectSrv.Addr)
			if err := redirectSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("HTTP→HTTPS 跳转服务启动失败", "error", err)
				os.Exit(1)
			}
		})
	}

	// 2. Start Web Server
	recovery.GoSafe("app:web_server", func() {
		if a.cfg.TLS.Enable {
			slog.Info("服务器启动（TLS）", "addr", addr)
			if err := srv.ListenAndServeTLS(a.cfg.TLS.CertFile, a.cfg.TLS.KeyFile); err != nil && err != http.ErrServerClosed {
				slog.Error("启动 TLS 服务器失败", "error", err)
				os.Exit(1)
			}
		} else {
			slog.Info("服务器启动", "addr", addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("启动服务器失败", "error", err)
				os.Exit(1)
			}
		}
	})

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

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("服务器强制关闭: %v", err)
	}

	// 关闭 HTTP→HTTPS 跳转服务（若启用 TLS）
	if redirectSrv != nil {
		if err := redirectSrv.Shutdown(ctx); err != nil {
			slog.Warn("关闭 HTTP→HTTPS 跳转服务失败", "error", err)
		}
	}

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
