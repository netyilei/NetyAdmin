// bootstrap.go：生命周期驱动器（收敛——注册/执行语义单一实现于 kernel/hooks）。
//
// 分工：hooks.Registry = 统一注册表（四类 19 阶段，优先级/超时/panic 语义单一实现）；
// 本包 = 生命周期 Run 驱动器（六段顺序、serve 阻塞契约、排空预算、ctx/serve 退出语义）。
// App.On/RunStage 转发 hooks.Registry；Stage 为类型别名（值互通）。
// 配置加载在 BeforeBootstrap 前由调用方完成；信号监听在 cmd——本包只认 ctx。
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/hooks"
)

// Stage 是生命周期阶段（类型别名：与 hooks.Stage 值互通， 收敛）。
var Stages = hooks.Stages // 诊断遍历用（hooks.Stages 为全阶段有序列表）
type Stage = hooks.Stage

// 六段生命周期（hooks 元数据表为策略唯一实现：fail-fast×3/排空 5s/AfterShutdown 独立预算）。
const (
	StageBeforeBootstrap = hooks.StageBeforeBootstrap
	StageAfterBootstrap  = hooks.StageAfterBootstrap
	StageBeforeListen    = hooks.StageBeforeListen
	StageAfterListen     = hooks.StageAfterListen
	StageBeforeShutdown  = hooks.StageBeforeShutdown
	StageAfterShutdown   = hooks.StageAfterShutdown
)

// HookFunc 是生命周期钩子（无 payload；hooks.Registry 的适配在转发层完成）。
type HookFunc func(ctx context.Context) error

// ServeFunc **阻塞运行**服务直至 ctx 取消或出错（与 http.Server.Serve 语义一致）。
// 监听器创建放 BeforeListen 段（fail-fast），则 AfterListen 触发时监听已就绪。
// 返回 nil 视为服务已正常退出（Run 进入关闭序列）。
type ServeFunc func(ctx context.Context) error

// ShutdownFunc 关闭服务（排空；收到带排空预算的 ctx）。
type ShutdownFunc func(ctx context.Context) error

// App 是生命周期驱动器：转发 hooks.Registry 注册，驱动 Run。
// 并发安全：On/SetStageTimeout 可在 Run 前并发调用；约定 Run 启动后冻结注册。
type App struct {
	stageTimeouts map[Stage]time.Duration
	reg           *hooks.Registry
	logger        *slog.Logger
}

// New 创建生命周期驱动器（logger 由调用方注入；nil 用 slog.Default）。
func New(logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}
	return &App{
		stageTimeouts: map[Stage]time.Duration{
			StageBeforeShutdown: 5 * time.Second, // 排空预算（§7.2）
			StageAfterShutdown:  time.Second,     // 独立预算（不复用排空剩余）
		},
		reg:    hooks.New(logger),
		logger: logger,
	}
}

// SetStageTimeout 覆盖阶段内单个钩子的默认超时。
func (a *App) SetStageTimeout(stage Stage, d time.Duration) {
	a.stageTimeouts[stage] = d
	a.reg.SetStageTimeout(stage, d)
}

// HookOption 定制单个钩子。
type HookOption = hooks.HookOption

// WithTimeout 覆盖该钩子的超时；d<=0 回退阶段默认——没有"不限时"语义。
var WithTimeout = hooks.WithTimeout

// On 注册生命周期钩子（同阶段 priority 降序 + 注册序；钩子内 panic 隔离）。
func (a *App) On(stage Stage, name string, priority int, fn HookFunc, opts ...hooks.HookOption) {
	if fn == nil {
		return
	}
	a.reg.On(stage, name, priority, func(hctx context.Context, _ any) error {
		return fn(hctx)
	}, opts...)
}

// Hooks 返回各阶段已注册钩子名（诊断数据面）。
func (a *App) Hooks() map[Stage][]string { return a.reg.Hooks() }

// Registry 返回统一钩子注册表（只读分发：transport 管线驱动 request 阶段用，
// TR-05；注册仍走 On/模块 Hooks，勿绕过）。
func (a *App) Registry() *hooks.Registry { return a.reg }

// Run 驱动完整生命周期：
//
//	BeforeBootstrap → AfterBootstrap → BeforeListen（fail-fast）
//	→ go serve() → AfterListen（log-only）→ 等 ctx 取消或 serve 退出
//	→ BeforeShutdown 排空（5s，单钩子失败不跳过其余）→ shutdown() → AfterShutdown
//
// 返回首个致命错误（fail-fast 段或 serve 失败）；排空段错误只聚合记录。
// 信号监听由调用方负责（cmd 接 signal.NotifyContext 到 ctx）。
func (a *App) Run(ctx context.Context, serve ServeFunc, shutdown ShutdownFunc) error {
	if serve == nil || shutdown == nil {
		return errors.New("bootstrap: serve and shutdown must not be nil")
	}

	for _, stage := range []Stage{StageBeforeBootstrap, StageAfterBootstrap, StageBeforeListen} {
		a.logStage(stage)
		if err := a.reg.RunStage(ctx, stage, nil); err != nil {
			return err
		}
	}

	// serve 在独立 goroutine 运行（阻塞契约）；panic 隔离转错误，不击穿进程
	serveDone := make(chan error, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				serveDone <- fmt.Errorf("serve panic: %v", p)
			}
		}()
		serveDone <- serve(ctx)
	}()

	a.logStage(StageAfterListen) // serve 已调度（监听就绪性由 BeforeListen 建 listener 保证）
	if err := a.reg.RunStage(ctx, StageAfterListen, nil); err != nil {
		a.logger.Warn("bootstrap: after_listen hooks failed (log-only, continue)", "err", err)
	}

	var serveErr error
	select {
	case serveErr = <-serveDone:
		if serveErr != nil && !errors.Is(serveErr, context.Canceled) {
			a.logger.Error("bootstrap: serve failed", "err", serveErr)
		}
	case <-ctx.Done():
		serveErr = ctx.Err()
	}

	// 排空：BeforeShutdown（5s 预算，单钩子失败不跳过其余）→ shutdown → AfterShutdown
	// 预算说明：§7.2 将 5s 系于 BeforeShutdown；**实际实现 shutdown() 与 BeforeShutdown 共享同一 5s ctx**（排空钩子耗预算后 transport 优雅排空拿过期 ctx 即失败——F9 登记：multi/慢排空部署前评估拆分独立预算），
	// 避免排空耗尽后关闭动作在过期 ctx 上全部失败。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.stageTimeouts[StageBeforeShutdown])
	defer cancel()
	if sdErr := a.reg.RunStage(shutdownCtx, StageBeforeShutdown, nil); sdErr != nil {
		a.logger.Warn("bootstrap: before_shutdown drain errors (continue closing)", "err", sdErr)
	}
	if err := shutdown(shutdownCtx); err != nil {
		a.logger.Warn("bootstrap: shutdown error", "err", err)
	}
	asCtx, asCancel := context.WithTimeout(context.Background(), a.stageTimeouts[StageAfterShutdown])
	defer asCancel()
	if err := a.reg.RunStage(asCtx, StageAfterShutdown, nil); err != nil {
		a.logger.Warn("bootstrap: after_shutdown hooks failed (log-only)", "err", err)
	}

	if serveErr != nil && !errors.Is(serveErr, context.Canceled) {
		return serveErr
	}
	return nil
}

func (a *App) logStage(stage Stage) {
	a.logger.Info("bootstrap: stage", "stage", string(stage))
}
