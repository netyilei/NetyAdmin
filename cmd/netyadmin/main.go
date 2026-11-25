// @title NetyAdmin Admin API
// @version 1.0.0
// @description 管理端 API 契约（adminapi 面；swag init --v3.1 生成入库 contracts/openapi）
// @BasePath /admin/v1
// @schemes http https
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 管理端/客户端 Bearer token：`Authorization: Bearer <access_token>`（登录端点换取；refresh 用 refresh_token）
// 网关面 HMAC 四头（X-NA-App-Id / X-NA-Timestamp / X-NA-Nonce / X-NA-Signature）由
// split-openapi.mjs 注入 GatewayHMAC scheme（swag v2 单 definition 限制——生成物仅保一块）。
//
// main.go：netyadmin 入口——全组件接线演示：
// config → log → observability → state 平面（coordination/cache/session）
// → database → bootstrap → transport（/health/live|ready、/metrics）。
// 模块装配清单单一入口见 assembly.go（拆卸=删目录+删装配行）。
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netyilei/netyadmin/internal/kernel/bootstrap"
	"github.com/netyilei/netyadmin/internal/kernel/bus"
	"github.com/netyilei/netyadmin/internal/kernel/config"
	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/log"
	"github.com/netyilei/netyadmin/internal/kernel/module"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/scheduler"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/security/captcha"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/kernel/security/ratelimit"
	"github.com/netyilei/netyadmin/internal/kernel/state/cache"
	"github.com/netyilei/netyadmin/internal/kernel/state/coordination"
	"github.com/netyilei/netyadmin/internal/kernel/state/health"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/kernel/transport/rpc"
	"github.com/netyilei/netyadmin/internal/modules/audit/adapters"
	"github.com/netyilei/netyadmin/internal/modules/audit/application"
	gwapplication "github.com/netyilei/netyadmin/internal/modules/gw/application"
	identityadapters "github.com/netyilei/netyadmin/internal/modules/identity/adapters"
	portrpc "github.com/netyilei/netyadmin/internal/modules/rbac/api/rpc"
	"github.com/netyilei/netyadmin/internal/modules/realtime/session"
	usersadapters "github.com/netyilei/netyadmin/internal/modules/users/adapters"
	usersapp "github.com/netyilei/netyadmin/internal/modules/users/application"
)

// 版本信息由构建期 ldflags 注入：-X main.version=... -X main.commit=...
var (
	version = "0.0.0-dev"
	commit  = "none"
)

// AfterBootstrap 钩子优先级（降序执行）：db-connect(10) 先于 bus-start(5)——
// bus Start 的 Outbox relay 依赖 AttachDB 的池。
const (
	hookPrioDBConnect       = 10
	hookPrioBusStart        = 5
	hookPrioSchedulerStart  = 4
	hookPrioTracingShutdown = -5 // 排空段末位：其余清理先行，span 最后 flush

	// admin_login 限流参数（security.md §5 公开 API 必须限流；防爆破口径
	// 10 次/分钟/IP——V1 同量级；策略名已登记 contracts/state-policies.yaml）。
	loginRateLimit  = 10
	loginRateWindow = time.Minute
	// api_rule 基线限流（RPC 路由组——内部调用受限流约束；基线值保守）。
	apiRuleRateLimit  = 600
	apiRuleRateWindow = time.Minute
)

func main() {
	showVersion := flag.Bool("version", false, "打印版本信息后退出")
	mode := flag.String("mode", "serve", "运行模式：serve（默认）| migrate（迁移子命令，配 -migrate-action）| reset-password（运维重置密码，配 -username）")
	migrateAction := flag.String("migrate-action", "up", "迁移动作（-mode=migrate 时）：up|status")
	username := flag.String("username", "", "目标管理员用户名（-mode=reset-password 必填）")
	configPath := flag.String("config", "config.yaml", "配置文件路径（不存在则用默认值+env）")
	addr := flag.String("addr", "", "HTTP 监听地址（覆盖 config.server.http_port）")
	flag.Parse()

	if *showVersion {
		fmt.Printf("netyadmin %s (commit %s)\n", version, commit)
		return
	}
	if *mode == "migrate" {
		os.Exit(runMigrate(*migrateAction, *configPath))
	}
	if *mode == "reset-password" {
		os.Exit(runResetPassword(*username, *configPath))
	}
	os.Exit(run(*configPath, *addr))
}

// gatewayGroupFor 组装网关业务组（/gateway/v1）：组级中间件仅业务链，不挂
// api_rule——限流=gateway_app 应用维度（edge ⑦ 步 quota，NAT 误伤裁决）。
// 独立函数=装配断言锚点（wiring_test 断言本组 handler 链零组级基线限流）。
func gatewayGroupFor(srv *transport.Server) gin.IRouter {
	return srv.Group("/gateway/v1")
}

// rpcHostGroupFor 组装内部 RPC 宿主组：保留 api_rule 基线（内部调用
// 受限流约束；调用方=登记服务，无 NAT 误伤面）。
func rpcHostGroupFor(srv *transport.Server, rlRPC gin.HandlerFunc) gin.IRouter {
	return srv.Group("", rlRPC)
}

// run 组装并运行；返回退出码。
func run(configPath, addrFlag string) int {
	// ① 配置分层（默认值 → YAML → env → Secrets）
	cfg, warnings, err := config.Load(config.LoadOptions{Path: configPath})
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "config warning:", w)
	}

	// ①.5 JWT 密钥（dev 首启自动生成；prod 缺文件 fail-closed）
	signingKeys, err := ensureJWTKeys(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keys: %v\n", err)
		return 1
	}

	// ② 日志（logs/<date>/<module>_<type>.log）
	debugTTL, err := time.ParseDuration(cfg.Log.DebugTTL) // 空串=0=不启用
	if cfg.Log.DebugTTL == "" {
		debugTTL = 0
		err = nil
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "log: bad debug_ttl %q: %v\n", cfg.Log.DebugTTL, err)
		return 1
	}
	logMgr, err := log.New(logOptions(cfg, debugTTL)) // config → log.Options 唯一映射点
	if err != nil {
		fmt.Fprintf(os.Stderr, "log: %v\n", err)
		return 1
	}
	defer logMgr.Close()
	logger := logMgr.Logger("kernel", "app")

	// 观测页阈值（config.observability.alerts 覆盖 + 默认集）：启动早期合并并对
	// 未知键告警——与部署角色无关（core 角色节点同样看得到配置错误）。
	obsThresholds, unknownObsAlerts := observability.MergeThresholds(cfg.Observability.Alerts)
	if len(unknownObsAlerts) > 0 {
		logger.Warn("observability.alerts: 未知阈值键（已忽略——键见 docs/rules/observability.md）",
			"keys", unknownObsAlerts)
	}

	// ③ 可观测（指标/诊断；追踪接线——未启用时零开销）
	metrics := observability.NewMetrics(cfg.Deployment.NodeID)
	diagnostics := observability.NewDiagnostics()
	tracingInst, err := observability.SetupTracing(observability.TracingOptions{
		Enabled:     cfg.Observability.Tracing.Enabled,
		Endpoint:    cfg.Observability.Tracing.Endpoint,
		SampleRatio: cfg.Observability.Tracing.SampleRatio,
		ServiceName: cfg.Observability.Tracing.ServiceName,
	})
	if err != nil {
		logger.Error("tracing init failed", "err", err)
		return 1
	}

	// ④ bootstrap（6 段生命周期）
	app := bootstrap.New(logger)

	// ⑤ 状态平面（coordination / cache / session）
	var coord *coordination.Coordination
	var redisOK bool
	var redisStartupFail bool
	if addr := redisAddr(cfg); addr != "" {
		if cli := coordination.NewRedisClient(addr); pingRedis(cli) == nil {
			coord = coordination.New(cli, topologyOf(cfg), coordNamespace(cfg), metrics)
			redisOK = true
		} else {
			logger.Warn("redis 不可达：coordination 原语按策略降级")
			redisStartupFail = true
		}
	}

	// ⑤.1 Cache 平面运行面（实装——2026-11-21：机制面完整交付，
	// 消费面（业务 entity）由下游模块经 Register 接入）：
	//   - L1 常开（contracts cache.l1.always_on，无开关）；
	//   - L2 与 coordination 同源判定（Redis 配置+探活一次，防双探漂移）：
	//     coord!=nil 接 Redis；coord=nil（single 无 Redis）= single_optional 语义
	//     （L1+loader 直读 DB，contracts cache.l2）——multi 缺 Redis 由 db-connect
	//     段 F4 fail-fast 兜底（multi_required，contracts cache.l2.multi_required）；
	//   - 面名 kernel.Cache 经 Registry Services Expose——下游模块
	//     ResolveAs[*cache.Store]("kernel.Cache") 消费并注册自己的 entity。
	var cacheL2 cache.L2Client
	if coord != nil {
		cacheL2 = cache.NewL2RedisClient(redisAddr(cfg))
	}
	// PII L2 加密钥（contracts cache.pii.encrypt_l2；env-only NETYADMIN_CACHE_L2_KEY）：
	// 配置了则解码注入（非法 base64/长度=cache.New 启动失败）；未配置=pii 实体注册
	// 失败（fail-closed，非 pii 实体不受影响——业务 entity 由下游 Register 时校验）。
	var cacheOpts []cache.Option
	if cfg.Cache.L2EncryptKey.IsSet() {
		raw, derr := base64.StdEncoding.DecodeString(cfg.Cache.L2EncryptKey.Value())
		if derr != nil {
			logger.Error("cache L2 encrypt key base64", "err", derr)
			return 1
		}
		cacheOpts = append(cacheOpts, cache.WithL2EncryptKey(raw))
	}
	cacheStore, err := cache.New(cacheNamespace(cfg), cacheL2, nil, metrics, cacheOpts...)
	if err != nil {
		logger.Error("cache init failed", "err", err)
		return 1
	}

	// ⑥ 数据库（连接池；迁移执行随模块迁移）
	var dbPool *database.Pool
	healthAgg := health.NewAggregator()
	// Redis 探针：配置并连通 → 真探活；multi 且启动失败 → 恒败探针（ready 如实
	// 反映，多机摘流量）；single 不配置 Redis → 不注册（L2 可选，contracts）
	if coord != nil {
		healthAgg.Register("redis", func(ctx context.Context) error { return pingRedisCoord(ctx, coord) })
	} else if redisStartupFail && cfg.Deployment.Topology == "multi" {
		healthAgg.Register("redis", func(context.Context) error {
			return errors.New("redis unreachable at startup")
		})
	}

	// ⑥.5 事件总线：multi 必须有 Redis（AGENTS §3，构造即拒绝）；
	// single 不配 Redis 时本地回退（broadcast/cluster 降级单机语义）。
	// 配置了 Redis 但不可达 → Start 段 fail-fast（对齐启动段错误策略）。
	busOpts := bus.Options{
		NodeID:    cfg.Deployment.NodeID, // 全集群唯一（consumer 标识 + Headers.source）
		Logger:    logMgr.Logger("bus", "app"),
		Metrics:   metrics,
		RedisAddr: redisAddr(cfg),
		KeyRoot:   cfg.Redis.Prefix,
		Env:       cfg.Server.Env,
	}
	if busOpts.RedisAddr == "" && cfg.Deployment.Topology != "multi" {
		busOpts.AllowLocalFallback = true
	}
	busInst, err := bus.New(busOpts)
	if err != nil {
		logger.Error("bus init failed", "err", err)
		return 1
	}

	// ⑥.6 调度器：cluster 域选主用 coordination（nil=自任
	// leader，单机降级）；运行记录依赖 DB（AttachDB 延迟注入）。内核自带任务
	// 随模块 Job 注册（无消费者时不预置）。
	schedInst, err := scheduler.New(scheduler.Options{
		NodeID:  cfg.Deployment.NodeID, // 全集群唯一（shard 分片/运行记录归属）
		Logger:  logMgr.Logger("scheduler", "app"),
		Metrics: metrics,
		Clock:   primitives.SystemClock{},
		Coord:   coord,
		Tracing: tracingInst,
	})
	if err != nil {
		logger.Error("scheduler init failed", "err", err)
		return 1
	}
	// auditSvc 声明前置（限流审计回调闭包引用；db-connect 段赋值——晚绑定）
	var auditSvc *application.Service
	var adminWS, gatewayWS, clientWS *session.Surface // 接入面（关停排空用）
	var dbCluster *database.Cluster                   //（replica 可选；nil=无副本单池）

	// ⑦.5 公开端点限流（核对项；security.md §5 公开 API 必须限流）。
	// admin_login 策略已登记 contracts/state-policies.yaml（fail-open）；
	// coord=nil（无 Redis）时直通——登记的幂等 nil 直通同语义面。
	var rlLogin, rlUsersLogin gin.HandlerFunc
	var rlEngine *ratelimit.Engine // rpc-host 钩子复用（nil=无 Redis 直通——限流中间件自身旁路）
	if coord != nil {
		rlEngine = ratelimit.NewEngine(coord)
		// api_rule：常规 API 基线限流（contracts 已登记；RPC 路由组消费——
		// 未注册=fail-closed 500，评审冒烟实证）
		if err := rlEngine.Register(ratelimit.Policy{
			Name:      "api_rule",
			Dimension: ratelimit.DimensionIP,
			Limit:     apiRuleRateLimit,
			Window:    apiRuleRateWindow,
		}); err != nil {
			logger.Error("ratelimit api_rule register failed", "err", err)
			return 1
		}
		if err := rlEngine.Register(ratelimit.Policy{
			Name:      "admin_login",
			Dimension: ratelimit.DimensionIP,
			Limit:     loginRateLimit,
			Window:    loginRateWindow,
		}); err != nil {
			logger.Error("ratelimit policy register failed", "err", err)
			return 1
		}
		rlLogin = transport.RateLimitMiddleware(rlEngine, "admin_login", nil, rateLimitAuditCallback(func() *application.Service { return auditSvc }))
		// users_login：client 公开登录/注册面（US-01——G2 公开 API 必须限流）
		if err := rlEngine.Register(ratelimit.Policy{
			Name:      "users_login",
			Dimension: ratelimit.DimensionIP,
			Limit:     loginRateLimit,
			Window:    loginRateWindow,
		}); err != nil {
			logger.Error("ratelimit users_login register failed", "err", err)
			return 1
		}
		rlUsersLogin = transport.RateLimitMiddleware(rlEngine, "users_login", nil,
			rateLimitAuditCallback(func() *application.Service { return auditSvc }))
	}

	// ⑧ 模块装配（单一入口；拆卸=删目录+删装配行）
	registry := module.NewRegistry(cfg, app)
	// cache 面注册（先于模块登记——下游 Register 期即可消费：
	// ResolveAs[*cache.Store]("kernel.Cache")）
	if err := registry.Services.Expose("kernel.Cache", cacheStore); err != nil {
		logger.Error("cache face expose failed", "err", err)
		return 1
	}
	// 指标面注册（下游业务指标唯一合规入口：ResolveAs[*observability.Metrics]
	// 后 Register 自有 na_<module>_* collector；与 /metrics、观测页同一 registry）。
	if err := registry.Services.Expose("kernel.Metrics", metrics); err != nil {
		logger.Error("metrics face expose failed", "err", err)
		return 1
	}
	// gw secret key（App secret AES-256——Secret env-only，base64 32 字节）
	gwSecretKey, err := decodeGWKey(cfg)
	if err != nil {
		logger.Error("gw secret key invalid", "err", err)
		return 1
	}
	identityMod, rbacMod, auditMod, configMod, gwMod, realtimeMod, messageMod, usersMod, storageMod, err := registerModules(logger, logMgr, busInst, registry,
		cfg, signingKeys, redisAddr(cfg), sessionTopology(cfg), gwSecretKey,
		cfg.Redis.Prefix+":"+cfg.Server.Env, coord, metrics)
	if err != nil {
		logger.Error("module assembly failed", "err", err)
		return 1
	}
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer buildCancel()
	if err := registry.Build(buildCtx); err != nil {
		logger.Error("module build failed", "err", err)
		return 1
	}
	// 菜单-模块一致性（rbac_menus.module）：启用模块集合=装配事实源（Registry
	// 已 Build 完成）——RouteTree 过滤「所属模块未启用」的菜单，避免
	// users/message/storage 关闭时残留死菜单（默认 users.enabled=false 即现）。
	enabledModules := make(map[string]bool, len(registry.Started()))
	for _, name := range registry.Started() {
		enabledModules[name] = true
	}
	rbacMod.Service().SetModuleFilter(func(name string) bool { return enabledModules[name] })

	// ⑧.1 模块 Job 登记（首个消费方 retention；spec 强类型断言防漂移）
	for _, entry := range registry.Jobs.Entries() {
		job, ok := entry.Spec.(scheduler.Job)
		if !ok {
			logger.Error("module job spec not scheduler.Job", "module", entry.Module, "name", entry.Name)
			return 1
		}
		if err := schedInst.Register(job); err != nil {
			logger.Error("module job register failed", "name", entry.Name, "err", err)
			return 1
		}
	}

	// 模块 DB 延迟接线面（db-connect 段遍历模块实例；与 bus/scheduler AttachDB 同惯例）
	type dbAttachable interface {
		AttachDB(pool *database.Pool, startCtx context.Context) error
	}
	attachModules := func(pool *database.Pool, startCtx context.Context) error {
		for _, name := range registry.Started() {
			m, ok := registry.Module(name)
			if !ok {
				continue
			}
			if a, ok := m.(dbAttachable); ok {
				if err := a.AttachDB(pool, startCtx); err != nil {
					return fmt.Errorf("module %s attach db: %w", name, err)
				}
				logger.Info("module db attached", "module", name)
			}
		}
		if v, err := module.ResolveAs[*application.Service](registry.Services, "audit.Service"); err == nil {
			auditSvc = v // 关停排空用（audit 唯一持有者）
		} else {
			logger.Warn("audit service not exposed", "err", err)
		}
		return nil
	}

	// bus-start 优先级 < db-connect：先 AttachDB（Outbox/DLQ 依赖）再 Start（起 relay）
	app.On(bootstrap.StageAfterBootstrap, "bus-start", hookPrioBusStart, func(ctx context.Context) error {
		return busInst.Start(ctx)
	})
	// scheduler-start 优先级 < bus-start：运行记录恢复依赖 AttachDB
	app.On(bootstrap.StageAfterBootstrap, "scheduler-start", hookPrioSchedulerStart, func(ctx context.Context) error {
		return schedInst.Start(ctx)
	})
	// tracing-shutdown 优先级最低（排空段 flush 未导出 span；log-only 不阻断关停）
	app.On(bootstrap.StageBeforeShutdown, "tracing-shutdown", hookPrioTracingShutdown, func(ctx context.Context) error {
		if err := tracingInst.Shutdown(ctx); err != nil {
			logger.Warn("tracing flush", "err", err)
		}
		return nil
	})

	// AfterBootstrap：DB 连接（BeforeBootstrap 段做配置校验，DB 在此段连接）
	app.On(bootstrap.StageAfterBootstrap, "db-connect", hookPrioDBConnect, func(ctx context.Context) error {
		pool, err := database.New(ctx, database.BuildDSN(cfg.Database), metrics,
			database.WithMaxConns(cfg.Database.MaxConns))
		if err != nil {
			return err
		}
		dbPool = pool
		// 读写分离（可选副本——BuildDSN 不含 replica_dsn，replica 走独立
		// DSN；无副本=读也走主。模块数据源（每模块独立 Cluster）随拆库消费方）
		if cfg.Database.ReplicaDSN != "" {
			replica, err := database.NewReplica(ctx, cfg.Database.ReplicaDSN)
			if err != nil {
				return err
			}
			dbCluster = &database.Cluster{Read: replica, Write: pool}
			// 副本探针（状-E）：启动期仍 fail-fast（声明了副本却不可用=配置错误，
			// 拒绝带病启动）；运行期副本故障经独立 check 名 pg_replica 可见——
			// 降级型注册：不翻 /health/ready（写面仍可用，不摘全流量；readiness
			// 只有 ok/unready 两态），明细 "degraded:" 供监控辨识。
			healthAgg.RegisterDegradable("pg_replica", func(ctx context.Context) error {
				return replica.Ping(ctx)
			})
			logger.Info("database replica attached")
		}
		// PG advisory 选主分支（multi 拓扑注入——Redis 挂时 leader 原语
		// 转会话级 advisory lock；single 不注入=分支禁用）
		if cfg.Deployment.Topology == "multi" {
			// F4 fail-fast：multi 拓扑 Redis 必需（coord=nil 时此处原为 nil panic
			// 栈）——选主/幂等/限流原语依赖 coordination，静默降级会让 leader 语义
			// 退化；拒绝启动并给出可操作指引。
			if coord == nil {
				return errors.New("multi topology requires redis: check config.redis.addr and connectivity")
			}
			coord.AttachPG(pool)
		}
		if err := busInst.AttachDB(pool); err != nil {
			return err
		}
		if err := schedInst.AttachDB(pool); err != nil {
			return err
		}
		if err := attachModules(pool, ctx); err != nil {
			return err
		}
		healthAgg.Register("pg", func(ctx context.Context) error { return pool.Ping(ctx) })
		logger.Info("database connected", "name", cfg.Database.Name)
		return nil
	})

	// ⑦ 传输面（HTTP 管线 + 健康端点 + /metrics；TR-05 hooks； span）
	transportSrv, err := transport.New(
		// config → transport.Options 唯一映射点（含 TrustedProxies 反代信任面；
		// idem=nil（无 Redis）时幂等键中间件直通；access 分流访问流水）
		transportOptions(cfg, addrOf(cfg.Server.HTTPPort, addrFlag), app.Registry(),
			tracingInst, coord, logMgr.Logger("transport", "access")),
		logMgr.Logger("transport", "app"),
		metrics,
		func(ctx context.Context) (bool, map[string]string) { return healthAgg.Ready(ctx) },
	)
	if err != nil {
		logger.Error("transport init failed", "err", err)
		return 1
	}

	// 接入面 role（部署形态；声明前置——⑦.8/⑧.5/⑧.6 共用）
	role := cfg.Deployment.Role

	// ⑦.8 ops 面（OPS-01：pprof 开关）+ clientapi 门（echo + RouteRegistrar）
	if err := registerOpsRoutes(transportSrv, cfg); err != nil {
		logger.Error("ops routes failed", "err", err)
		return 1
	}
	if mountClientGate(role) { // client 门仅 all（判定单点——role_test 对偶）
		// client WS surface（ticket 签发 fail-closed——User JWT Provider 随
		// US-01 接线激活； 移交同构）
		wsClient, err := registerClientAPI(transportSrv, registry, realtimeMod, logMgr, metrics,
			gwMod.AccessControlMiddleware(coord, gwapplication.AppKeyFrontendApp))
		if err != nil {
			logger.Error("clientapi routes failed", "err", err)
			return 1
		}
		if wsClient != nil {
			clientWS = wsClient
		}
		// 激活（US-01 User JWT Provider 就位）：users 自注册 client 路由
		// （RouteRegistrar 组——clientapi 零 import），authed 组挂 ticket 签发面。
		// client 业务面（users/message/storage）只依赖 users 鉴权面，与 realtime
		// 开关解耦（realtime 禁用时 MountTicket 跳过——WS ticket 面缺席 fail-closed，
		// 业务路由照常注册；冒烟教训：此前被 wsClient!=nil 连带误杀）。
		// US-01 开关门=wireClientModuleRoutes 单点（usersMod==nil → 零注册）。
		registrar, rerr := module.ResolveAs[gin.IRouter](registry.Services, "clientapi.RouteRegistrar")
		if rerr != nil {
			logger.Error("route registrar missing", "err", rerr)
			return 1
		}
		if rerr := wireClientModuleRoutes(registrar, usersMod, messageMod, storageMod,
			wsClient, rlUsersLogin); rerr != nil {
			logger.Error("client module routes failed", "err", rerr)
			return 1
		}
	}
	// configcenter 热更消费（首个消费者=log 模块/类型级别；键契约见 wireConfigHotReload）
	wireConfigHotReload(configMod.Watcher(), logMgr)
	// SMTP 运行期配置消费（组 message；admin「邮件设置」写入→广播→各节点重建驱动；
	// 启动初始装载同样走本回调——config.yaml 为回退值）
	wireMessageSMTPConfig(configMod.Watcher(), messageMod, cfg, logMgr)
	// taskcenter 暂停态消费（F2 集群一致——job 注册已完成、DB 初始装载未跑，
	// 顺序即契约；键契约=组 taskcenter 见 wireTaskcenterPause）
	wireTaskcenterPause(configMod.Watcher(), schedInst, logMgr)

	// ⑧.55 接入面 role 过滤（部署形态——判定在 mountHTTPGates）
	httpGates := mountHTTPGates(role)
	// api_rule 基线限流中间件（rpc-host 组消费——单实例构造）。
	// 网关业务面不挂 api_rule：限流=gateway_app 应用维度滑窗（edge ⑦ 步）；
	// IP 维度基线在 NAT 下误伤多调用方（2026-11-22 压测裁决——50QPS 实测
	// 71% 429，单 IP 天花板≈10QPS 低于合法调用 SLA），关注点分离：
	// api_rule=常规 API 基线、网关面=应用维度配额。
	var rlRPC gin.HandlerFunc
	if rlEngine != nil {
		rlRPC = transport.RateLimitMiddleware(rlEngine, "api_rule", nil, nil)
	}

	// ⑧.5 adminapi 门组装（+）：链由 cmd 组合根持有——
	// 组级操作审计（产生方=接入面）；public/authed 分组（authed 组级
	// authn——rbac/audit 端点同链同组，公开端点不进该组）；RBAC 判定面
	//（rbac Service Authorizer）+ deny 审计回调（audit Service）经 kernel
	// 接口注入（模块间零 import）。
	app.On(bootstrap.StageAfterBootstrap, "adminapi-routes", 9, func(ctx context.Context) error {
		if !httpGates {
			logger.Info("adminapi gate skipped (role)", "role", role) // core 角色：无 HTTP 管理面
			return nil
		}
		// 访问流水=文件（mwAccessLog→transport_access.log——2026-11-19不进 DB）；
		// 组级仅挂操作审计（管理端变更——G2 审计合规面保留 DB）。
		// admin 面不挂应用级访问控制（2026-11-21 用户：admin 端 API/WS
		// 默认开放，授权=JWT+RBAC——应用限制面仅 client 面）
		adminGroup := transportSrv.Group("/admin/v1", operationAuditMiddleware(auditSvc))
		verifier, err := identityMod.AccessVerifier()
		if err != nil {
			return err
		}
		publicGroup := adminGroup.Group("/auth") // 公开：login/refresh（组内限流）
		// authed 组级：authn → 幂等键占位（组级挂载——L48 根修方案 A
		// ：未认证请求不得造键；公开端点 login/refresh 有独立
		// 限流+登录审计面兜底，不占幂等键）。
		authedGroup := adminGroup.Group("", identityadapters.AuthMiddleware(verifier), transportSrv.Idempotency())
		publicCfgGroup := adminGroup.Group("") // 公开：/configs/public（免鉴权读面）
		// rbac.Authorizer 端口绑定（/：config.binding 段——base_urls
		// 空=local 同进程直调；非空=remote 经内部 RPC 轮询 LB；§13.3 同接口）
		authorizer, err := bindAuthorizer(registry.Services, cfg.Binding,
			func(baseURLs []string) (authz.Authorizer, error) {
				client, err := rpc.NewClient(rpc.ClientOptions{
					Signer: signingKeys.Signer(), ServiceID: cfg.Deployment.ServiceName,
					Endpoints: baseURLs, Metrics: metrics,
				})
				if err != nil {
					return nil, err
				}
				return portrpc.NewRemoteAuthorizer(client), nil
			})
		if err != nil {
			return err
		}
		onDeny := onDenyAudit(auditSvc)
		// 登录验证码（补强——2026-11-21 用户；存储按 coord 可用性
		// 二选一：Redis=多节点共享一致（GETDEL 原子消费）；coord=nil（未配/
		// 探活失败，single）=进程内降级——降级语义登记 contracts/state-policies
		// captcha 段；multi 由 db-connect 段 fail-fast 拒启，运行期 Redis 故障
		// =校验失败拒登录不降级）
		loginCaptcha := captcha.New(captcha.Options{
			RedisAddr: cfg.Redis.Addr,
			Namespace: captchaNamespace(cfg),
			Memory:    coord == nil,
		})
		if coord == nil {
			// 降级可见（G2 日志：静默降级=运行期不可观测）：single 拓扑无 Redis
			// 时验证码答案落进程内，重启丢失/不跨节点——启动期显式告警。
			logger.Warn("login captcha: memory store fallback (redis unavailable, single topology only)")
		}
		if err := identityMod.RegisterAdminRoutes(publicGroup, authedGroup, authorizer, onDeny, rlLogin, loginCaptcha); err != nil {
			return err
		}
		if err := rbacMod.RegisterAdminRoutes(authedGroup, authorizer, onDeny); err != nil {
			return err
		}
		if err := auditMod.RegisterAdminRoutes(authedGroup, authorizer, onDeny); err != nil {
			return err
		}
		if err := configMod.RegisterAdminRoutes(publicCfgGroup, authedGroup, authorizer, onDeny); err != nil {
			return err
		}
		if err := gwMod.RegisterAdminRoutes(authedGroup, authorizer, onDeny); err != nil {
			return err
		}
		if messageMod != nil {
			if err := messageMod.RegisterAdminRoutes(authedGroup, authorizer, onDeny); err != nil {
				return err
			}
			// SMTP 运行期配置（admin「邮件设置」：configcenter 存储+热更；
			// 密码 env-only；message 模块禁用时整组缺席）
			RegisterSMTPConfigRoutes(authedGroup, SMTPDeps{
				Config: cfg, Configs: configMod.Service(), Message: messageMod,
			}, authorizer, onDeny)
		}
		if storageMod != nil {
			if err := storageMod.RegisterAdminRoutes(authedGroup, authorizer, onDeny); err != nil {
				return err
			}
		}
		RegisterTaskcenterRoutes(authedGroup, schedInst, configMod.Service(), authorizer, onDeny) // （ccSvc=F2 暂停态外置）
		RegisterBusDLQRoutes(authedGroup, busInst, authorizer, onDeny)                            //  死信重放（权限码复用 admin.taskcenter.operate）
		// 观测面（指标概览/浏览器；数据源=进程内共享 registry——与 /metrics 同实例；
		// 阈值=config.observability.alerts 覆盖 + observability.md §6 默认集）。
		RegisterObservabilityRoutes(authedGroup, ObservabilityDeps{
			Metrics:    metrics,
			NodeID:     cfg.Deployment.NodeID,
			Thresholds: obsThresholds,
			State: func() string {
				return string(health.ResolveMode(cfg.Deployment.Topology, cfg.State.Mode,
					redisAddr(cfg) != "", redisOK))
			},
		}, authorizer, onDeny)
		// static/SPA（Geeker-Admin 产物—— 唯一剩余收尾）：NoRoute
		// 分流=非 API 前缀走静态文件/index.html fallback（前端路由 history 模式）；
		// 产物路径=部署物相邻 admin/dist（缺失=纯 API 形态不挂载）。API 前缀未
		// 命中路由由下方显式 404 信封返回（非 gin 默认 404——统一信封由 transport
		// 管线渲染）。
		staticDir := "admin/dist"
		if st, serr := os.Stat(staticDir); serr == nil && st.IsDir() {
			transportSrv.Engine().NoRoute(func(c *gin.Context) {
				p := c.Request.URL.Path
				// API 面（显式前缀或非 GET 方法）一律 404 统一信封——SPA HTML
				// 不得吞 API 语义（联测：proxy 剥前缀误命中时曾返回 HTML 200）
				apiPrefixes := []string{"/admin/", "/client/", "/gateway/", "/internal/", "/v1/", "/geeker/", "/health", "/metrics"} // /geeker/=Geeker 前端旧封装残留前缀（audit B5）
				if c.Request.Method != http.MethodGet {
					transport.FailOf(c, errorx.New(errorx.CodeNotFound))
					return
				}
				for _, pre := range apiPrefixes {
					if strings.HasPrefix(p, pre) {
						transport.FailOf(c, errorx.New(errorx.CodeNotFound))
						return
					}
				}
				f := filepath.Join(staticDir, filepath.Clean("/"+p))
				if st, ferr := os.Stat(f); ferr == nil && !st.IsDir() {
					c.File(f)
					return
				}
				c.File(filepath.Join(staticDir, "index.html"))
			})
			logger.Info("admin static spa mounted", "dir", staticDir)
		}
		if usersMod != nil {
			// US-01 验收词「adminapi 经 Registry Resolve 管理 users 数据」——
			// 管理服务经 Port 消费（跨模块协作示范），非模块实例直调。
			svc, rerr := module.ResolveAs[*usersapp.Service](registry.Services, "users.Service")
			if rerr != nil {
				return rerr
			}
			usersadapters.RegisterAdminRoutes(authedGroup, usersadapters.AdminDeps{
				Service: svc, Authorizer: authorizer, OnDeny: onDeny,
			})
		}
		// Admin WS（机制面首个消费方）——ticket 签发挂 authed 组
		//（JWT 后）；upgrade 挂公开组（ticket=一次性凭证，绑定身份经 Take 还原）；
		// 频道 ACL=admin 权限码 ws:channel:<name>+私有频道仅本人； hooks=
		// 连接级审计（audit svc 直调——组合根惯例）。realtime disabled=表面缺席
		// （零开销——复评 A1 语义）。
		if realtimeMod.Cfg().Enabled && realtimeMod.Tickets() != nil {
			wsAdmin, err := realtimeMod.Surface(session.SurfaceOptions{
				Name:           "admin",
				OriginPatterns: cfg.Realtime.AdminOrigins, // §1 Origin 白名单（前端接入前空）
				Auth:           adminChannelAuthorizer{authz: authorizer},
				Hooks:          wsAuditHooks{log: logMgr},
				Metrics:        metrics,
				Logger:         logger,
				RequireHTTPS:   cfg.Realtime.RequireHTTPS, // 默认 false；prod 由 config.normalize 强制 true
			})
			if err != nil {
				return err
			}
			wsAdmin.Start(context.WithoutCancel(ctx)) // 钩子 ctx 返回即 cancel——会话挂常驻 ctx
			wsAdmin.MountTicket(authedGroup)
			wsAdmin.MountUpgrade(adminGroup)
			adminWS = wsAdmin
		} else {
			logger.Info("admin ws surface skipped (realtime disabled)")
		}
		// Gateway WS（/gateway/v1——edge HMAC 管线；nonce/IPAC 注入；
		// 安-F9 根修：单一分派器承载 WS/平台端点/业务 API）
		// 限流面=gateway_app 应用维度（edge ⑦ 步 quota），不挂 api_rule（NAT 误伤裁决）
		gatewayGroup := gatewayGroupFor(transportSrv)
		edgeAuth, apiGate, err := gwMod.EdgePipeline(coord, gwMod.IPACChecker(), metrics)
		if err != nil {
			return err
		}
		var gwWS *session.Surface
		if realtimeMod.Cfg().Enabled && realtimeMod.Tickets() != nil {
			w, err := realtimeMod.Surface(session.SurfaceOptions{
				Name:         "gateway",
				Auth:         gwChannelAuthorizer{},
				Hooks:        wsAuditHooks{log: logMgr},
				Metrics:      metrics,
				Logger:       logger,
				RequireHTTPS: cfg.Realtime.RequireHTTPS, // 同 admin 面（prod 强制 true）
			})
			if err != nil {
				return err
			}
			w.Start(context.WithoutCancel(ctx))
			gwWS = w
			gatewayWS = w
		}
		gatewayGroup.Any("/*path", gatewayDispatcher(edgeAuth, apiGate, gwWS, gwMod.Stats(),
			newGatewayForwarder(addrOf(cfg.Server.HTTPPort, addrFlag), cfg.Server.HTTPPort)))
		return nil
	})

	// ⑧.7 诊断面独立挂载（审计修：内部拓扑必须过 authn）——role
	// 无关（core 角色保留运维观测面；verifier 本进程 identity 构造）。
	app.On(bootstrap.StageAfterBootstrap, "diagnostics-routes", 8, func(ctx context.Context) error {
		verifier, err := identityMod.AccessVerifier()
		if err != nil {
			return err
		}
		diagGroup := transportSrv.Group("/admin/v1/system", identityadapters.AuthMiddleware(verifier))
		diagGroup.GET("/diagnostics", func(c *gin.Context) {
			transport.OK(c, diagnostics.Names())
		})
		diagGroup.GET("/diagnostics/:name", func(c *gin.Context) {
			snap := diagnostics.Snapshot(c.Request.Context())
			v, ok := snap[c.Param("name")]
			if !ok {
				transport.FailOf(c, fmt.Errorf("diagnostic %q not registered", c.Param("name")))
				return
			}
			transport.OK(c, v)
		})
		return nil
	})
	// ⑧.6 内部 RPC 宿主（TR-02 机制面宿主化——/internal/v1/rpc 同 engine
	// 路径隔离，独立端口随部署形态；接收方白名单 fail-closed——
	// config.rpc.allowed_callers 显式登记调用方服务；na_rpc_* 指标接线）。
	app.On(bootstrap.StageAfterBootstrap, "rpc-host", 7, func(ctx context.Context) error {
		serviceVerifier, err := authn.NewServiceVerifier(authn.ServiceOptions{
			BaseOptions: authn.BaseOptions{PublicKeys: signingKeys.Verifiers()},
		})
		if err != nil {
			return err
		}
		allow := rpc.NewAllowList(cfg.RPC.AllowedCallers...)
		// role×allowed_callers 启动对账（R3 轻量固化——告警不阻断；
		// 调用方服务名需授权面时应登记同 code rbac 角色）
		if len(cfg.RPC.AllowedCallers) > 0 {
			if err := reconcileAllowedCallerRoles(ctx, rbacMod, cfg.RPC.AllowedCallers, logger); err != nil {
				logger.Warn("rpc allowed_callers 对账失败（不阻断启动）", "err", err)
			}
		}
		rpcSrv, err := rpc.NewServer(rpc.ServerOptions{
			Verifier: serviceVerifier, Allow: allow, Metrics: metrics,
			OnAccess: onAccessAudit(func() *application.Service { return auditSvc }), // AccessRecord 产生方=transport RPC
		})
		if err != nil {
			return err
		}
		// 模块自注册 RPC 方法（清单与 assembly.go 模块清单对齐——新模块落地同步登记）
		if err := rbacMod.RegisterRPC(rpcSrv); err != nil {
			return err
		}
		// 内部调用受限流约束（api_rule 策略已登记 contracts——ip 维度）
		rpcSrv.Mount(rpcHostGroupFor(transportSrv, rlRPC))
		logger.Info("rpc host mounted", "allowed_callers", len(cfg.RPC.AllowedCallers))
		return nil
	})
	app.On(bootstrap.StageAfterBootstrap, "audit-login-subscribe", 8, func(ctx context.Context) error {
		return adapters.SubscribeLoginAudit(busInst, auditMod.Service())
	})

	// ⑨ 诊断数据面（diagnostics；七类补齐——observability.md §5）
	diagnostics.Register("modules", func(context.Context) any { return registry.Started() })
	diagnostics.Register("bindings", func(context.Context) any { return registry.Services.Snapshot() })
	diagnostics.Register("hooks", func(context.Context) any { return app.Hooks() })
	diagnostics.Register("bus", func(context.Context) any { return busInst.Snapshot() })
	diagnostics.Register("events", func(context.Context) any { return registry.EventsSnapshot() })
	diagnostics.Register("scheduler", func(context.Context) any { return schedInst.Catalog() })
	diagnostics.Register("config", func(context.Context) any { return cfg.Diagnostics() })
	diagnostics.Register("config_dynamic", func(context.Context) any {
		w := configMod.Watcher()
		return map[string]any{
			"version": configMod.Service().Version(),
			"loaded":  w.Loaded(),
			"keys":    len(w.Snapshot()), // 来源=configcenter 快照（变更时间见 config_history）
		}
	})
	diagnostics.Register("realtime", func(context.Context) any {
		chans, subs := realtimeMod.Hub().ChannelStats()
		return map[string]any{"conns": realtimeMod.Hub().Count(), "channels": chans, "subscriptions": subs}
	})
	diagnostics.Register("database", func(context.Context) any {
		return map[string]any{"replica": dbCluster != nil && dbCluster.Read != nil} // 读写分离面
	})
	diagnostics.Register("state", func(context.Context) any {
		mode := health.ResolveMode(cfg.Deployment.Topology, cfg.State.Mode, redisAddr(cfg) != "", redisOK)
		return map[string]any{
			"mode":     string(mode),
			"describe": health.Describe(mode),
			"topology": cfg.Deployment.Topology,
			"redis":    map[string]any{"configured": redisAddr(cfg) != "", "reachable": redisOK},
			"coordination_prefix": func() string {
				if coord != nil {
					return coordNamespace(cfg)
				}
				return ""
			}(),
		}
	})

	logger.Info("netyadmin starting", "env", cfg.Server.Env, "http_port", addrOf(cfg.Server.HTTPPort, addrFlag))

	// ⑩ 运行（bootstrap 驱动 6 段生命周期；信号 → ctx 由 signal.NotifyContext 接入）
	err = app.Run(signalContext(), func(ctx context.Context) error {
		return transportSrv.ListenAndServe()
	}, func(ctx context.Context) error {
		if err := transportSrv.Shutdown(ctx); err != nil {
			return err
		}
		if err := busInst.Close(); err != nil {
			logger.Warn("bus close", "err", err)
		}
		if err := schedInst.Close(); err != nil {
			logger.Warn("scheduler close", "err", err)
		}
		if err := cacheStore.Close(); err != nil {
			logger.Warn("cache close", "err", err)
		}
		/// 连接排空（1001 Going Away；transport Shutdown 之后客户端已断；
		// surface.Stop 等 wg——WriterLoop 排空队列后退出）
		if adminWS != nil {
			adminWS.Stop()
		}
		if gatewayWS != nil {
			gatewayWS.Stop()
		}
		if clientWS != nil {
			clientWS.Stop()
		}
		gwMod.Stats().Close() //  终桶快照落文件
		realtimeMod.CloseAll(ctx)
		if auditSvc != nil {
			if err := auditSvc.Close(ctx); err != nil {
				logger.Warn("audit flush close", "err", err)
			}
		}
		if dbPool != nil {
			dbPool.Close()
		}
		return logMgr.Close()
	})
	if err != nil {
		logger.Error("netyadmin exited with error", "err", err)
		return 1
	}
	logger.Info("netyadmin stopped")
	return 0
}
