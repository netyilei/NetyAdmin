// Package users 是 C 端用户演示模块（US-01）——注册（邮箱+密码→
// 随机用户名）/登录（User JWT）/改用户名/改手机号；演示下游接入安全子系统
// （User JWT AuthN Provider 自注册——SecurityCollector.RegisterAuthN）+ PII
// 掩码注册示范 + 模块开关（enabled=false 零注册零开销）+ 拆卸演练（删目录+
// 删装配行 → 编译通过）。模块契约边界：module.go 只做装配注册。
package users

import (
	"context"
	"crypto/rsa"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	klog "github.com/netyilei/netyadmin/internal/kernel/log"
	"github.com/netyilei/netyadmin/internal/kernel/module"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/modules/users/adapters"
	"github.com/netyilei/netyadmin/internal/modules/users/application"
)

//go:embed all:migrations
var migrationsFS embed.FS

// Migrations 返回模块迁移源。
func Migrations() fs.FS { return database.MigrationsOf(migrationsFS) }

// Module 实现 kernel/module 契约。
type Module struct {
	service  *application.Service
	verifier *authn.JWTVerifier
}

// New 构造（signer/publicKeys= SigningKeys 面——签发与验签共用；
// logMgr=users 日志面）。
func New(signer *rsa.PrivateKey, publicKeys []*rsa.PublicKey, logMgr *klog.Manager, bcryptCost int) (*Module, error) {
	svc := application.New(application.Config{BcryptCost: bcryptCost}, signer,
		logMgr.Logger("users", "access"), logMgr.Logger("users", "app"))
	verifier, err := authn.NewJWTVerifier(authn.PrincipalUser, authn.BaseOptions{
		PublicKeys:        publicKeys,
		Issuer:            "netyadmin",
		Audience:          "client",
		CheckTokenVersion: true,
		VersionSource:     svc.TokenVersionSource,
	})
	if err != nil {
		return nil, fmt.Errorf("users: jwt verifier: %w", err)
	}
	return &Module{service: svc, verifier: verifier}, nil
}

// Name 实现 Module。
func (m *Module) Name() string { return "users" }

// DependsOn 实现 Module（无业务模块依赖）。
func (m *Module) DependsOn() []string { return nil }

// Init 纯装配：迁移源/Provider 自注册/Port Expose（禁 IO——repo 由 Attach 注入）。
func (m *Module) Init(_ context.Context, r *module.Registry) error {
	if err := r.Migrations.Register(m.Name(), Migrations()); err != nil {
		return fmt.Errorf("users: register migrations: %w", err)
	}
	// User JWT Provider 自注册（US-01「演示下游接入安全子系统」——
	// SecurityCollector.RegisterAuthN；name 全局唯一）
	provider := &usersProvider{verifier: m.verifier}
	if err := r.Security.RegisterAuthN(m.Name(), "users.jwt", provider); err != nil {
		return fmt.Errorf("users: register authn provider: %w", err)
	}
	if err := r.Services.Expose("users.Service", m.service); err != nil {
		return fmt.Errorf("users: expose service: %w", err)
	}
	// 跨模块消费面：其他模块读用户资料走 api.Users（不暴露 application 类型）。
	if err := r.Services.Expose("users.Users", usersPort{m.service}); err != nil {
		return fmt.Errorf("users: expose users port: %w", err)
	}
	return nil
}

// AttachDB 在 DB 连接后由装配调用（repo 注入）。
func (m *Module) AttachDB(pool *database.Pool, _ context.Context) error {
	if pool == nil {
		return errors.New("users: nil pool") // 批间一致性——audit/message 同守卫
	}
	m.service.Attach(adapters.NewUserRepo(pool))
	return nil
}

// Service 访问器。
func (m *Module) Service() *application.Service { return m.service }

// RegisterClientRoutes 是 client 路由自注册面（cmd 传 RouteRegistrar 组——
// users 自注册子组：public 端点直挂、authed 子组挂 User JWT 中间件；返回
// authed 组供 ticket 签发面挂载）。
func (m *Module) RegisterClientRoutes(registrar gin.IRouter, rlLogin gin.HandlerFunc) (authed gin.IRouter, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("users: client routes: %v", r)
		}
	}()
	root, ok := registrar.(gin.IRoutes)
	if !ok {
		return nil, fmt.Errorf("users: registrar is not gin.IRoutes")
	}
	limited := []gin.HandlerFunc{}
	if rlLogin != nil {
		limited = append(limited, rlLogin) // G2 公开 API 必须限流（users_login 策略）
	}
	limited = append(limited, adapters.RegisterHandler(m.service))
	root.POST("/users/register", limited...)
	limitedLogin := []gin.HandlerFunc{}
	if rlLogin != nil {
		limitedLogin = append(limitedLogin, rlLogin)
	}
	limitedLogin = append(limitedLogin, adapters.LoginHandler(m.service))
	root.POST("/users/login", limitedLogin...)
	authed = registrar.Group("", AuthMiddleware(m.verifier))
	authed.GET("/users/me", adapters.ProfileHandler(m.service))
	authed.PUT("/users/username", adapters.UpdateUsernameHandler(m.service))
	authed.PUT("/users/phone", adapters.UpdatePhoneHandler(m.service))
	return authed, nil
}

// RegisterAdminRoutes 注册 users 管理端点（cmd 组合根调用）。
func (m *Module) RegisterAdminRoutes(authed gin.IRouter,
	authorizer authz.Authorizer, onDeny func(*gin.Context, authn.Principal, string, string)) error {

	adapters.RegisterAdminRoutes(authed, adapters.AdminDeps{
		Service: m.service, Authorizer: authorizer, OnDeny: onDeny,
	})
	return nil
}

// AuthMiddleware 是 User JWT 鉴权（client authed 面——adapters 转发）。
func AuthMiddleware(v *authn.JWTVerifier) gin.HandlerFunc {
	return adapters.AuthMiddleware(v)
}

// usersProvider 是 SecurityCollector 自注册适配（US-01 演示——provider 面
// 委托模块 verifier）。
type usersProvider struct {
	verifier *authn.JWTVerifier
}

// Scheme 实现 authn.Provider。
func (p *usersProvider) Scheme() string { return p.verifier.Scheme() }

// Authenticate 实现 authn.Provider。
func (p *usersProvider) Authenticate(ctx context.Context, cred authn.Credential) (authn.Principal, error) {
	return p.verifier.Authenticate(ctx, cred)
}
