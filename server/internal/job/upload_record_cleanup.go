package job

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"NetyAdmin/internal/pkg/pubsub"
	"NetyAdmin/internal/pkg/task"
	storageRepo "NetyAdmin/internal/repository/storage"
)

// UploadRecordCleanupJob 上传记录清理任务。
// 扫描 status=pending 且已超期(expires_at < now) 的上传凭证记录，标记为 expired，
// 避免用户签发凭证后不真实上传导致 pending 记录无限堆积。
type UploadRecordCleanupJob struct {
	recordRepo storageRepo.RecordRepository
	eventBus   pubsub.EventBus
}

func NewUploadRecordCleanupJob(recordRepo storageRepo.RecordRepository, eventBus pubsub.EventBus) *UploadRecordCleanupJob {
	return &UploadRecordCleanupJob{recordRepo: recordRepo, eventBus: eventBus}
}

func (j *UploadRecordCleanupJob) Name() string {
	return "upload_record_cleanup"
}

func (j *UploadRecordCleanupJob) DisplayName() string {
	return "Upload Record Cleanup"
}

func (j *UploadRecordCleanupJob) Run(ctx context.Context) error {
	affected, err := j.recordRepo.CleanupExpiredPending(ctx, time.Now())
	if err != nil {
		slog.Error("标记过期 pending 记录失败", "error", err)
		return err
	}
	if affected > 0 {
		slog.Info("已标记过期上传记录为 expired", "count", affected)
		// 聚合事件（best-effort）：通知下游有凭证超期作废（疑似上传未完成），
		// payload 仅含数量；需要逐条明细的下游可后续扩展 repo 返回 ID 列表
		if j.eventBus != nil {
			if payload, mErr := json.Marshal(map[string]interface{}{"count": affected, "markedAt": time.Now().Format(time.RFC3339)}); mErr == nil {
				if pErr := j.eventBus.Publish(ctx, pubsub.TopicStorageExpired, string(payload)); pErr != nil {
					slog.Warn("storage expired event: publish failed (best-effort)", "err", pErr)
				}
			}
		}
	}
	return nil
}

func (j *UploadRecordCleanupJob) Execute(ctx context.Context, payload json.RawMessage) error {
	return j.Run(ctx)
}

func (j *UploadRecordCleanupJob) DefaultMetadata() task.TaskMetadata {
	return task.TaskMetadata{
		Name:        j.Name(),
		DisplayName: j.DisplayName(),
		Type:        task.TypeCron,
		Spec:        "0 */30 * * * *", // 每 30 分钟执行一次
		Weight:      task.WeightLow,
		Enabled:     true,
	}
}
