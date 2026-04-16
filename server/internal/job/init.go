package job

import (
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/task"
	contentRepo "NetyAdmin/internal/repository/content"
	logRepo "NetyAdmin/internal/repository/log"
	taskRepoPkg "NetyAdmin/internal/repository/task"
)

// AllJobs 任务初始化中心：在这里聚合所有任务，实现“一站式”加载。
// 每当新增一个任务文件，只需在此列表中添加一行即可，无需修改 wire.go。
func AllJobs(
	articleRepo contentRepo.ContentArticleRepository,
	taskLogRepo taskRepoPkg.TaskLogRepository,
	opsLogRepo *logRepo.OperationRepository,
	errLogRepo *logRepo.ErrorRepository,
	watcher configsync.ConfigWatcher,
) []task.Task {
	return []task.Task{
		NewArticlePublishJob(articleRepo),                                    // 文章定时发布任务 (业务级)
		NewSystemLogCleanupJob(taskLogRepo, opsLogRepo, errLogRepo, watcher), // 日志清理任务 (运维级)
	}
}
