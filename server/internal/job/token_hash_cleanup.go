package job

import (
	"context"
	"encoding/json"
	"log/slog"

	"NetyAdmin/internal/pkg/task"
	userRepo "NetyAdmin/internal/repository/user"
)

// TokenHashCleanupJob cleans up expired session rows from both user_tokens and admin_tokens.
//
// Why both tables:
//   - user_tokens (client multi-device): accumulates per (user_id, platform) over time.
//   - admin_tokens (admin sessions): admin login also produces rows that outlive their
//     access/refresh expiry; without a sweep they pile up indefinitely.
//
// Trigger: cron "0 0 * * * *" (top of every hour).
type TokenHashCleanupJob struct {
	userTokenRepo userRepo.UserTokenRepository
	userRepo      userRepo.UserRepository
}

func NewTokenHashCleanupJob(userTokenRepo userRepo.UserTokenRepository, userRepo userRepo.UserRepository) *TokenHashCleanupJob {
	return &TokenHashCleanupJob{userTokenRepo: userTokenRepo, userRepo: userRepo}
}

func (j *TokenHashCleanupJob) Name() string {
	return "token_hash_cleanup"
}

func (j *TokenHashCleanupJob) DisplayName() string {
	return "Token Hash Cleanup"
}

func (j *TokenHashCleanupJob) Run(ctx context.Context) error {
	// 1. 清理 user_tokens 过期行
	affected, err := j.userTokenRepo.DeleteExpired(ctx)
	if err != nil {
		slog.Error("清理过期 user_tokens 失败", "error", err)
		return err
	}
	if affected > 0 {
		slog.Info("已清理过期 user_tokens 记录", "count", affected)
	}
	// 2. 清理 admin_tokens 过期行（admin 登录产生的 token 行同样需要定期回收）
	adminAffected, err := j.userRepo.DeleteExpiredTokenHashes(ctx)
	if err != nil {
		slog.Error("清理过期 admin_tokens 失败", "error", err)
		return err
	}
	if adminAffected > 0 {
		slog.Info("已清理过期 admin_tokens 记录", "count", adminAffected)
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
