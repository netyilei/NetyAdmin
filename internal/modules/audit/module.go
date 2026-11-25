// Package audit 是审计模块：审计统一记录模型 + Sink 异步批量 +
// operation_logs DB 落库 + 保留清理。模块契约边界：module.go 只做装配注册
// （迁移/Job/配置/端口 Expose），业务在 application，存储在 adapters。
// 查询/导出面随；access/security 记录产生方（transport/ 中间件）
// 接线随各自里程碑（本模块交付记录唯一归属的存储/保留面）。
package audit

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/module"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/audit/adapters"
	"github.com/netyilei/netyadmin/internal/modules/audit/application"
	idapievents "github.com/netyilei/netyadmin/internal/modules/identity/api/events"
)

//go:embed all:migrations
var migrationsFS embed.FS

// Migrations 返回模块迁移源（cmd 迁移命令登记用；assembly 同款单一入口纪律）。
func Migrations() fs.FS { return database.MigrationsOf(migrationsFS) }

// Config 是模块配置段（config.audit）。
type Config struct {
	RetentionDays int `yaml:"retention_days"` // ≤0 取默认 180（application 兜底）
}

// Module 实现 kernel/module 契约。
type Module struct {
	logger  *slog.Logger
	cfg     Config
	service *application.Service
	query   *application.QueryService
	job     *application.RetentionJob
}

// New 构造（logger 由装配注入）。
func New(logger *slog.Logger) *Module { return &Module{logger: logger} }

// Name 实现 Module。
func (m *Module) Name() string { return "audit" }

// DependsOn 实现 Module（无业务模块依赖；DB 为 kernel 设施，AttachDB 延迟注入）。
func (m *Module) DependsOn() []string { return nil }

// Init 纯装配：迁移源 / retention Job / 配置绑定 / 端口 Expose（禁 IO/goroutine——
// pool 延迟注入，flush loop 由 Attach 启动）。
func (m *Module) Init(_ context.Context, r *module.Registry) error {
	if r.Config != nil && r.Config.Exists("audit") {
		if err := r.Config.Bind("audit", &m.cfg); err != nil {
			return fmt.Errorf("audit: bind config: %w", err)
		}
	}

	if err := r.Migrations.Register(m.Name(), Migrations()); err != nil {
		return fmt.Errorf("audit: register migrations: %w", err)
	}

	m.service = application.New(application.Config{RetentionDays: m.cfg.RetentionDays},
		m.logger.With("module", "audit"), nil)
	if err := r.Services.Expose("audit.Service", m.service); err != nil {
		return fmt.Errorf("audit: expose service: %w", err)
	}
	// 跨模块消费面：业务模块写审计走 api.Recorder（不暴露 application 类型）。
	if err := r.Services.Expose("audit.Recorder", recorderPort{m.service}); err != nil {
		return fmt.Errorf("audit: expose recorder: %w", err)
	}

	m.job = application.NewRetentionJob(m.cfg.RetentionDays)
	m.job.SetClock(primitives.SystemClock{})
	if err := r.Jobs.Register(m.Name(), m.job.Name(), m.job); err != nil {
		return fmt.Errorf("audit: register job: %w", err)
	}
	// 事件订阅声明（单一事实源——运行时 Subscribe 必须有 Manifest 对偶，
	// 发布方=identity TopicLoginAudit；防「幽灵事件链」绕过 Build ④ 校验）。
	r.Events.Register(m.Name(), module.EventSubscribe, []string{idapievents.TopicLoginAudit})
	return nil
}

// AttachDB 在 DB 连接后由装配调用（与 bus/scheduler AttachDB 同惯例）：构造 DB Sink +
// 查询 repo + 为 retention job 接线 pool + 启动 flush loop（goroutine 禁止在
// Init 起——模块契约）。
func (m *Module) AttachDB(pool *database.Pool, startCtx context.Context) error {
	if pool == nil {
		return errors.New("audit: nil pool")
	}
	if m.service == nil {
		return errors.New("audit: service not initialized")
	}
	if err := m.service.RegisterSink(adapters.NewDBSink(pool)); err != nil {
		return fmt.Errorf("audit: register db sink: %w", err)
	}
	m.query = application.NewQueryService(adapters.NewQueryRepo(pool))
	m.job.SetPool(pool)
	m.service.Start(startCtx)
	return nil
}

// Service 访问器（登录审计订阅接线用——cmd 组装层调 audit.SubscribeLoginAudit）。
func (m *Module) Service() *application.Service { return m.service }

// Query 访问器（查询面；AttachDB 后就绪）。
func (m *Module) Query() *application.QueryService { return m.query }

// RegisterAdminRoutes 注册审计查询端点（面组装调用）。
func (m *Module) RegisterAdminRoutes(rg gin.IRouter,
	authorizer authz.Authorizer, onDeny func(*gin.Context, authn.Principal, string, string)) error {

	adapters.RegisterAdminRoutes(rg, adapters.Deps{
		Query:      m.query,
		Authorizer: authorizer,
		OnDeny:     onDeny,
	})
	return nil
}
