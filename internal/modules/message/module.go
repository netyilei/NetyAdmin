// Package message 是消息通知模块——模板/发送记录/驱动模式
// （精神：email=SMTP 标准协议内置驱动；sms=log 默认驱动+驱动注册
// 表开放——厂商驱动下游按 domain.Driver 契约自写 Register 注册）。
// 发送路径：模板渲染 → 异步 worker → active 驱动 → 记录台账；retry Job 周期
// 扫 pending/failed（≤MaxRetries）。
package message

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/gin-gonic/gin"
	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/log"
	"github.com/netyilei/netyadmin/internal/kernel/module"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/modules/message/adapters"
	"github.com/netyilei/netyadmin/internal/modules/message/application"
)

//go:embed all:migrations
var migrationsFS embed.FS

// Migrations 返回模块迁移源。
func Migrations() fs.FS { return database.MigrationsOf(migrationsFS) }

// Module 实现 kernel/module 契约。
type Module struct {
	service *application.Service
}

// New 构造（logMgr=消息模块日志面——access 流水+log 驱动出口；cfg=驱动选择
// 与 SMTP 参数——kernel config.Message 映射；obs=共享指标面，nil=不出指标）。
func New(logMgr *log.Manager, cfg application.Config, obs *observability.Metrics) *Module {
	return &Module{service: application.New(cfg,
		logMgr.Logger("message", "access"), logMgr.Logger("message", "error"), obs)}
}

// Name 实现 Module。
func (m *Module) Name() string { return "message" }

// DependsOn 实现 Module（无业务模块依赖）。
func (m *Module) DependsOn() []string { return nil }

// Init 纯装配：迁移源/Port Expose（禁 IO——worker 由 Attach 启动）。
func (m *Module) Init(_ context.Context, r *module.Registry) error {
	if err := r.Migrations.Register(m.Name(), Migrations()); err != nil {
		return fmt.Errorf("message: register migrations: %w", err)
	}
	if err := r.Services.Expose("message.Sender", senderPort{m.service}); err != nil {
		return fmt.Errorf("message: expose sender: %w", err)
	}
	if err := r.Services.Expose("message.Templates", templatesPort{m.service}); err != nil {
		return fmt.Errorf("message: expose templates: %w", err)
	}
	if err := r.Jobs.Register(m.Name(), application.NewRetryJob(m.service).Name(),
		application.NewRetryJob(m.service)); err != nil {
		return fmt.Errorf("message: register retry job: %w", err)
	}
	return nil
}

// AttachDB 在 DB 连接后由装配调用（repo 注入+发送 worker 启动——audit 同惯例）。
func (m *Module) AttachDB(pool *database.Pool, _ context.Context) error {
	if pool == nil {
		return errors.New("message: nil pool")
	}
	return m.service.Attach(&application.RepoPair{
		Templates: adapters.NewTemplateRepo(pool),
		Records:   adapters.NewRecordRepo(pool),
		Inbox:     adapters.NewRecordRepo(pool),
	})
}

// Service 访问器（adminapi 端点+retry Job 消费）。
func (m *Module) Service() *application.Service { return m.service }

// RegisterClientInboxRoutes 注册站内信 client 端点（authed 组由
// users JWT 中间件承载——cmd 接线；user_id 取 principal 防越权）。
func (m *Module) RegisterClientInboxRoutes(authed gin.IRouter) {
	adapters.RegisterInboxClientRoutes(authed, m.service)
}

// RegisterAdminRoutes 注册消息管理端点（cmd 组合根调用）。
func (m *Module) RegisterAdminRoutes(authed gin.IRouter,
	authorizer authz.Authorizer, onDeny func(*gin.Context, authn.Principal, string, string)) error {

	adapters.RegisterAdminRoutes(authed, adapters.Deps{
		Service:    m.service,
		Authorizer: authorizer,
		OnDeny:     onDeny,
	})
	return nil
}
