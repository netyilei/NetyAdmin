package job

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/task"
	logRepo "NetyAdmin/internal/repository/log"
	msgRepo "NetyAdmin/internal/repository/message"
	openRepo "NetyAdmin/internal/repository/open_platform"
	taskRepoPkg "NetyAdmin/internal/repository/task"
)

// SystemLogCleanupJob 统一日志清理任务
type SystemLogCleanupJob struct {
	taskLogRepo taskRepoPkg.TaskLogRepository
	opsLogRepo  *logRepo.OperationRepository
	errLogRepo  *logRepo.ErrorRepository
	msgRepo     msgRepo.MsgRepository
	openLogRepo openRepo.OpenLogRepository
	watcher     configsync.ConfigWatcher
}

func NewSystemLogCleanupJob(
	taskLogRepo taskRepoPkg.TaskLogRepository,
	opsLogRepo *logRepo.OperationRepository,
	errLogRepo *logRepo.ErrorRepository,
	msgRepo msgRepo.MsgRepository,
	openLogRepo openRepo.OpenLogRepository,
	watcher configsync.ConfigWatcher,
) *SystemLogCleanupJob {
	return &SystemLogCleanupJob{
		taskLogRepo: taskLogRepo,
		opsLogRepo:  opsLogRepo,
		errLogRepo:  errLogRepo,
		msgRepo:     msgRepo,
		openLogRepo: openLogRepo,
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

	// 4. Message Records
	if days, ok := j.getRetentionDays("msg_record_config", "retention_days"); ok && days > 0 {
		before := time.Now().AddDate(0, 0, -days)
		if err := j.msgRepo.DeleteRecordsBefore(ctx, before); err != nil {
			log.Printf("[Cleaner] Clean message records failed: %v", err)
		} else {
			log.Printf("[Cleaner] Clean message records older than %d days success", days)
		}
	}

	// 5. Open Platform Logs
	if days, ok := j.getRetentionDays("open_platform_config", "log_retention_days"); ok && days > 0 {
		if err := j.openLogRepo.Clear(ctx, days); err != nil {
			log.Printf("[Cleaner] Clean open platform logs failed: %v", err)
		} else {
			log.Printf("[Cleaner] Clean open platform logs older than %d days success", days)
		}
	}

	return nil
}

func (j *SystemLogCleanupJob) Execute(ctx context.Context, payload json.RawMessage) error {
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
		Spec:        "0 0 2 * * *",  // 每天凌晨2点执行
		Weight:      task.WeightLow, // 10 (低优先级清理任务)
		Enabled:     true,
	}
}
