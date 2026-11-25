// migrations.go：bus 设施表迁移源（outbox/dlq；cmd migrate 命令登记用——
// 冒烟复：kernel 设施迁移此前未接入迁移命令，outbox/dlq 表缺失）。
package bus

import (
	"embed"
	"io/fs"

	"github.com/netyilei/netyadmin/internal/kernel/database"
)

//go:embed all:migrations
var migrationsFS embed.FS

// Migrations 返回 bus 设施表迁移源（模块段 330-339，contracts/migrations.yaml）。
func Migrations() fs.FS { return database.MigrationsOf(migrationsFS) }
