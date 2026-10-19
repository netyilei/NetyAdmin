package log

import (
	"context"
	"time"

	"NetyAdmin/internal/domain/entity"
	logEntity "NetyAdmin/internal/domain/entity/log"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/like"
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
		query = query.Where("action LIKE ?", like.LikeContains(req.Action))
	}

	query = applyTimeRange(query, req.StartDate, req.EndDate)

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
	_, err := database.DeleteBatchBefore(ctx, r.getDB(ctx), "admin_operation_log", before, 1000)
	return err
}

// applyTimeRange 日志时间范围过滤（同包 operation/error 共用，YYYY-MM-DD 含当日）。
// 非法格式静默忽略（与历史行为一致，不因查询参数报错）；
// endDate 上界取次日 0 点实现"含当日"（timestamptz 语义下与本地/UTC 解析一致性同 operation 历史）。
func applyTimeRange(db *gorm.DB, startDate, endDate string) *gorm.DB {
	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			db = db.Where("created_at >= ?", t)
		}
	}
	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			db = db.Where("created_at <= ?", t.Add(24*time.Hour))
		}
	}
	return db
}
