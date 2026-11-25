// runs.go：运行记录持久化（scheduler_runs）——先写 running 再执行、
// 终态回写、启动时本节点 running→unknown 崩溃恢复、失败穷尽深度校准、
// RecentRuns 管理面查询。status smallint 枚举（0=running 1=succeeded
// 2=failed 3=unknown；Go 侧具名常量，migrations.md §11）。
package scheduler

import (
	"context"
	"fmt"
	"time"
)

// 运行状态枚举（SMALLINT；具名常量禁魔法数字——G1-08）。
const (
	runRunning   = 0
	runSucceeded = 1
	runFailed    = 2
	runUnknown   = 3

	runRetention = 30 * 24 * time.Hour // 终态行保留窗口（超期清理，防无限增长——审计 S2）
)

// cleanupRuns 清理超保留窗口的终态行（succeeded/failed/unknown；running 不动）。
// 挂失败深度校准时顺带执行（低频；审计 S2 修复）。
func (s *Scheduler) cleanupRuns(ctx context.Context) {
	if s.db == nil {
		return
	}
	cutoff := s.clock.Now().Add(-runRetention)
	if _, err := s.db.Exec(ctx, `DELETE FROM scheduler_runs
		WHERE status <> $1 AND started_at < $2`, runRunning, cutoff); err != nil {
		s.logger.Warn("scheduler runs cleanup failed", "err", err)
	}
}

// runRecord 行形状（列集；duration 由 started/finished 派生不落列）。
type runRecord struct {
	runID       string
	job         string
	node        string
	triggerKind string
	shard       int
	attempt     int
	traceID     string
	startedAt   time.Time
}

// insertRun 执行前写 running（先写 running 再执行）。
func (s *Scheduler) insertRun(ctx context.Context, r runRecord) error {
	if _, err := s.db.Exec(ctx, `INSERT INTO scheduler_runs
		(run_id, job, node, trigger_kind, shard, status, attempt, error, trace_id, started_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'',$8,$9)`,
		r.runID, r.job, r.node, r.triggerKind, r.shard, runRunning, r.attempt, r.traceID, r.startedAt); err != nil {
		return fmt.Errorf("scheduler: run insert: %w", err)
	}
	return nil
}

// finishRun 终态回写（成功/失败；含重试后的最终 attempt）。
func (s *Scheduler) finishRun(ctx context.Context, runID, job string, status int, attempt int, errMsg string) {
	if s.db == nil {
		return
	}
	if _, err := s.db.Exec(ctx, `UPDATE scheduler_runs
		SET status = $3, attempt = $4, error = $5, finished_at = $6, updated_at = $6
		WHERE run_id = $1 AND job = $2`, runID, job, status, attempt, errMsg, s.clock.Now()); err != nil {
		s.logger.Warn("scheduler run finish failed", "job", job, "run_id", runID, "err", err)
	}
}

// markUnknown ctx 取消/排空超时的在途运行标记 unknown（关闭语义）。
func (s *Scheduler) markUnknown(ctx context.Context, runID, job string) {
	if s.db == nil {
		return
	}
	if _, err := s.db.Exec(ctx, `UPDATE scheduler_runs
		SET status = $3, finished_at = $4, updated_at = $4
		WHERE run_id = $1 AND job = $2 AND status = $5`,
		runID, job, runUnknown, s.clock.Now(), runRunning); err != nil {
		s.logger.Warn("scheduler run unknown-mark failed", "job", job, "run_id", runID, "err", err)
	}
}

// recoverRuns 启动恢复：本节点上次生命周期的 running 行（进程已死，状态未知）
// 标记 unknown；多节点其他存活节点的在途行不动（存活判定随成员感知）。
func (s *Scheduler) recoverRuns(ctx context.Context) error {
	if _, err := s.db.Exec(ctx, `UPDATE scheduler_runs
		SET status = $2, finished_at = $3, updated_at = $3
		WHERE node = $4 AND status = $1`, runRunning, runUnknown, s.clock.Now(), s.nodeID); err != nil {
		return fmt.Errorf("scheduler: run recovery: %w", err)
	}
	return nil
}

// markAbandonedRunning 排空超时后统一兜底：本节点超龄 running 全部标记 unknown。
func (s *Scheduler) markAbandonedRunning(ctx context.Context, older time.Duration) {
	if s.db == nil {
		return
	}
	cutoff := s.clock.Now().Add(-older)
	if _, err := s.db.Exec(ctx, `UPDATE scheduler_runs
		SET status = $2, finished_at = $3, updated_at = $3
		WHERE node = $4 AND status = $1 AND started_at < $5`,
		runRunning, runUnknown, s.clock.Now(), s.nodeID, cutoff); err != nil {
		s.logger.Warn("scheduler abandoned-mark failed", "err", err)
	}
}

// syncFailedDepth 校准失败穷尽深度 gauge（重试穷尽 failed 行数=可重放死信量），
// 顺带清理超保留窗口的终态行。
func (s *Scheduler) syncFailedDepth(ctx context.Context) {
	if s.db == nil {
		return
	}
	var n int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM scheduler_runs WHERE status = $1`, runFailed).Scan(&n); err != nil {
		return // 校准失败不告警（下次失败时重试）
	}
	s.metrics.SchedulerDLQDepth.Set(float64(n))
	s.cleanupRuns(ctx)
}

// RunRecord 管理面查询行（taskcenter 端点随）。
type RunRecord struct {
	ID         int64      `json:"id"`
	RunID      string     `json:"run_id"`
	Job        string     `json:"job"`
	Node       string     `json:"node"`
	Trigger    string     `json:"trigger_kind"`
	Shard      int        `json:"shard"`
	Status     string     `json:"status"`
	Attempt    int        `json:"attempt"`
	Error      string     `json:"error"`
	TraceID    string     `json:"trace_id"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// runStatusText smallint → 管理面字符串（查询面出口归一）。
func runStatusText(v int) string {
	switch v {
	case runRunning:
		return "running"
	case runSucceeded:
		return "succeeded"
	case runFailed:
		return "failed"
	case runUnknown:
		return "unknown"
	}
	return "unknown"
}

// RecentRuns 最近运行记录（管理面/诊断查询；job 为空=全部任务）。
func (s *Scheduler) RecentRuns(ctx context.Context, job string, limit int) ([]RunRecord, error) {
	if s.db == nil {
		return nil, ErrNoDB
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200 // 上限封顶（§8 边界；不做静默降级）——审计 S3/F6
	}
	const q = `SELECT id, run_id, job, node, trigger_kind, shard, status, attempt,
		error, trace_id, started_at, finished_at FROM scheduler_runs
		WHERE ($1 = '' OR job = $1) ORDER BY id DESC LIMIT $2`
	rows, err := s.db.Query(ctx, q, job, limit)
	if err != nil {
		return nil, fmt.Errorf("scheduler: runs query: %w", err)
	}
	defer rows.Close()
	out := []RunRecord{}
	for rows.Next() {
		var r RunRecord
		var status int
		if err := rows.Scan(&r.ID, &r.RunID, &r.Job, &r.Node, &r.Trigger, &r.Shard,
			&status, &r.Attempt, &r.Error, &r.TraceID, &r.StartedAt, &r.FinishedAt); err != nil {
			return nil, fmt.Errorf("scheduler: runs scan: %w", err)
		}
		r.Status = runStatusText(status)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scheduler: rows: %w", err)
	}
	return out, nil
}
