// Package migration 提供数据库迁移能力。
//
// 基于 github.com/golang-migrate/migrate/v4（v4.18.2）实现，替代原先自研的迁移器。
// 迁移 SQL 文件以 go:embed 方式编译进二进制，部署时无需携带迁移文件。
//
// 迁移文件组织（golang-migrate 标准 NNNN_name.up.sql / .down.sql 格式）：
//   - 0001-0036: 表结构（CREATE TABLE，含历史增量变更压扁后的最终形态）
//   - 0037-0039: 外键约束
//   - 0040-0057: 种子数据（INSERT，无 down 文件，重建即恢复）
//
// 版本管理：golang-migrate 会在数据库中创建 schema_migrations 表追踪版本号。
package migration

import (
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // 注册 postgres driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// migrationFS 嵌入 migrations 子目录下的所有 .sql 文件。
// 编译进二进制后，部署无需携带外部迁移文件。
//
//go:embed migrations/*.sql
var migrationFS embed.FS

// migrationFSRoot 嵌入的迁移文件在 embed.FS 中的根目录路径。
const migrationFSRoot = "migrations"

// Run 执行数据库迁移到最新版本。
//
// 参数 dsn 为 PostgreSQL 连接串（keyword=value 格式，见 config.DatabaseConfig.DSN()）。
//
// 行为说明：
//   - 全新数据库：从版本 0 顺序执行所有 up 迁移（建表 + 约束 + 种子数据）。
//   - 已有数据库且版本为最新：返回 nil（ErrNoChange 视为成功）。
//   - 已有数据库但版本落后：自动执行增量迁移。
//
// 注意：本函数会创建独立的数据库连接（不复用 GORM 连接池），
// 以避免 golang-migrate 的 advisory lock 与业务查询相互阻塞。
func Run(dsn string) error {
	src, err := iofs.New(migrationFS, migrationFSRoot)
	if err != nil {
		return fmt.Errorf("初始化迁移源失败: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("创建迁移实例失败: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			// 数据库已是最新版本，无变更需要执行，视为成功
			return nil
		}
		return fmt.Errorf("执行迁移失败: %w", err)
	}
	return nil
}
