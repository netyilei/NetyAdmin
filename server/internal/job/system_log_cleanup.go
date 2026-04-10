package job

import (
	"context"
	"log"
	"strconv"
	"time"

	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/task"
	logRepo "NetyAdmin/internal/repository/log"
	systemRepo "NetyAdmin/internal/repository/system"
)

// SystemLogCleanupJob 统一日志清理任务
type SystemLogCleanupJob struct {
	taskLogRepo systemRepo.TaskLogRepository
	opsLogRepo  *logRepo.OperationRepository
	errLogRepo  *logRepo.ErrorRepository
	watcher     configsync.ConfigWatcher
}

func NewSystemLogCleanupJob(
	taskLogRepo systemRepo.TaskLogRepository,
	opsLogRepo *logRepo.OperationRepository,
	errLogRepo *logRepo.ErrorRepository,
	watcher configsync.ConfigWatcher,
) *SystemLogCleanupJob {
	return &SystemLogCleanupJob{
		taskLogRepo: taskLogRepo,
		opsLogRepo:  opsLogRepo,
		errLogRepo:  errLogRepo,
		watcher:     watcher,
	}
}

func (j *SystemLogCleanupJob) Name() string {
	return "system_log_cleanup"
}

func (j *SystemLogCleanupJob) DisplayName() string {
	return "System Log Cleaner"
}

func (j *SystemLogCleanupJob) Run(ctx context.Context) error {
	// 1. Task Logs
	if days, ok := j.getRetentionDays("task_config", "retention_days"); ok && days > 0 {
		before := time.Now().AddDate(0, 0, -days)
		// 注意: 我们之前 TaskLogRepository.DeleteBefore 使用的是 gorm.DeletedAt，
		// 这里由于我们改成了通用 time.Time 处理，稍后我会去同步修改 repository 的参数类型以保持一致。
		if err := j.taskLogRepo.DeleteBefore(ctx, before); err != nil {
			log.Printf("[Cleaner] Clean task logs failed: %v", err)
		} else {
			log.Printf("[Cleaner] Clean task logs older than %d days success", days)
		}
	}

	// 2. Ops Logs
	if days, ok := j.getRetentionDays("ops_config", "retention_days"); ok && days > 0 {
		before := time.Now().AddDate(0, 0, -days)
		if err := j.opsLogRepo.DeleteBefore(ctx, before); err != nil {
			log.Printf("[Cleaner] Clean ops logs failed: %v", err)
		} else {
			log.Printf("[Cleaner] Clean ops logs older than %d days success", days)
		}
	}

	// 3. Error Logs
	if days, ok := j.getRetentionDays("error_config", "retention_days"); ok && days > 0 {
		before := time.Now().AddDate(0, 0, -days)
		if err := j.errLogRepo.DeleteBefore(ctx, before); err != nil {
			log.Printf("[Cleaner] Clean error logs failed: %v", err)
		} else {
			log.Printf("[Cleaner] Clean error logs older than %d days success", days)
		}
	}

	return nil
}

func (j *SystemLogCleanupJob) getRetentionDays(group, key string) (int, bool) {
	val, exists := j.watcher.GetConfig(group, key)
	if !exists {
		return 0, false
	}
	days, err := strconv.Atoi(val)
	if err != nil {
		return 0, false
	}
	return days, true
}

func (j *SystemLogCleanupJob) DefaultMetadata() task.TaskMetadata {
	return task.TaskMetadata{
		Name:        j.Name(),
		DisplayName: j.DisplayName(),
		Type:        task.TypeCron,
		Spec:        "0 0 2 * * *", // 每天凌晨2点执行
		Weight:      task.WeightNormal,
		Enabled:     true,
	}
}
