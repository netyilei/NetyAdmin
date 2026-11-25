// migrations.go：scheduler 运行记录迁移源（运行记录表；cmd migrate 命令登记用——
// 冒烟复：kernel 设施迁移此前未接入迁移命令，运行记录表缺失）。
package scheduler

import (
	"embed"
	"io/fs"

	"github.com/netyilei/netyadmin/internal/kernel/database"
)

//go:embed all:migrations
var migrationsFS embed.FS

// Migrations 返回 scheduler 设施表迁移源（模块段 290-309，contracts/migrations.yaml）。
func Migrations() fs.FS { return database.MigrationsOf(migrationsFS) }
