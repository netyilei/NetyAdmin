// Package configcenter 是动态配置模块（第 4 层）：sys_configs
// 键值 + 变更历史/回滚 + 热更广播（platform.config.changed）+ Watcher 消费面。
// 模块契约边界：module.go 只做装配注册（迁移/事件/端口 Expose），业务在
// application，存储在 adapters。首个热更消费者=log 级别动态开关（cmd 接线）。
package configcenter

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netyilei/netyadmin/internal/kernel/bus"
	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/module"
	"github.com/netyilei/netyadmin/internal/kernel/refresh"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/modules/configcenter/adapters"
	"github.com/netyilei/netyadmin/internal/modules/configcenter/api/events"
	"github.com/netyilei/netyadmin/internal/modules/configcenter/application"
)

//go:embed all:migrations
var migrationsFS embed.FS

// Migrations 返回模块迁移源（cmd 迁移命令登记用）。
func Migrations() fs.FS { return database.MigrationsOf(migrationsFS) }

// Module 实现 kernel/module 契约。
type Module struct {
	logger    *slog.Logger
	bus       *bus.Bus
	service   *application.Service
	watcher   *application.Watcher
	reconcile *refresh.Job // 周期对账兜底（Broadcast 域 at-most-once 消费侧）
}

// jobSnapshotReconcile 周期对账 Job 名（Manifest jobs 双源同字面）。
const jobSnapshotReconcile = "configcenter.snapshot.reconcile"

// New 构造（bus=变更广播源）。
func New(logger *slog.Logger, b *bus.Bus) *Module {
	return &Module{logger: logger, bus: b, reconcile: refresh.NewJob(jobSnapshotReconcile)}
}

// Name 实现 Module。
func (m *Module) Name() string { return "configcenter" }

// DependsOn 实现 Module（无业务模块依赖；DB 为 kernel 设施延迟注入）。
func (m *Module) DependsOn() []string { return nil }

// Init 纯装配：迁移源/事件目录/端口 Expose（禁 IO——AttachDB 起资源）。
func (m *Module) Init(_ context.Context, r *module.Registry) error {
	if err := r.Migrations.Register(m.Name(), Migrations()); err != nil {
		return fmt.Errorf("configcenter: register migrations: %w", err)
	}

	m.service = application.New(m.bus, m.logger)
	m.watcher = application.NewWatcher()
	if err := r.Services.Expose("configcenter.Provider", providerPort{m.service}); err != nil {
		return fmt.Errorf("configcenter: expose service: %w", err)
	}
	if err := r.Services.Expose("configcenter.Watcher", watcherPort{m.watcher}); err != nil {
		return fmt.Errorf("configcenter: expose watcher: %w", err)
	}

	// 事件目录运行时登记（与 Manifest events 声明一致；发布方管理面）
	r.Events.Register(m.Name(), module.EventPublish, []string{events.TopicConfigChanged})
	r.Events.Register(m.Name(), module.EventSubscribe, []string{events.TopicConfigChanged})
	// 周期对账兜底（销项）：Broadcast 域 at-most-once 的消费侧补偿
	// ——handler 退避重试覆盖瞬断；本 Job 兜住丢消息/重试穷尽的持久陈旧
	// （盲 Reload watcher 快照，5min 收敛上界）。
	if err := r.Jobs.Register(m.Name(), m.reconcile.Name(), m.reconcile); err != nil {
		return fmt.Errorf("configcenter: register reconcile job: %w", err)
	}
	return nil
}

// initialLoadTimeout 是模块初始装载限时（F3：DB 挂起 bootstrap 卡死
// 消除——30s 覆盖慢盘/慢网初始同步；与 rbac 同基线，超时走各自既有降级路径）。
const initialLoadTimeout = 30 * time.Second

// AttachDB 在 DB 连接后由装配调用：接线存储 + 订阅 Bus 热更 reload（幂等）。
// 初始装载失败降级运行（空快照 + repo 直读兜底 + Error 告警 + reload 可恢复——
// 与 rbac/ipac 同语义；配置面非判定面，不炸启动）。
func (m *Module) AttachDB(pool *database.Pool, startCtx context.Context) error {
	if pool == nil {
		return errors.New("configcenter: nil pool")
	}
	if m.service == nil {
		return errors.New("configcenter: service not initialized")
	}
	repo := adapters.NewRepo(pool)
	m.watcher.SetRepo(repo)
	// 初始装载限时（F3：原 context.Background() 无界——DB 挂起会
	// 卡死 bootstrap；超时后走既有降级路径（空快照 + repo 直读 + reload 可恢复），
	// 降级语义不变仅加时限）。
	loadCtx, cancel := context.WithTimeout(startCtx, initialLoadTimeout)
	defer cancel()
	if err := m.service.Attach(loadCtx, repo); err != nil {
		m.logger.Error("configcenter: initial attach failed, running on empty snapshot", "err", err)
	}
	if err := m.watcher.Reload(loadCtx); err != nil {
		m.logger.Error("configcenter: initial snapshot load failed, reads fall back to repo", "err", err)
	}
	if m.bus != nil {
		// 订阅广播 reload（幂等全量替换）；失败进程内退避重试（DB 瞬断当场
		// 救回），穷尽后 Error 告警并回错给 Bus 计数（陈旧由周期对账兜底）。
		if err := refresh.Subscribe(m.bus, events.TopicConfigChanged, "configcenter: reload",
			m.watcher.Reload, m.logger); err != nil {
			return err
		}
	}
	m.reconcile.SetReload(m.watcher.Reload) // 周期对账接线（AttachDB 后有效）
	return nil
}

// Service 访问器（诊断面用）。
func (m *Module) Service() *application.Service { return m.service }

// Watcher 访问器（cmd 接线消费回调用——log 级别等）。
func (m *Module) Watcher() *application.Watcher { return m.watcher }

// RegisterAdminRoutes 注册配置管理端点（面组装调用）。
// public（免鉴权）= /configs/public；authed = 列表/更新（权限码声明）。
func (m *Module) RegisterAdminRoutes(public, authed gin.IRouter,
	authorizer authz.Authorizer, onDeny func(*gin.Context, authn.Principal, string, string)) error {

	adapters.RegisterAdminRoutes(public, authed, adapters.Deps{
		Service:    m.service,
		Authorizer: authorizer,
		OnDeny:     onDeny,
	})
	return nil
}
