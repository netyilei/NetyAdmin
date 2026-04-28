package log

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	logEntity "NetyAdmin/internal/domain/entity/log"
	"NetyAdmin/internal/pkg/pagination"
)

type ErrorRepository struct {
	db *gorm.DB
}

func NewErrorRepository(db *gorm.DB) *ErrorRepository {
	return &ErrorRepository{db: db}
}

func (r *ErrorRepository) UpsertByHash(ctx context.Context, logRecord *logEntity.Error) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "hash"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"occurrence_count": gorm.Expr("admin_error_log.occurrence_count + ?", 1),
			"last_occurred_at": time.Now(),
			"request_id":       logRecord.RequestID,
			"ip":               logRecord.IP,
		}),
	}).Create(logRecord).Error
}

func (r *ErrorRepository) BatchUpsertByHash(ctx context.Context, logs []*logEntity.Error) error {
	if len(logs) == 0 {
		return nil
	}
	for _, l := range logs {
		if err := r.UpsertByHash(ctx, l); err != nil {
			return err
		}
	}
	return nil
}

func (r *ErrorRepository) List(ctx context.Context, level string, resolved *bool, page, pageSize int) ([]logEntity.Error, int64, error) {
	var logs []logEntity.Error
	var total int64

	query := r.db.WithContext(ctx).Model(&logEntity.Error{})

	if level != "" {
		query = query.Where("level = ?", level)
	}

	if resolved != nil {
		query = query.Where("resolved = ?", *resolved)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").Scopes(pagination.Paginate(page, pageSize)).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *ErrorRepository) Resolve(ctx context.Context, id, resolvedBy uint) error {
	return r.db.WithContext(ctx).Model(&logEntity.Error{}).Where("id = ?", id).Updates(map[string]interface{}{
		"resolved":    true,
		"resolved_by": resolvedBy,
	}).Error
}

func (r *ErrorRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&logEntity.Error{}, id).Error
}

func (r *ErrorRepository) DeleteBatch(ctx context.Context, ids []uint) error {
	return r.db.WithContext(ctx).Delete(&logEntity.Error{}, ids).Error
}

func (r *ErrorRepository) DeleteBefore(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).Unscoped().Where("created_at < ?", before).Delete(&logEntity.Error{}).Error
}
