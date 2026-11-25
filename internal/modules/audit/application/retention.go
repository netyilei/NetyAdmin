// retention.go：operation_logs 保留清理 job（验收「保留」；
// observability.md §1 基线 180 天，可配）。cluster 域：全集群单执行（清理幂等，
// 选主去重）；失败重试穷尽即死信（TriggerNow 重放， 运行记录语义）。
package application

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/scheduler"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
)

// RetentionJob 实现 scheduler.Job（audit.operation_logs.retention）。
// pool 延迟接线（模块 Init 纯装配，AttachDB 后 SetPool；未接线时 Run fail-closed）。
type RetentionJob struct {
	pool          atomic.Pointer[database.Pool]
	clock         primitives.Clock // 业务保留窗口计算（ai-coding §3-5；nil=SystemClock）
	retentionDays int
}

// NewRetentionJob 创建（retentionDays ≤0 取默认 180）。
func NewRetentionJob(retentionDays int) *RetentionJob {
	return &RetentionJob{retentionDays: retentionDays}
}

// SetClock 注入时钟（测试固定窗口；生产默认 SystemClock 由调用方 Set）。
func (j *RetentionJob) SetClock(clock primitives.Clock) { j.clock = clock }

// SetPool 接线连接池（模块 Attach 阶段调用）。
func (j *RetentionJob) SetPool(pool *database.Pool) { j.pool.Store(pool) }

// Name 实现 scheduler.Job。
func (j *RetentionJob) Name() string { return "audit.operation_logs.retention" }

// Spec 实现 scheduler.Job：每日 03:30 cluster 单执行；超时/重试显式声明。
func (j *RetentionJob) Spec() scheduler.Spec {
	return scheduler.Spec{
		Trigger:  scheduler.TriggerCron,
		Schedule: "30 3 * * *",
		Timeout:  5 * time.Minute,
		Retries:  2,
		Scope:    scheduler.ScopeCluster,
	}
}

// Run 实现 scheduler.Job：删除保留窗口外记录（分批防长事务——单语句即可，
// created_at 索引 12100 覆盖；行数大时分批循环）。
func (j *RetentionJob) Run(ctx context.Context, _ scheduler.Execution) error {
	pool := j.pool.Load()
	if pool == nil {
		return errors.New("audit: retention pool not attached (fail-closed)")
	}
	clock := j.clock
	if clock == nil {
		clock = primitives.SystemClock{}
	}
	cutoff := clock.Now().UTC().AddDate(0, 0, -j.retentionDays)
	const batchLimit = 5000
	for {
		tag, err := pool.Exec(ctx,
			`DELETE FROM operation_logs WHERE id IN (
				SELECT id FROM operation_logs WHERE created_at < $1 LIMIT $2
			)`, cutoff, batchLimit)
		if err != nil {
			return fmt.Errorf("audit: retention delete: %w", err)
		}
		n := tag.RowsAffected()
		if n < batchLimit {
			break
		}
	}
	return nil
}
