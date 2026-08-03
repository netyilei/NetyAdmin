package job

import (
	"context"
	"encoding/json"
	"log/slog"

	"NetyAdmin/internal/pkg/task"
	userRepo "NetyAdmin/internal/repository/user"
)

// TokenHashCleanupJob cleans up expired client session rows from user_tokens.
//
// Why only user_tokens (not admin_tokens): client sessions accumulate per
// (user_id, platform) and a single device can produce many rows over time
// (re-install, re-login on new device with same platform string, etc.).
// admin_tokens are reclaimed by admin Logout / forced logout flows and do not
// need a background sweep.
//
// Trigger: cron "0 0 * * * *" (top of every hour). Uses the
// idx_user_tokens_access_expires index for the range scan.
type TokenHashCleanupJob struct {
	userTokenRepo userRepo.UserTokenRepository
}

func NewTokenHashCleanupJob(userTokenRepo userRepo.UserTokenRepository) *TokenHashCleanupJob {
	return &TokenHashCleanupJob{userTokenRepo: userTokenRepo}
}

func (j *TokenHashCleanupJob) Name() string {
	return "token_hash_cleanup"
}

func (j *TokenHashCleanupJob) DisplayName() string {
	return "Token Hash Cleanup"
}

func (j *TokenHashCleanupJob) Run(ctx context.Context) error {
	affected, err := j.userTokenRepo.DeleteExpired(ctx)
	if err != nil {
		slog.Error("清理过期 user_tokens 失败", "error", err)
		return err
	}
	if affected > 0 {
		slog.Info("已清理过期 user_tokens 记录", "count", affected)
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
