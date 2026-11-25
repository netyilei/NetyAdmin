// migrate.go：迁移族子命令（单二进制 `-mode=migrate`）——
// up/status + PG advisory lock（audit-m1 数-登1）+ 首启管理员 bootstrap +
// 运维重置密码（F11 清偿：bootstrap 密码遗失锁死出口）。
// 迁移源登记=assembly 同款单一入口纪律：新增模块在 moduleMigrations 加一行
// （拆卸=删行）。漏登记/死条目由 CI check-migrations 对偶门禁双向拦截
// （gw P0/storage 冒烟 500 两次复发的结构性消除）。
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/bus"
	"github.com/netyilei/netyadmin/internal/kernel/config"
	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/scheduler"
	"github.com/netyilei/netyadmin/internal/modules/audit"
	"github.com/netyilei/netyadmin/internal/modules/configcenter"
	"github.com/netyilei/netyadmin/internal/modules/gw"
	"github.com/netyilei/netyadmin/internal/modules/identity"
	identitydomain "github.com/netyilei/netyadmin/internal/modules/identity/domain"
	"github.com/netyilei/netyadmin/internal/modules/message"
	"github.com/netyilei/netyadmin/internal/modules/rbac"
	"github.com/netyilei/netyadmin/internal/modules/storage"
	"github.com/netyilei/netyadmin/internal/modules/users"
)

// moduleMigrations 是启用模块的迁移源清单（单一入口；顺序无关——runner 按号段排序）。
func moduleMigrations() []database.MigrationSource {
	// 迁移源单一入口（与 assembly.go registerModules 模块清单对齐——
	// 新模块落地必须同步登记；冒烟 P0 教训：漏登记=迁移命令建不出表，集成
	// 测试自建 runner 掩盖）。
	// 清单=kernel 设施（bus/scheduler）+ 模块（audit/configcenter/identity/rbac）。
	return []database.MigrationSource{
		{Module: "bus", FS: bus.Migrations()},
		{Module: "scheduler", FS: scheduler.Migrations()},
		{Module: "audit", FS: audit.Migrations()},
		{Module: "configcenter", FS: configcenter.Migrations()},
		{Module: "gw", FS: gw.Migrations()},
		{Module: "identity", FS: identity.Migrations()},
		{Module: "rbac", FS: rbac.Migrations()},
		{Module: "message", FS: message.Migrations()},
		// users 迁移恒登记（模块运行开关 config.users.enabled 不影响部署期迁移——
		// 表存在且模块关闭=零开销；开关只控制路由/Job/goroutine）
		{Module: "users", FS: users.Migrations()},
		{Module: "storage", FS: storage.Migrations()},
	}
}

// withMigrationDB 迁移族子命令共用面（G1-06 收拢：up/status/重置密码同一
// 配置加载+连接池+迁移锁形态）：迁移锁内执行 fn，错误打 stderr 返回 1。
func withMigrationDB(cfgPath string, fn func(ctx context.Context, pool *database.Pool) error) int {
	cfg, _, err := config.Load(config.LoadOptions{Path: cfgPath})
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: config: %v\n", err)
		return 1
	}
	pool, err := database.New(context.Background(), database.BuildDSN(cfg.Database), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: db: %v\n", err)
		return 1
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := pool.WithMigrationLock(ctx, func(mctx context.Context) error {
		return fn(mctx, pool)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		return 1
	}
	return 0
}

// runMigrate 执行迁移子命令（action: up|status）。返回退出码。
func runMigrate(action, cfgPath string) int {
	return withMigrationDB(cfgPath, func(mctx context.Context, pool *database.Pool) error {
		runner := database.NewRunner(pool, database.RunnerOptions{Sources: moduleMigrations()})
		if action == "status" {
			return runner.Status(mctx, os.Stdout)
		}
		if action != "up" {
			return fmt.Errorf("unknown migrate action %q (up|status)", action)
		}
		if err := runner.Up(mctx); err != nil {
			return err
		}
		// bootstrap admin + super-admin 绑定在迁移锁内执行（双实例并发首启防
		// username 唯一约束撞车；配套）
		password, adminID, created, err := identity.EnsureBootstrapAdmin(mctx, pool)
		if err != nil {
			return fmt.Errorf("bootstrap admin: %w", err)
		}
		bound, err := rbac.EnsureBootstrapBinding(mctx, pool, adminID)
		if err != nil {
			return fmt.Errorf("bootstrap binding: %w", err)
		}
		if bound {
			fmt.Println("  bound to: super-admin")
		}
		// 内置网关面应用自举（——frontend-app 接受 IP/频率管控；
		// secret 随机加密入库不外显）。
		mcfg, _, cfgErr := config.Load(config.LoadOptions{Path: cfgPath})
		if cfgErr != nil {
			return cfgErr
		}
		gwSecretKey, gwKeyErr := decodeGWKey(mcfg)
		if gwKeyErr != nil {
			return fmt.Errorf("gw secret key: %w", gwKeyErr)
		}
		createdApps, gwErr := gw.EnsureGatewayApps(mctx, pool, gwSecretKey)
		if gwErr != nil {
			return fmt.Errorf("gateway apps bootstrap: %w", gwErr)
		}
		for _, key := range createdApps {
			fmt.Println("  gateway app created: " + key)
		}
		if created {
			fmt.Println("==============================================")
			fmt.Println("bootstrap admin created:")
			fmt.Println("  username: admin")
			fmt.Println("  password: " + password)
			fmt.Println("  (shown once; must_change_password enforced on first login)")
			fmt.Println("==============================================")
		}
		return nil
	})
}

// runResetPassword 执行运维重置密码子命令。迁移锁内执行（与 up 同锁面）：
// TokenVersion++ 全会话作废 + 随机密码 + must_change_password，明文一次性
// 输出 stdout（不出本函数范围——不落日志/DB）。
func runResetPassword(username, cfgPath string) int {
	if username == "" {
		fmt.Fprintln(os.Stderr, "reset-password: -username is required")
		return 1
	}
	return withMigrationDB(cfgPath, func(mctx context.Context, pool *database.Pool) error {
		password, err := identity.ResetAdminPassword(mctx, pool, username)
		if errors.Is(err, identitydomain.ErrNotFound) {
			return fmt.Errorf("reset-password: admin %q not found", username)
		}
		if err != nil {
			return err
		}
		fmt.Println("==============================================")
		fmt.Println("admin password reset:")
		fmt.Println("  username: " + username)
		fmt.Println("  password: " + password)
		fmt.Println("  (shown once; all previous sessions revoked; must_change_password enforced on first login)")
		fmt.Println("==============================================")
		return nil
	})
}
