// Package rbac 是 RBAC 模块（RB-01）：角色/菜单/API/按钮数据 +
// admin 绑定 + Casbin 策略装载 + Bus 热更重载。判定引擎在 kernel/security/authz
// ，本模块是策略数据唯一归属（职责切分：引擎=kernel、数据=模块）。
// 模块契约边界：module.go 只做装配注册（迁移/事件/端口 Expose），业务在
// application，存储在 adapters。管理端 CRUD 端点随同批的。
package rbac

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
	"github.com/netyilei/netyadmin/internal/modules/rbac/adapters"
	"github.com/netyilei/netyadmin/internal/modules/rbac/api/events"
	"github.com/netyilei/netyadmin/internal/modules/rbac/application"
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
	reconcile *refresh.Job // 周期对账兜底（Broadcast 域 at-most-once 消费侧）
}

// jobPolicyReconcile 周期对账 Job 名（Manifest jobs 双源同字面）。
const jobPolicyReconcile = "rbac.policy.reconcile"

// New 构造（bus=策略变更广播源：订阅热更 reload， 管理面写入后发布）。
func New(logger *slog.Logger, b *bus.Bus) *Module {
	return &Module{logger: logger, bus: b, reconcile: refresh.NewJob(jobPolicyReconcile)}
}

// Name 实现 Module。
func (m *Module) Name() string { return "rbac" }

// DependsOn 实现 Module（无业务模块依赖；DB 为 kernel 设施延迟注入）。
func (m *Module) DependsOn() []string { return nil }

// Init 纯装配：迁移源/事件目录/端口 Expose（Casbin 构造即建——loader 未接线
// 时空策略全 deny，AttachDB 后 Reload 装入真实策略）。
func (m *Module) Init(_ context.Context, r *module.Registry) error {
	if err := r.Migrations.Register(m.Name(), Migrations()); err != nil {
		return fmt.Errorf("rbac: register migrations: %w", err)
	}

	svc, err := application.New(m.logger, m.bus)
	if err != nil {
		return fmt.Errorf("rbac: service: %w", err)
	}
	m.service = svc
	if err := r.Services.Expose("rbac.Authorizer", svc.Authorizer()); err != nil {
		return fmt.Errorf("rbac: expose authorizer: %w", err)
	}

	// 事件目录运行时登记（与 Manifest events 声明一致；发布方管理面）
	r.Events.Register(m.Name(), module.EventPublish, []string{events.TopicPolicyChanged})
	r.Events.Register(m.Name(), module.EventSubscribe, []string{events.TopicPolicyChanged})
	// 周期对账兜底（销项）：Broadcast 域 at-most-once 的消费侧补偿
	// ——handler 退避重试覆盖瞬断；本 Job 兜住丢消息/重试穷尽的持久陈旧
	// （盲 Reload Casbin 策略，5min 收敛上界）。
	if err := r.Jobs.Register(m.Name(), m.reconcile.Name(), m.reconcile); err != nil {
		return fmt.Errorf("rbac: register reconcile job: %w", err)
	}
	return nil
}

// initialLoadTimeout 是模块初始装载限时（F3：DB 挂起 bootstrap 卡死
// 消除——30s 覆盖慢盘/慢网初始同步；与 configcenter 同基线，超时走各自既有
// 降级路径）。
const initialLoadTimeout = 30 * time.Second

// AttachDB 在 DB 连接后由装配调用：装载策略 + 订阅 Bus 热更（幂等 reload；
// 初始装载失败降级运行——空策略全 deny + Error 告警 + reload 可恢复，
// **与 ipac 失败哲学分叉（F10）**：ipac 初始装载 fail-fast、rbac 降级 deny-all 继续 run——两安全面差异见各自 doc， 迁移命令化落地前规则表可能未建）。
func (m *Module) AttachDB(pool *database.Pool, startCtx context.Context) error {
	if pool == nil {
		return errors.New("rbac: nil pool")
	}
	if m.service == nil {
		return errors.New("rbac: service not initialized")
	}
	// 初始装载限时（F3：原 context.Background() 无界——DB 挂起会
	// 卡死 bootstrap；超时后走既有 deny-all 降级（Error 告警 + reload 可恢复），
	// 降级语义不变仅加时限）。
	loadCtx, cancel := context.WithTimeout(startCtx, initialLoadTimeout)
	defer cancel()
	if err := m.service.Attach(loadCtx, adapters.NewRepo(pool)); err != nil {
		m.logger.Error("rbac: initial policy load failed, running deny-all until reload", "err", err)
	}
	if m.bus != nil {
		// 订阅广播 reload（幂等全量替换）；失败进程内退避重试（DB 瞬断当场
		// 救回），穷尽后 Error 告警并回错给 Bus 计数（陈旧由周期对账兜底）。
		if err := refresh.Subscribe(m.bus, events.TopicPolicyChanged, "rbac: reload",
			m.service.Reload, m.logger); err != nil {
			return err
		}
	}
	m.reconcile.SetReload(m.service.Reload) // 周期对账接线（AttachDB 后有效）
	return nil
}

// Service 访问器（bootstrap 绑定/诊断面用）。
func (m *Module) Service() *application.Service { return m.service }

// RegisterAdminRoutes 注册 RBAC 管理端点（面组装调用）。
// authorizer/onDeny 由 cmd 装配注入（本模块即判定数据源，判定面仍经 kernel
// 接口注入——装配可替换/可测）。
func (m *Module) RegisterAdminRoutes(rg gin.IRouter,
	authorizer authz.Authorizer, onDeny func(*gin.Context, authn.Principal, string, string)) error {

	adapters.RegisterAdminRoutes(rg, adapters.Deps{
		Service:    m.service,
		Authorizer: authorizer,
		OnDeny:     onDeny,
	})
	return nil
}

// EnsureBootstrapBinding 首启绑定：bootstrap admin → super-admin 角色
// （security.md §1 首启管理员直连全权；幂等——已绑定跳过）。包级函数
// （cmd 迁移命令直接调——bootstrap 不依赖 Module 装配态，与 identity 同模式）。
func EnsureBootstrapBinding(ctx context.Context, pool *database.Pool, adminID string) (bool, error) {
	if pool == nil {
		return false, errors.New("rbac: nil pool")
	}
	svc, err := application.New(nil, nil)
	if err != nil {
		return false, err
	}
	if err := svc.Attach(ctx, adapters.NewRepo(pool)); err != nil {
		return false, err
	}
	return svc.EnsureBootstrapBinding(ctx, adminID)
}
