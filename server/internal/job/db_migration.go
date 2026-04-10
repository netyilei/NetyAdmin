package job

import (
	"context"
	"silentorder/internal/pkg/migration"
	"silentorder/internal/pkg/task"
)

// DBMigrationJob 数据库迁移任务
type DBMigrationJob struct {
	migrator *migration.Migrator
}

func NewDBMigrationJob(migrator *migration.Migrator) *DBMigrationJob {
	return &DBMigrationJob{migrator: migrator}
}

func (j *DBMigrationJob) Name() string {
	return "db_migration"
}

func (j *DBMigrationJob) DisplayName() string {
	return "Database Migrator"
}

func (j *DBMigrationJob) Run(ctx context.Context) error {
	return j.migrator.Run()
}

// DefaultMetadata 定义系统级高优先级权重
func (j *DBMigrationJob) DefaultMetadata() task.TaskMetadata {
	return task.TaskMetadata{
		Name:        j.Name(),
		DisplayName: j.DisplayName(),
		Type:        task.TypeOnce,
		Weight:      task.WeightSystem, // 100 - 系统级最高权重
		Enabled:     true,
	}
}
