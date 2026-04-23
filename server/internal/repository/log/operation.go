package log

import (
	"context"
	"time"

	logEntity "NetyAdmin/internal/domain/entity/log"
	"NetyAdmin/internal/pkg/pagination"

	"gorm.io/gorm"
)

type OperationQuery struct {
	AdminID   uint
	Action    string
	StartDate string
	EndDate   string
	Page      int
	PageSize  int
}

type OperationRepository struct {
	db *gorm.DB
}

func NewOperationRepository(db *gorm.DB) *OperationRepository {
	return &OperationRepository{db: db}
}

func (r *OperationRepository) BatchCreate(ctx context.Context, logs []*logEntity.Operation) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

func (r *OperationRepository) List(ctx context.Context, req *OperationQuery) ([]logEntity.Operation, int64, error) {
	var logs []logEntity.Operation
	var total int64

	query := r.db.WithContext(ctx).Model(&logEntity.Operation{})

	if req.AdminID != 0 {
		query = query.Where("admin_id = ?", req.AdminID)
	}

	if req.Action != "" {
		query = query.Where("action LIKE ?", "%"+req.Action+"%")
	}

	if req.StartDate != "" {
		if startTime, err := time.Parse("2006-01-02", req.StartDate); err == nil {
			query = query.Where("created_at >= ?", startTime)
		}
	}

	if req.EndDate != "" {
		if endTime, err := time.Parse("2006-01-02", req.EndDate); err == nil {
			query = query.Where("created_at <= ?", endTime.Add(24*time.Hour))
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").Scopes(pagination.Paginate(req.Page, req.PageSize)).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *OperationRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&logEntity.Operation{}, id).Error
}

func (r *OperationRepository) DeleteBatch(ctx context.Context, ids []uint) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&logEntity.Operation{}, ids).Error
}

func (r *OperationRepository) DeleteBefore(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).Unscoped().Where("created_at < ?", before).Delete(&logEntity.Operation{}).Error
}
