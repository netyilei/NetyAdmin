package job

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"NetyAdmin/internal/pkg/task"
	storageRepo "NetyAdmin/internal/repository/storage"
)

// UploadRecordCleanupJob 上传记录清理任务。
// 扫描 status=pending 且已超期(expires_at < now) 的上传凭证记录，标记为 expired，
// 避免用户签发凭证后不真实上传导致 pending 记录无限堆积。
type UploadRecordCleanupJob struct {
	recordRepo storageRepo.RecordRepository
}

func NewUploadRecordCleanupJob(recordRepo storageRepo.RecordRepository) *UploadRecordCleanupJob {
	return &UploadRecordCleanupJob{recordRepo: recordRepo}
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
		log.Printf("[UploadRecordCleanup] 标记过期 pending 记录失败: %v", err)
		return err
	}
	if affected > 0 {
		log.Printf("[UploadRecordCleanup] 已标记 %d 条过期上传记录为 expired", affected)
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
