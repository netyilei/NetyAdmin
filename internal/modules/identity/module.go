// Package identity 是管理员身份模块（ID-01；identity 收缩纯 admin——30）：
// 账户/登录/token 轮换/改密/吊销。模块契约边界：module.go 只做装配注册
// （迁移/端口 Expose），业务在 application，存储在 adapters。
// HTTP 端点随（adminapi 面）；登录审计随端点层接线（产生方
// =接入面，有 request_id/IP/UA—— code-audit 后裁决，模块层不跨模块直写 audit）。
// C 端用户体系不属本模块（users 演示模块 US-01 自注册 clientapi）。
package identity

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/netyilei/netyadmin/internal/kernel/bus"
	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/module"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/security/captcha"
	"github.com/netyilei/netyadmin/internal/kernel/security/keys"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/kernel/state/session"
	"github.com/netyilei/netyadmin/internal/modules/identity/adapters"
	"github.com/netyilei/netyadmin/internal/modules/identity/api/events"
	identityapplication "github.com/netyilei/netyadmin/internal/modules/identity/application"
	"github.com/netyilei/netyadmin/internal/modules/identity/domain"
)

//go:embed all:migrations
var migrationsFS embed.FS

// Migrations 返回模块迁移源（cmd 迁移命令登记用；assembly 同款单一入口纪律）。
func Migrations() fs.FS { return database.MigrationsOf(migrationsFS) }

// Module 实现 kernel/module 契约。
type Module struct {
	logger    *slog.Logger
	redis     string            // session 平面 Redis 地址（装配注入）
	namespace string            // session 键命名空间 <root>:<env>:sess（装配注入——铁律 6 环境段）
	topology  session.Topology  // 部署拓扑（session 降级语义按拓扑裁决——contracts）
	bus       *bus.Bus          // 登录审计事件发布源（可 nil）
	keys      *keys.SigningKeys // JWT 密钥集（装配注入；AccessVerifier 构造用）
	store     *session.Store    // AttachDB 后保存（AccessVerifier 吊销面）
	repo      domain.Repo       // AttachDB 后保存（TokenVersionSource）
	service   *identityapplication.Service
	metrics   *observability.Metrics                    // 共享指标面（na_session_* 落统一 registry；nil=不计数）
	job       *identityapplication.RevocationCleanupJob // 吊销行清理（JOB-01；AttachDB 接线 repo）
}

// New 构造（signer=JWT 签发（keys 集成）；eventBus=登录审计事件发布源（可 nil）；
// topology=session 降级语义输入；sessionNS=session 键命名空间 <root>:<env>:sess；
// metrics=共享指标面（同一实例——自建第二套会导致 na_session_* 不可抓取））。
func New(logger *slog.Logger, redisAddr string, topology session.Topology, sessionNS string,
	signer identityapplication.Signer, eventBus *bus.Bus, signingKeys *keys.SigningKeys,
	metrics *observability.Metrics) *Module {
	return &Module{
		logger:    logger,
		redis:     redisAddr,
		namespace: sessionNS,
		topology:  topology,
		bus:       eventBus,
		keys:      signingKeys,
		metrics:   metrics,
		service: identityapplication.New(identityapplication.Config{}, logger.With("module", "identity"),
			primitives.SystemClock{}, signer, eventBus),
	}
}

// Name 实现 Module。
func (m *Module) Name() string { return "identity" }

// DependsOn 实现 Module（无业务模块依赖；DB/session 均为 kernel 设施延迟注入）。
func (m *Module) DependsOn() []string { return nil }

// Init 纯装配：迁移源/端口 Expose（禁 IO——AttachDB 起资源）。
func (m *Module) Init(_ context.Context, r *module.Registry) error {
	if err := r.Migrations.Register(m.Name(), Migrations()); err != nil {
		return fmt.Errorf("identity: register migrations: %w", err)
	}
	if err := r.Services.Expose("identity.Service", identityPort{m.service}); err != nil {
		return fmt.Errorf("identity: expose service: %w", err)
	}
	// 事件发布声明（单一事实源——运行时 Publish 必须有 Manifest 对偶，
	// 防「幽灵事件链」：TopicLoginAudit 消费方=audit 模块 subscriber；
	// TopicTokenRevoked 当前无订阅方= 事件化留痕，消费方接入即用）。
	r.Events.Register(m.Name(), module.EventPublish, []string{events.TopicLoginAudit, events.TopicTokenRevoked})
	// 吊销行清理 Job（JOB-01 移交项；audit RetentionJob 同款——Init 注册 +
	// AttachDB 接线；双源校验要求 Manifest jobs 声明对偶）。
	m.job = identityapplication.NewRevocationCleanupJob()
	if err := r.Jobs.Register(m.Name(), m.job.Name(), m.job); err != nil {
		return fmt.Errorf("identity: register job: %w", err)
	}
	return nil
}

// AttachDB 在 DB 连接后由装配调用（与 audit/ipac 同惯例）：构造 repo +
// session.Store（DB 回退回调接 repo；降级按装配注入的拓扑裁决——
// single: db_fallback_or_fail / multi: fail_closed，contracts session 段）+
// 关闭就绪门。
func (m *Module) AttachDB(pool *database.Pool, _ context.Context) error {
	if pool == nil {
		return errors.New("identity: nil pool")
	}
	repo := adapters.NewRepo(pool)
	store := session.New(session.NewRedisClient(m.redis), m.topology, m.namespace,
		repo.IsRevokedFallback, repo.GetSessionFallback, m.metrics)
	m.store = store
	m.repo = repo
	m.service.Attach(repo, store)
	m.job.SetRepo(repo)
	return nil
}

// AccessVerifier 组装 admin access token 验证器（验证链：公钥验签 +
// 吊销面 + TokenVersion）。cmd 组装 adminapi 鉴权中间件用。
func (m *Module) AccessVerifier() (*authn.JWTVerifier, error) {
	if m.store == nil || m.repo == nil {
		return nil, errors.New("identity: access verifier requires AttachDB")
	}
	if m.keys == nil {
		return nil, errors.New("identity: access verifier requires signing keys")
	}
	return authn.NewJWTVerifier(authn.PrincipalAdmin, authn.BaseOptions{
		PublicKeys:        m.keys.Verifiers(),
		CheckRevocation:   true,
		Revocation:        m.store,
		CheckTokenVersion: true,
		VersionSource: func(ctx context.Context, adminID string) (int64, error) {
			admin, err := m.repo.FindByID(ctx, adminID)
			if err != nil {
				return 0, err
			}
			return admin.TokenVersion, nil
		},
	})
}

// RegisterAdminRoutes 注册 admin auth/管理端点（/； 面组装调用）。
// public/authed 由 cmd 分组（authn 中间件组级挂载——rbac/audit 端点同链）：
// public=login/refresh（限流注入）；authed=自身面+管理面（authn → RequirePerm →
// authz）。登录审计经 Bus 事件（记录唯一归属）；authorizer/onDeny 由
// cmd 注入（rbac 判定面 + deny 审计回调 + 登录验证码面——模块间只经 kernel
// 接口解耦；captcha 由 cmd 按 config.redis.addr 构造传入）。
func (m *Module) RegisterAdminRoutes(public, authed gin.IRouter,
	authorizer authz.Authorizer, onDeny func(*gin.Context, authn.Principal, string, string),
	rateLimit gin.HandlerFunc, cap *captcha.Captcha) error {

	// verifier/authn 面由 cmd 构造（AccessVerifier + AuthMiddleware 组级挂载，
	// rbac/audit 端点同链）——module 侧只交端点（装配形态见 main.go adminapi-routes）。
	deps := adapters.Deps{Service: m.service, RateLimit: rateLimit, Captcha: cap}
	adapters.RegisterPublicRoutes(public, deps)
	adapters.RegisterAuthedRoutes(authed, deps, authorizer, onDeny)
	return nil
}

// opsBcryptCost 是 cmd 迁移族命令（bootstrap/重置密码）的哈希成本（与
// application 服务默认 12 同口径——G1-08 具名）。
const opsBcryptCost = 12

// EnsureBootstrapAdmin 首启管理员（security.md §1：admin+首启随机密码，
// must_change_password 强制首登改密）。幂等：已有账户即跳过，但回带既有
// bootstrap 账户 ID（super-admin 绑定面随 migrate up 自愈历史空绑——2026-11-21
// 真机复盘：部分清库（identity 存留）时曾以空 adminID 绑定出脏行，authz
// g(r.sub,p.sub) 永不命中=全 deny）。
// 返回明文密码仅用于 stdout 一次性打印（不出 cmd 范围）。包级函数（cmd 迁移
// 命令直接调——bootstrap 不依赖 Module 装配态）。密码生成与重置面共用
// application.GeneratePassword（G1-06 收拢）。
func EnsureBootstrapAdmin(ctx context.Context, pool *database.Pool) (password, adminID string, created bool, err error) {
	repo := adapters.NewRepo(pool)
	n, err := repo.CountAdmins(ctx)
	if err != nil {
		return "", "", false, err
	}
	if n > 0 {
		a, ferr := repo.FindByUsername(ctx, "admin")
		if ferr != nil {
			if errors.Is(ferr, domain.ErrNotFound) {
				return "", "", false, nil // bootstrap 账户缺席（多管理员系统删 admin 合法）——绑定面按空 ID 跳过
			}
			return "", "", false, ferr // 真实 DB 故障不得伪装成「账户缺席」——静默跳过绑定=authz 全 deny 事故复发面
		}
		return "", a.ID, false, nil
	}
	password, err = identityapplication.GeneratePassword()
	if err != nil {
		return "", "", false, fmt.Errorf("identity: bootstrap password: %w", err)
	}
	hash, err := primitives.HashPassword(password, opsBcryptCost)
	if err != nil {
		return "", "", false, fmt.Errorf("identity: bootstrap hash: %w", err)
	}
	now := primitives.SystemClock{}.Now().UTC()
	admin := domain.Admin{
		ID:                 primitives.NewULID(primitives.SystemClock{}),
		Username:           "admin",
		PasswordHash:       hash,
		Name:               "admin", // 种子数据姓名
		Status:             domain.StatusActive,
		MustChangePassword: true,
		TokenVersion:       1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := repo.InsertAdmin(ctx, admin); err != nil {
		return "", "", false, err
	}
	return password, admin.ID, true, nil
}

// ResetAdminPassword 运维重置密码（-mode=reset-password 子命令；迁移锁内调用
// ——F11 清偿：bootstrap 密码遗失的锁死出口）。按用户名定位
// 账户 → 随机密码 + TokenVersion++（全会话作废）+ must_change_password 置位。
// 明文仅经返回值交付（stdout 一次性打印，不出 cmd 范围）。包级函数与
// EnsureBootstrapAdmin 同款（migrate 模式不构造 bus/signer，不走 Service 装配态）。
func ResetAdminPassword(ctx context.Context, pool *database.Pool, username string) (string, error) {
	if username == "" {
		return "", errors.New("identity: reset password: empty username")
	}
	repo := adapters.NewRepo(pool)
	admin, err := repo.FindByUsername(ctx, username)
	if err != nil {
		return "", err // ErrNotFound 透传（cmd 层转可读错误）
	}
	password, err := identityapplication.GeneratePassword()
	if err != nil {
		return "", fmt.Errorf("identity: reset password: %w", err)
	}
	hash, err := primitives.HashPassword(password, opsBcryptCost)
	if err != nil {
		return "", fmt.Errorf("identity: reset hash: %w", err)
	}
	if err := repo.UpdatePassword(ctx, admin.ID, hash, true, true); err != nil {
		return "", err
	}
	return password, nil
}

// Service 访问器（诊断面用）。
func (m *Module) Service() *identityapplication.Service { return m.service }
