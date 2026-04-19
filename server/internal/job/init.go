package job

import (
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/task"
	contentRepo "NetyAdmin/internal/repository/content"
	logRepo "NetyAdmin/internal/repository/log"
	msgRepo "NetyAdmin/internal/repository/message"
	taskRepoPkg "NetyAdmin/internal/repository/task"
)

func AllJobs(
	articleRepo contentRepo.ContentArticleRepository,
	taskLogRepo taskRepoPkg.TaskLogRepository,
	opsLogRepo *logRepo.OperationRepository,
	errLogRepo *logRepo.ErrorRepository,
	msgRepository msgRepo.MsgRepository,
	watcher configsync.ConfigWatcher,
) []task.Task {
	return []task.Task{
		NewArticlePublishJob(articleRepo),                                                // 文章定时发布任务 (业务级)
		NewSystemLogCleanupJob(taskLogRepo, opsLogRepo, errLogRepo, msgRepository, watcher), // 日志清理任务 (运维级)
	}
}
