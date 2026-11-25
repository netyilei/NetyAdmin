// Package storage 是对象存储模块——存储配置/S3 通用协议驱动
// （minio-go，兼容 AWS S3/MinIO/其他 S3 端点）/presigned 直传凭证/HMAC
// complete 防伪/过期清理 Job。驱动模式（精神「通用协议优先」）：
// S3 API 即通用协议；厂商专用驱动下游按 domain.StorageDriver 自写。
// 模块契约边界：module.go 只做装配注册，业务在 application，存储在 adapters。
package storage

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	klog "github.com/netyilei/netyadmin/internal/kernel/log"
	"github.com/netyilei/netyadmin/internal/kernel/module"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/modules/storage/adapters"
	"github.com/netyilei/netyadmin/internal/modules/storage/application"
)

//go:embed all:migrations
var migrationsFS embed.FS

// Migrations 返回模块迁移源。
func Migrations() fs.FS { return database.MigrationsOf(migrationsFS) }

// Module 实现 kernel/module 契约。
type Module struct {
	service *application.Service
}

// New 构造（hmacSalt=complete 防伪盐 env Secret——空=complete 全拒 fail-closed；
// ttlSeconds=凭证有效期配置面——credentials_ttl_seconds，0=默认 900。
// 原 TTL 配置未接线=改配置无效果，联测根修）。
func New(hmacSalt string, ttlSeconds int, logMgr *klog.Manager) *Module {
	access := logMgr.Logger("storage", "access")
	return &Module{service: application.New(application.Config{
		HMACSalt: hmacSalt, CredentialsTTL: time.Duration(ttlSeconds) * time.Second,
	}, hmacSalt, access)}
}

// Name 实现 Module。
func (m *Module) Name() string { return "storage" }

// DependsOn 实现 Module（无业务模块依赖）。
func (m *Module) DependsOn() []string { return nil }

// Init 纯装配：迁移源/Job 注册/Port Expose（禁 IO——repo 由 Attach 注入）。
func (m *Module) Init(_ context.Context, r *module.Registry) error {
	if err := r.Migrations.Register(m.Name(), Migrations()); err != nil {
		return fmt.Errorf("storage: register migrations: %w", err)
	}
	if err := r.Jobs.Register(m.Name(), application.NewCleanupJob(m.service).Name(),
		application.NewCleanupJob(m.service)); err != nil {
		return fmt.Errorf("storage: register cleanup job: %w", err)
	}
	if err := r.Services.Expose("storage.UploadService", uploadPort{m.service}); err != nil {
		return fmt.Errorf("storage: expose service: %w", err)
	}
	return nil
}

// AttachDB 在 DB 连接后由装配调用。
func (m *Module) AttachDB(pool *database.Pool, _ context.Context) error {
	if pool == nil {
		return errors.New("storage: nil pool") // 批间一致性——audit/message 同守卫
	}
	m.service.Attach(&application.RepoPair{
		Configs: adapters.NewStorageConfigRepo(pool),
		Records: adapters.NewUploadRecordRepo(pool),
	})
	return nil
}

// Service 访问器。
func (m *Module) Service() *application.Service { return m.service }

// RegisterAdminRoutes 注册存储管理端点（cmd 组合根调用）。
func (m *Module) RegisterAdminRoutes(authed gin.IRouter,
	authorizer authz.Authorizer, onDeny func(*gin.Context, authn.Principal, string, string)) error {

	adapters.RegisterAdminRoutes(authed, adapters.AdminDeps{
		Service: m.service, Authorizer: authorizer, OnDeny: onDeny,
	})
	return nil
}

// RegisterClientRoutes 注册 client 上传端点（authed 组由 users JWT
// 中间件承载——cmd 接线；principal 即 uploader）。
func (m *Module) RegisterClientRoutes(authed gin.IRouter) {
	adapters.RegisterClientRoutes(authed, m.service)
}
