package task

import (
	taskEntity "NetyAdmin/internal/domain/entity/task"
	"context"
	"time"

	"gorm.io/gorm"
)

type TaskLogRepository interface {
	Create(ctx context.Context, log *taskEntity.TaskLog) error
	BatchCreate(ctx context.Context, logs []*taskEntity.TaskLog) error
	List(ctx context.Context, name string, page, size int) ([]*taskEntity.TaskLog, int64, error)
	GetLatest(ctx context.Context, name string) (*taskEntity.TaskLog, error)
	DeleteBefore(ctx context.Context, before time.Time) error
}

type taskLogRepository struct {
	db *gorm.DB
}

func NewTaskLogRepository(db *gorm.DB) TaskLogRepository {
	return &taskLogRepository{db: db}
}

func (r *taskLogRepository) Create(ctx context.Context, log *taskEntity.TaskLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *taskLogRepository) BatchCreate(ctx context.Context, logs []*taskEntity.TaskLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

func (r *taskLogRepository) List(ctx context.Context, name string, page, size int) ([]*taskEntity.TaskLog, int64, error) {
	var logs []*taskEntity.TaskLog
	var total int64

	db := r.db.WithContext(ctx).Model(&taskEntity.TaskLog{})
	if name != "" {
		db = db.Where("name = ?", name)
	}

	// 先统计总数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 再执行分页查询
	err := db.Order("id DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&logs).Error

	return logs, total, err
}

func (r *taskLogRepository) GetLatest(ctx context.Context, name string) (*taskEntity.TaskLog, error) {
	var log taskEntity.TaskLog
	err := r.db.WithContext(ctx).Where("name = ?", name).Order("id DESC").First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *taskLogRepository) DeleteBefore(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).Unscoped().Where("created_at < ?", before).Delete(&taskEntity.TaskLog{}).Error
}
