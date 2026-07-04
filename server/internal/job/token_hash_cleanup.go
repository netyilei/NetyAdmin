package job

import (
	"context"
	"encoding/json"
	"log/slog"

	"NetyAdmin/internal/pkg/task"
	userRepo "NetyAdmin/internal/repository/user"
)

// TokenHashCleanupJob 用户 token hash 清理任务。
// 定时物理删除 user_token_hashes 表中 expired_at < NOW() 的过期记录，
// 避免用户登录/刷新令牌过程中产生的 token hash 行无限堆积。
//
// 触发方式：Cron 表达式 "0 0 * * * *"，每小时整点执行一次。
// 利用 user_token_hashes 表上的 idx_user_token_expired 索引加速范围删除。
type TokenHashCleanupJob struct {
	userRepo userRepo.UserRepository
}

func NewTokenHashCleanupJob(userRepo userRepo.UserRepository) *TokenHashCleanupJob {
	return &TokenHashCleanupJob{userRepo: userRepo}
}

func (j *TokenHashCleanupJob) Name() string {
	return "token_hash_cleanup"
}

func (j *TokenHashCleanupJob) DisplayName() string {
	return "Token Hash Cleanup"
}

func (j *TokenHashCleanupJob) Run(ctx context.Context) error {
	affected, err := j.userRepo.DeleteExpiredTokenHashes(ctx)
	if err != nil {
		slog.Error("清理过期 token hash 失败", "error", err)
		return err
	}
	if affected > 0 {
		slog.Info("已清理过期 token hash 记录", "count", affected)
	}
	return nil
}

func (j *TokenHashCleanupJob) Execute(ctx context.Context, payload json.RawMessage) error {
	return j.Run(ctx)
}

func (j *TokenHashCleanupJob) DefaultMetadata() task.TaskMetadata {
	return task.TaskMetadata{
		Name:        j.Name(),
		DisplayName: j.DisplayName(),
		Type:        task.TypeCron,
		Spec:        "0 0 * * * *", // 每小时整点执行一次（cron.WithSeconds 6 字段）
		Weight:      task.WeightLow,
		Enabled:     true,
	}
}
