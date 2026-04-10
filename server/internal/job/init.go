package job

import (
	"netyadmin/internal/pkg/configsync"
	"netyadmin/internal/pkg/migration"
	"netyadmin/internal/pkg/task"
	contentRepo "netyadmin/internal/repository/content"
	logRepo "netyadmin/internal/repository/log"
	systemRepo "netyadmin/internal/repository/system"
)

// AllJobs 任务初始化中心：在这里聚合所有任务，实现“一站式”加载。
// 每当新增一个任务文件，只需在此列表中添加一行即可，无需修改 wire.go。
func AllJobs(
	migrator *migration.Migrator,
	articleRepo contentRepo.ContentArticleRepository,
	taskLogRepo systemRepo.TaskLogRepository,
	opsLogRepo *logRepo.OperationRepository,
	errLogRepo *logRepo.ErrorRepository,
	watcher configsync.ConfigWatcher,
) []task.Task {
	return []task.Task{
		NewDBMigrationJob(migrator),                                          // 数据库迁移任务 (系统级)
		NewArticlePublishJob(articleRepo),                                    // 文章定时发布任务 (业务级)
		NewSystemLogCleanupJob(taskLogRepo, opsLogRepo, errLogRepo, watcher), // 日志清理任务 (运维级)
	}
}
