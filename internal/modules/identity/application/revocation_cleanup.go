// revocation_cleanup.go：admin_token_revocations 过期行清理 job（
// JOB-01 移交项；11002 表注释「expires_at=TTL 对齐 token 寿命（清理口径）」）。
// cluster 域 24h：全集群单执行（清理幂等，选主去重）；过期行读源恒 miss，
// 删除纯容量维护不触安全语义。audit RetentionJob 同款惯例：repo 延迟接线
// （Init 纯装配，AttachDB 后 SetRepo；未接线时 Run fail-closed）。
package application

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/scheduler"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/identity/domain"
)

// RevocationCleanupJob 实现 scheduler.Job（identity.token_revocations.cleanup）。
type RevocationCleanupJob struct {
	mu    sync.Mutex
	repo  domain.Repo      // 清理契约（adapters 实现；AttachDB 接线）
	clock primitives.Clock // 截止时间计算（测试固定窗口；nil=SystemClock）
}

// NewRevocationCleanupJob 创建。
func NewRevocationCleanupJob() *RevocationCleanupJob {
	return &RevocationCleanupJob{}
}

// SetRepo 接线存储契约（模块 AttachDB 阶段调用）。
func (j *RevocationCleanupJob) SetRepo(repo domain.Repo) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.repo = repo
}

// SetClock 注入时钟（测试固定窗口；生产默认 SystemClock）。
func (j *RevocationCleanupJob) SetClock(clock primitives.Clock) { j.clock = clock }

// Name 实现 scheduler.Job。
func (j *RevocationCleanupJob) Name() string { return "identity.token_revocations.cleanup" }

// Spec 实现 scheduler.Job：24h 周期 cluster 单执行；超时/重试显式声明。
func (j *RevocationCleanupJob) Spec() scheduler.Spec {
	return scheduler.Spec{
		Trigger:  scheduler.TriggerInterval,
		Duration: 24 * time.Hour,
		Timeout:  5 * time.Minute,
		Retries:  2,
		Scope:    scheduler.ScopeCluster,
	}
}

// Run 实现 scheduler.Job：删除 expires_at 已过的吊销行（分批在 repo 内收口）。
func (j *RevocationCleanupJob) Run(ctx context.Context, _ scheduler.Execution) error {
	j.mu.Lock()
	repo := j.repo
	j.mu.Unlock()
	if repo == nil {
		return errors.New("identity: revocation cleanup repo not attached (fail-closed)")
	}
	clock := j.clock
	if clock == nil {
		clock = primitives.SystemClock{}
	}
	_, err := repo.CleanExpiredRevocations(ctx, clock.Now().UTC())
	return err
}
