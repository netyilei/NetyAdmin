// migrations.go：迁移源收拢（G1-06——embed migrations 目录的 fs.Sub 剥取此前
// 在 11 处手写重复：9 个模块的 Migrations() 访问器与 Init 注册各一份 + kernel
// bus/scheduler 设施源）。迁移设施（MigrationSource/Runner/MigrationsOf）单一面。
package database

import "io/fs"

// MigrationsOf 从模块 embed 根（//go:embed all:migrations）剥出 migrations/
// 子目录作为迁移源。embed 模式编译期保证 migrations/ 目录存在，运行期失败=
// 程序员错误（误传非 embed FS），panic 快速暴露。
func MigrationsOf(fsys fs.FS) fs.FS {
	sub, err := fs.Sub(fsys, "migrations")
	if err != nil {
		panic(err) // embed 结构编译期保证
	}
	return sub
}
