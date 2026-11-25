// retry.go：发送重试 Job（scheduler Job——cluster 域单执行）。
// 周期扫描 pending/failed 且未穷尽重试的记录逐条 attempt（幂等——状态机推进）。
package application

import (
	"context"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/scheduler"
)

// RetryJob 实现 scheduler.Job（message.records.retry——Manifest jobs 声明对偶）。
type RetryJob struct {
	svc *Service
}

// NewRetryJob 创建（svc=消息服务——repo/driver 经其访问）。
func NewRetryJob(svc *Service) *RetryJob { return &RetryJob{svc: svc} }

// Name 实现 scheduler.Job。
func (j *RetryJob) Name() string { return "message.records.retry" }

// Spec 实现 scheduler.Job：每 5 分钟 cluster 单执行；超时 60s；重试 2 次。
func (j *RetryJob) Spec() scheduler.Spec {
	return scheduler.Spec{
		Trigger:  scheduler.TriggerInterval,
		Duration: 5 * time.Minute,
		Timeout:  60 * time.Second,
		Retries:  2,
		Scope:    scheduler.ScopeCluster,
	}
}

// Run 实现 scheduler.Job（单条失败不阻断——下轮重扫）。
func (j *RetryJob) Run(ctx context.Context, _ scheduler.Execution) error {
	recs, err := j.svc.repo.Records.Retryable(ctx, 50)
	if err != nil {
		return err
	}
	for _, rec := range recs {
		j.svc.attempt(rec)
	}
	return nil
}
