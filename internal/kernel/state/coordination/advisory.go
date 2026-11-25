// advisory.go：PG advisory 选主分支（「leader multi 的 PG
// advisory 分支」兑现）——Redis 挂时按 pg_advisory_lock_or_fail 策略转
// PostgreSQL 会话级 advisory lock（pg_try_advisory_lock：锁绑定连接生命周期，
// 连接断=自动让位——无 TTL 续租语义，RenewLeadership PG 分支恒成功）。
// pool 由装配方可选注入（AttachPG；未注入时 multi+PGAdvisoryOrFail=ErrUnavailable
// fail-closed 保持——与原语义一致）。
package coordination

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/netyilei/netyadmin/internal/kernel/database"
)

// pgAdvisoryCampaign 是一次会话级 advisory 选主的持有态——**独立专用连接**
// （pgx.Connect 非 pool Acquire：持锁连接不占池预算，pool.Close 不被阻塞；
// 进程退出连接断=PG 会话锁自动释放=崩溃自动让位，正是选主语义）。
type pgAdvisoryCampaign struct {
	conn *pgx.Conn
	name string
}

// ErrNotPGHolder 是非持有者操作 PG 选主面的错误（Resign/Renew 校验）。
var ErrNotPGHolder = errors.New("coordination: not the pg advisory holder")

// AttachPG 注入 PG advisory 分支的 DSN 源（cmd db-connect 后装配；nil=禁用分支）。
// dsn 取自主池连接串（同库——advisory lock 是实例级，非池级资源）。
func (c *Coordination) AttachPG(pool *database.Pool) {
	c.pgMu.Lock()
	// 旧持有态处置（审计 P1-B：换池不释放=连接泄漏+旧会话锁滞留——他节点
	// 永选不上；Resign 会话锁=unlock+连接断）
	if c.pgHolder != nil && c.pgPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _ = c.pgHolder.conn.Exec(ctx, `SELECT pg_advisory_unlock(hashtext($1))`, "coord:leader:"+c.pgHolder.name)
		cancel()
		_ = c.pgHolder.conn.Close(context.Background())
	}
	c.pgHolder = nil
	c.pgPool = pool
	c.pgMu.Unlock()
}

// pgCampaign 会话级选主：Acquire 专用连接 → pg_try_advisory_lock（hashtext(name)）。
// 成功=持有（连接保持到 Resign/进程退出——锁自动释放）；失败=他人持有（isLeader=false
// 非 error）；连接/查询失败=ErrUnavailable（fail-closed——PG 也不可用时不该自任 leader）。
func (c *Coordination) pgCampaign(ctx context.Context, name string) (holder string, isLeader bool, err error) {
	c.pgMu.Lock()
	defer c.pgMu.Unlock()
	if c.pgPool == nil {
		return "", false, ErrUnavailable
	}
	// 已持有者处理（评审复）：
	// 同名=活性校验后幂等返回（PG 重启/连接断→会话锁已释放，本进程不得自认
	// leader——Ping 失败即清持有态重选）；异名=拒绝（单进程单会话锁——异名
	// Campaign 会覆盖持有态泄漏旧连接+旧锁滞留，评审复）。
	if c.pgHolder != nil {
		// 活性校验带 deadline（审计 P1-A：TCP 半开 Ping 挂起=锁已让位而本进程
		// 自认 leader 的双主窗口——2s 预算与 scheduler Resign 同口径）
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := c.pgHolder.conn.Ping(pingCtx)
		cancel()
		if err != nil {
			// 连接死=锁已随会话释放——清持有态，走重新选主
			_ = c.pgHolder.conn.Close(ctx)
			c.pgHolder = nil
		}
	}
	if c.pgHolder != nil {
		if c.pgHolder.name == name {
			return "pg:" + name, true, nil
		}
		return "", false, fmt.Errorf("%w: process holds %q (single session lock per process)", ErrNotPGHolder, c.pgHolder.name)
	}
	conn, err := pgx.Connect(ctx, c.pgPool.Config().ConnString())
	if err != nil {
		return "", false, fmt.Errorf("%w: pg connect: %w", ErrUnavailable, err)
	}
	var ok bool
	if err := conn.QueryRow(ctx,
		`SELECT pg_try_advisory_lock(hashtext($1))`, "coord:leader:"+name).Scan(&ok); err != nil {
		_ = conn.Close(ctx)
		return "", false, fmt.Errorf("%w: pg try lock: %w", ErrUnavailable, err)
	}
	if !ok {
		_ = conn.Close(ctx) // 他人持有——连接即断（下轮 Campaign 再试）
		return "", false, nil
	}
	c.pgHolder = &pgAdvisoryCampaign{conn: conn, name: name}
	if c.metrics != nil {
		c.metrics.CoordFallbacks.WithLabelValues("leader:pg").Inc()
	}
	return "pg:" + name, true, nil
}

// pgRenew PG 分支续租：会话锁随连接生命周期无 TTL——续租即活性校验（评审
// 复：Ping 失败=PG 重启/连接断，会话锁已释放→清持有态返回错误，调用方
// 下轮 Campaign 重选——不得恒成功自认 leader 双主）。
func (c *Coordination) pgRenew(ctx context.Context, name, holder string) error {
	c.pgMu.Lock()
	defer c.pgMu.Unlock()
	if c.pgHolder == nil || c.pgHolder.name != name || holder != "pg:"+name {
		return ErrNotPGHolder
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	pingErr := c.pgHolder.conn.Ping(pingCtx)
	cancel()
	if pingErr != nil {
		_ = c.pgHolder.conn.Close(ctx)
		c.pgHolder = nil
		return fmt.Errorf("%w: pg session dead (%w)", ErrNotPGHolder, pingErr)
	}
	return nil
}

// pgResign 释放会话锁（unlock + 连接归还；非持有者拒绝）。
func (c *Coordination) pgResign(ctx context.Context, name, holder string) error {
	c.pgMu.Lock()
	defer c.pgMu.Unlock()
	if c.pgHolder == nil || c.pgHolder.name != name || holder != "pg:"+name {
		return ErrNotPGHolder
	}
	if _, err := c.pgHolder.conn.Exec(ctx, `SELECT pg_advisory_unlock(hashtext($1))`, "coord:leader:"+name); err != nil {
		return fmt.Errorf("coordination: pg unlock: %w", err)
	}
	_ = c.pgHolder.conn.Close(ctx)
	c.pgHolder = nil
	return nil
}
