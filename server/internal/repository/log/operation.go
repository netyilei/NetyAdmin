package log

import (
	"context"
	"time"

	"NetyAdmin/internal/domain/entity"
	logEntity "NetyAdmin/internal/domain/entity/log"
	"NetyAdmin/internal/pkg/database"
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

// getDB 根据 context 中是否携带事务，返回事务内的 *gorm.DB 或回退到 r.db
func (r *OperationRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *OperationRepository) BatchCreate(ctx context.Context, logs []*logEntity.Operation) error {
	if len(logs) == 0 {
		return nil
	}
	return r.getDB(ctx).Create(&logs).Error
}

func (r *OperationRepository) List(ctx context.Context, req *OperationQuery) ([]logEntity.Operation, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = entity.DefaultPageSize
	}

	var logs []logEntity.Operation
	var total int64

	query := r.getDB(ctx).Model(&logEntity.Operation{})

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
	return r.getDB(ctx).Unscoped().Delete(&logEntity.Operation{}, id).Error
}

func (r *OperationRepository) DeleteBatch(ctx context.Context, ids []uint) error {
	return r.getDB(ctx).Unscoped().Delete(&logEntity.Operation{}, ids).Error
}

func (r *OperationRepository) DeleteBefore(ctx context.Context, before time.Time) error {
	return r.getDB(ctx).Unscoped().Where("created_at < ?", before).Delete(&logEntity.Operation{}).Error
}
