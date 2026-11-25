// cleanup.go：过期上传记录清理 Job（scheduler Job——cluster 域单执行；
// 过期 pending → expired 终态；对象删除=驱动面尽力——S3 对象由桶生命周期
// 策略兜底/下轮再扫，本 Job 只推进状态机）。
package application

import (
	"context"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/scheduler"
)

// CleanupJob 实现 scheduler.Job（storage.records.cleanup——Manifest 对偶）。
type CleanupJob struct {
	svc *Service
}

// NewCleanupJob 创建。
func NewCleanupJob(svc *Service) *CleanupJob { return &CleanupJob{svc: svc} }

// Name 实现 scheduler.Job。
func (j *CleanupJob) Name() string { return "storage.records.cleanup" }

// Spec 实现 scheduler.Job：每 10 分钟 cluster 单执行；超时 60s；重试 2 次。
func (j *CleanupJob) Spec() scheduler.Spec {
	return scheduler.Spec{
		Trigger:  scheduler.TriggerInterval,
		Duration: j.svc.cfg.CleanupInterval,
		Timeout:  60 * time.Second,
		Retries:  2,
		Scope:    scheduler.ScopeCluster,
	}
}

// Run 实现 scheduler.Job。
func (j *CleanupJob) Run(ctx context.Context, _ scheduler.Execution) error {
	_, err := j.svc.CleanupExpired(ctx)
	return err
}
