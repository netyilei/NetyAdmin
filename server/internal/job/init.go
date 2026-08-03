package job

import (
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/task"
	contentRepo "NetyAdmin/internal/repository/content"
	logRepo "NetyAdmin/internal/repository/log"
	msgRepo "NetyAdmin/internal/repository/message"
	openRepo "NetyAdmin/internal/repository/open_platform"
	storageRepo "NetyAdmin/internal/repository/storage"
	taskRepoPkg "NetyAdmin/internal/repository/task"
	userRepo "NetyAdmin/internal/repository/user"
)

func AllJobs(
	articleRepo contentRepo.ContentArticleRepository,
	taskLogRepo taskRepoPkg.TaskLogRepository,
	opsLogRepo *logRepo.OperationRepository,
	errLogRepo *logRepo.ErrorRepository,
	msgRepository msgRepo.MsgRepository,
	openLogRepo openRepo.OpenLogRepository,
	uploadRecordRepo storageRepo.RecordRepository,
	userTokenRepo userRepo.UserTokenRepository,
	watcher configsync.ConfigWatcher,
) []task.Task {
	return []task.Task{
		NewArticlePublishJob(articleRepo), // 文章定时发布任务 (业务级)
		NewSystemLogCleanupJob(taskLogRepo, opsLogRepo, errLogRepo, msgRepository, openLogRepo, watcher), // 日志清理任务 (运维级)
		NewUploadRecordCleanupJob(uploadRecordRepo),                                                      // 上传记录过期清理任务 (运维级)
		NewTokenHashCleanupJob(userTokenRepo),                                                            // user_tokens 过期清理任务 (运维级)
	}
}
