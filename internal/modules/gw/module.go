// Package gw 是网关治理+入口模块（自研领域网关）——
// governance 上下文（App/Scope/API 治理数据面 + 管理端点）与 edge 上下文
// （HMAC 签名验签面）同模块目录、两上下文。
// secret 加密面：AES-256-GCM（primitives；key=Secret 注入——config secrets）。
// 模块契约边界：module.go 只做装配注册，业务在 application，存储在 adapters。
package gw

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netyilei/netyadmin/internal/kernel/bus"
	"github.com/netyilei/netyadmin/internal/kernel/database"
	"log/slog"

	klog "github.com/netyilei/netyadmin/internal/kernel/log"
	"github.com/netyilei/netyadmin/internal/kernel/module"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/refresh"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/state/coordination"
	"github.com/netyilei/netyadmin/internal/modules/gw/adapters"
	"github.com/netyilei/netyadmin/internal/modules/gw/api/events"
	"github.com/netyilei/netyadmin/internal/modules/gw/application"
	gwdomain "github.com/netyilei/netyadmin/internal/modules/gw/domain"
)

//go:embed all:migrations
var migrationsFS embed.FS

// Migrations 返回模块迁移源（cmd 迁移命令登记用）。
func Migrations() fs.FS { return database.MigrationsOf(migrationsFS) }

// Config 是模块配置段（config.gw）——secret 加密钥（Secret 面，32 字节 base64）。
type Config struct {
	SecretKey string `yaml:"secret_key"` // env-only Secret 注入（base64 → 32 字节）
}

// Module 实现 kernel/module 契约。
type Module struct {
	logger     *slog.Logger
	service    *application.Service
	stats      *application.StatsService
	ipacEngine *application.IPEngine
	ipacMgr    *application.IPACManager
	ipPool     *database.Pool
	bus        *bus.Bus     // 规则/应用快照变更广播（DomainBroadcast——跨节点 reload）
	reconcile  *refresh.Job // 周期对账兜底（Broadcast 域 at-most-once 消费侧）
}

// initialReloadRetries/initialReloadDelay 初始装载重试参数（DB 瞬断救回窗；
// 线性退避 1s/2s，总窗 ~3s+装载耗时——不显著拖慢启动；IP 引擎与应用快照
// 两组装载共用同一策略）。
const (
	initialReloadRetries = 3
	initialReloadDelay   = 1 * time.Second
)

// jobSnapshotsReconcile 周期对账 Job 名（Manifest jobs 双源同字面）。
const jobSnapshotsReconcile = "gw.snapshots.reconcile"

// New 构造（secretKey=32 字节解码后密钥；nil/空=启动失败——secret 面 fail-closed；
// logMgr=统计流水/快照文件面（全文件化——nil=仅内存聚合，测试面））。
func New(secretKey []byte, logMgr *klog.Manager, b *bus.Bus) (*Module, error) {
	svc, err := application.New(secretKey)
	if err != nil {
		return nil, err
	}
	return &Module{logger: logMgr.Logger("gw", "app"), service: svc, stats: application.NewStatsService(logMgr),
		ipacEngine: application.NewIPEngine(), bus: b, reconcile: refresh.NewJob(jobSnapshotsReconcile)}, nil
}

// Stats 访问器（统计面——cmd dispatcher 记录+adminapi 端点消费）。
func (m *Module) Stats() *application.StatsService { return m.stats }

// Name 实现 Module。
func (m *Module) Name() string { return "gw" }

// DependsOn 实现 Module（无业务模块依赖）。
func (m *Module) DependsOn() []string { return nil }

// Init 纯装配：迁移源/端口 Expose/事件目录登记（禁 IO）。
func (m *Module) Init(_ context.Context, r *module.Registry) error {
	if err := r.Migrations.Register(m.Name(), Migrations()); err != nil {
		return fmt.Errorf("gw: register migrations: %w", err)
	}
	if err := r.Services.Expose("gw.Governance", governancePort{m.service}); err != nil {
		return fmt.Errorf("gw: expose service: %w", err)
	}
	if err := r.Services.Expose("gw.Stats", statsPort{m.stats}); err != nil {
		return fmt.Errorf("gw: expose stats: %w", err)
	}
	// 事件目录登记（自发布自订阅——IP 规则/应用判定快照变更跨节点 reload 收敛，
	// 根修：此前仅发布无订阅=其他节点快照永久陈旧）
	r.Events.Register(m.Name(), module.EventPublish, []string{events.TopicRulesChanged, events.TopicAppsChanged})
	r.Events.Register(m.Name(), module.EventSubscribe, []string{events.TopicRulesChanged, events.TopicAppsChanged})
	// 周期对账兜底（销项）：Broadcast 域 at-most-once 的消费侧补偿
	// ——handler 退避重试覆盖瞬断；本 Job 兜住丢消息/重试穷尽的持久陈旧
	// （盲 Reload 两面，5min 收敛上界）。
	if err := r.Jobs.Register(m.Name(), m.reconcile.Name(), m.reconcile); err != nil {
		return fmt.Errorf("gw: register reconcile job: %w", err)
	}
	return nil
}

// AttachDB 在 DB 连接后由装配调用（与 identity/rbac 同惯例）。
func (m *Module) AttachDB(pool *database.Pool, startCtx context.Context) error {
	if pool == nil {
		return errors.New("gw: nil pool")
	}
	m.ipPool = pool
	m.ipacMgr = application.NewIPACManager(m.ipacEngine, pool, m.logger, func(ctx context.Context) {
		// IP 规则变更广播（DomainBroadcast——跨节点 reload；对照 configcenter/rbac 惯例）
		if m.bus != nil {
			// 发布失败必须可见（静默忽略=其他节点停留在旧快照且无迹可查）
			if err := m.bus.Publish(ctx, bus.Message{Topic: events.TopicRulesChanged}, bus.WithDomain(bus.DomainBroadcast)); err != nil {
				m.logger.Warn("gw: rules-changed publish failed (other nodes stay stale until next event)", "err", err)
			}
		}
	})
	// 初始装载带短退避重试（DB 瞬断场景此前一次失败即留 nil 快照=
	// edge 全拒直至下次变更事件；规则数据坏场景重试无效，由写时校验拦截+
	// error 告警——nil 快照 fail-closed 兜底语义不变）
	var reloadErr error
	for attempt := 1; attempt <= initialReloadRetries; attempt++ {
		if reloadErr = m.ipacEngine.Reload(startCtx, pool); reloadErr == nil {
			break
		}
		if attempt < initialReloadRetries {
			time.Sleep(initialReloadDelay * time.Duration(attempt)) // 1s/2s 线性退避
		}
	}
	if reloadErr != nil {
		m.logger.Error("gw: ip rules initial reload failed (edge fail-closed until next change event)",
			"err", reloadErr, "attempts", initialReloadRetries)
	}
	if m.bus != nil {
		// 订阅广播 reload（幂等——本节点变更已在 changed() 直Reload；此处承接
		// 其他节点广播，构成跨节点快照收敛闭环）；失败进程内退避重试（DB 瞬断
		// 当场救回），穷尽后 Error 告警并回错给 Bus 计数（陈旧由周期对账兜底）。
		if err := refresh.Subscribe(m.bus, events.TopicRulesChanged, "gw: ip rules reload",
			func(ctx context.Context) error { return m.ipacEngine.Reload(ctx, pool) }, m.logger); err != nil {
			return err
		}
	}
	if err := m.service.Attach(adapters.NewRepo(pool)); err != nil {
		return err
	}
	// edge 判定快照装载面（销项）：pool=装载源；本节点写路径
	// commit 后 changed() 直 Reload，onChanged 发布 DomainBroadcast 供其他节点
	// 订阅 Reload（与 IP 引擎同构——受控装载面非通用 L1 缓存）。
	m.service.AttachSnapshot(pool, m.logger, func(ctx context.Context) {
		if m.bus == nil {
			return
		}
		if err := m.bus.Publish(ctx, bus.Message{Topic: events.TopicAppsChanged},
			bus.WithDomain(bus.DomainBroadcast)); err != nil {
			m.logger.Warn("gw: apps-changed publish failed (other nodes stay stale until next event)", "err", err)
		}
	})
	var appReloadErr error
	for attempt := 1; attempt <= initialReloadRetries; attempt++ {
		if appReloadErr = m.service.ReloadSnapshot(startCtx); appReloadErr == nil {
			break
		}
		if attempt < initialReloadRetries {
			time.Sleep(initialReloadDelay * time.Duration(attempt))
		}
	}
	if appReloadErr != nil {
		m.logger.Error("gw: app snapshot initial reload failed (edge fail-closed until next change event)",
			"err", appReloadErr, "attempts", initialReloadRetries)
	}
	if m.bus != nil {
		// 订阅广播 reload（幂等——本节点变更已在 changed() 直 Reload；此处承接
		// 其他节点广播，构成跨节点快照收敛闭环）；失败进程内退避重试，穷尽后
		// Error 告警并回错（陈旧由周期对账兜底）。
		if err := refresh.Subscribe(m.bus, events.TopicAppsChanged, "gw: app snapshot reload",
			m.service.ReloadSnapshot, m.logger); err != nil {
			return err
		}
	}
	m.service.AttachIPAC(m.ipacMgr)          // 组集绑定写路径的存在性校验面
	m.reconcile.SetReload(m.reconcileReload) // 周期对账接线（AttachDB 后有效）
	return nil
}

// reconcileReload 周期对账盲 Reload 两面（IP 规则引擎 + edge 应用快照）：
// 全量替换幂等、无副作用、无版本依赖（三面均无统一订阅侧版本号——选型 a）。
// 任一面失败返回聚合错误：scheduler 短退避重试，穷尽留待下一周期；旧快照
// 继续服务（降级非中断）。
func (m *Module) reconcileReload(ctx context.Context) error {
	if m.ipPool == nil {
		return nil // 未接 DB（单测装配）无对账面
	}
	return errors.Join(m.ipacEngine.Reload(ctx, m.ipPool), m.service.ReloadSnapshot(ctx))
}

// Service 访问器。
func (m *Module) Service() *application.Service { return m.service }

// AccessControlMiddleware 面级访问控制中间件（——appKey 绑定内置面应用；
// IP 组集判定+应用 quota 滑窗；deps 组装集中——coord/repo 由 cmd 注入。
// 应用行数据源=模块装载型快照（销项：原 poolFn+30s 单机缓存已删——
// 与 edge 同源同失效链，快照未装载/未命中 → 503 fail-closed）。
func (m *Module) AccessControlMiddleware(coord *coordination.Coordination,
	appKey string) gin.HandlerFunc {

	return application.AccessControlMiddleware(application.AccessControlDeps{
		AppByKey: func(ctx context.Context, key string) (*gwdomain.App, error) {
			a, err := m.service.AppByKey(ctx, key)
			if err != nil {
				return nil, err
			}
			return &a, nil
		},
		Engine: m.ipacEngine,
		Quota: func(ctx context.Context, app *gwdomain.App, ip string) (bool, string) {
			if app.Quota.Rate <= 0 {
				return true, "" // 0=不限（G3 契约语义——显式无限，免 Redis 往返）
			}
			if coord == nil {
				return false, "unavailable" // 无 Redis=限流面不可用——fail-closed（contracts gateway_app on_unavailable 决策）
			}
			rate := app.Quota.Rate
			if err := coord.AllowByPolicy(ctx, "gateway_app", app.ID, rate, time.Second); err != nil {
				if errors.Is(err, coordination.ErrDenied) {
					return false, "quota exceeded"
				}
				return false, "unavailable" // 不可用 fail-closed
			}
			return true, ""
		},
	}, appKey)
}

// EnsureGatewayApps 包级自举入口（cmd migrate 段调用——对照 EnsureBootstrapBinding 惯例；
// pool 直调，secret 随机 AES 加密入库不外显）。
func EnsureGatewayApps(ctx context.Context, pool *database.Pool, aesKey []byte) ([]string, error) {
	return application.EnsureGatewayApps(ctx, pool, aesKey)
}

// IPACChecker 是 edge IP 判定最小面（adapters 契约——实现=本模块引擎适配器）。
type IPACChecker = adapters.IPACChecker

// engineChecker 将 IPEngine.MatchGroups 适配 edge IPACChecker 面
// （ctx 未用——快照判定零 IO；引擎未装载/未接线=全拒 fail-closed）。
type engineChecker struct{ engine *application.IPEngine }

func (e engineChecker) CheckGroups(_ context.Context, ip netip.Addr, groupIDs []string) gwdomain.IPDecision {
	if e.engine == nil {
		return gwdomain.IPDecision{Allowed: false, Reason: "fail_closed"}
	}
	return e.engine.MatchGroups(ip, groupIDs)
}

// IPACChecker 返回 edge IP 判定面（cmd 组合根 EdgePipeline 消费——
// 快照持有者=AttachDB 装载的本模块引擎）。
func (m *Module) IPACChecker() IPACChecker { return engineChecker{engine: m.ipacEngine} }

// EdgePipeline 返回网关 edge 面中间件两段（cmd 组合根分派器消费——
// audit-m6 安-F9 根修：单一 catch-all 收进 cmd 分派器，WS/平台端点与业务 API
// 在同一分派器内路由，组内不再被 gin 通配冲突阻塞）。
// edgeAuth=①-⑦+主体注入（HMAC/app/IPAC/nonce/quota）；apiGate=⑧ scope→API
// 白名单（仅业务 API 分派分支——平台端点 /ws/* 不参与 gw_apis 登记）。
func (m *Module) EdgePipeline(nonce *coordination.Coordination, ipac IPACChecker,
	metrics *observability.Metrics) (edgeAuth, apiGate gin.HandlerFunc, err error) {

	verifier, err := authn.NewAppHMACVerifier(authn.HMACOptions{
		Secret: func(ctx context.Context, appKey string) ([]byte, error) {
			return m.service.AppSecret(ctx, appKey) // 装载快照读——零 DB 往返
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("gw: hmac verifier: %w", err)
	}
	edgeAuth = adapters.EdgeAuthMiddleware(adapters.EdgeDeps{
		Service: m.service, Nonce: nonce, IPAC: ipac, Metrics: metrics,
	}, verifier)
	apiGate = adapters.APIGate(adapters.EdgeDeps{Service: m.service})
	return edgeAuth, apiGate, nil
}

// RegisterAdminRoutes 注册治理端点（/； 面组装调用）。
func (m *Module) RegisterAdminRoutes(authed gin.IRouter,
	authorizer authz.Authorizer, onDeny func(*gin.Context, authn.Principal, string, string)) error {

	adapters.RegisterAdminRoutes(authed, adapters.Deps{
		Service:    m.service,
		Stats:      m.stats,
		Authorizer: authorizer,
		OnDeny:     onDeny,
		IPAC:       m.ipacMgr,
		Engine:     m.ipacEngine,
		Pool:       m.ipPool,
	})
	return nil
}
