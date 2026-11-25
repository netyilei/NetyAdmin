// migrate_cmd.go：迁移命令支撑（前置；audit-m1 数-登1）——部署期 advisory
// lock（双进程并发迁移防护）+ 模块迁移源登记（assembly 同款单一入口纪律：
// 新增模块在 cmd 迁移清单加一行，拆卸=删行）。
package database

import (
	"context"
	"errors"
	"fmt"
)

// ErrMigrationLocked 是另一进程持有迁移锁（fast fail，不等待——部署期人工重试）。
var ErrMigrationLocked = errors.New("database: migration lock held by another process")

// advisoryLockKey 是迁移锁的固定键（hashtext 输入；单库一把——迁移是库级操作）。
const advisoryLockKey = "netyadmin.migrate.v1"

// WithMigrationLock 在 PG advisory lock 下执行 fn（会话级锁：fn 结束或连接断开
// 自动释放；pgxpool 单连接语义——用专用 Acquire 保证同连接加锁/解锁）。
// 已被持有 → ErrMigrationLocked（try-lock，不阻塞部署流水线）。
func (p *Pool) WithMigrationLock(ctx context.Context, fn func(ctx context.Context) error) error {
	conn, err := p.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("database: acquire for migration lock: %w", err)
	}
	defer conn.Release()

	var acquired bool
	err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, advisoryLockKey).Scan(&acquired)
	if err != nil {
		return fmt.Errorf("database: advisory lock: %w", err)
	}
	if !acquired {
		return ErrMigrationLocked
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(hashtext($1))`, advisoryLockKey)
	}()
	return fn(ctx)
}
