package open_platform

import (
	"context"
	"time"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	"NetyAdmin/internal/domain/entity/open_platform"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/pagination"
)

type OpenLogRepository interface {
	Create(ctx context.Context, log *open_platform.OpenPlatformLog) error
	BatchCreate(ctx context.Context, logs []*open_platform.OpenPlatformLog) error
	List(ctx context.Context, query *OpenLogRepoQuery) ([]*open_platform.OpenPlatformLog, int64, error)
	GetByID(ctx context.Context, id uint64) (*open_platform.OpenPlatformLog, error)
	DeleteBatch(ctx context.Context, ids []uint64) error
	Clear(ctx context.Context, days int) error
}

type OpenLogRepoQuery struct {
	Page       int
	PageSize   int
	AppID      string
	AppKey     string
	ApiPath    string
	StatusCode *int
	StartTime  string
	EndTime    string
}

type openLogRepository struct {
	db *gorm.DB
}

func NewOpenLogRepository(db *gorm.DB) OpenLogRepository {
	return &openLogRepository{db: db}
}

// getDB 根据 context 中是否携带事务，返回事务内的 *gorm.DB 或回退到 r.db
func (r *openLogRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *openLogRepository) Create(ctx context.Context, log *open_platform.OpenPlatformLog) error {
	return r.getDB(ctx).Create(log).Error
}

func (r *openLogRepository) BatchCreate(ctx context.Context, logs []*open_platform.OpenPlatformLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.getDB(ctx).Create(&logs).Error
}

func (r *openLogRepository) List(ctx context.Context, query *OpenLogRepoQuery) ([]*open_platform.OpenPlatformLog, int64, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = entity.DefaultPageSize
	}

	var list []*open_platform.OpenPlatformLog
	var total int64
	db := r.getDB(ctx).Model(&open_platform.OpenPlatformLog{})

	if query.AppID != "" {
		db = db.Where("app_id = ?", query.AppID)
	}
	if query.AppKey != "" {
		db = db.Where("app_key = ?", query.AppKey)
	}
	if query.ApiPath != "" {
		db = db.Where("api_path LIKE ?", "%"+query.ApiPath+"%")
	}
	if query.StatusCode != nil {
		db = db.Where("status_code = ?", *query.StatusCode)
	}
	if query.StartTime != "" {
		db = db.Where("created_at >= ?", query.StartTime)
	}
	if query.EndTime != "" {
		db = db.Where("created_at <= ?", query.EndTime)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := db.Order("created_at DESC").Scopes(pagination.Paginate(query.Page, query.PageSize)).Find(&list).Error
	return list, total, err
}

func (r *openLogRepository) GetByID(ctx context.Context, id uint64) (*open_platform.OpenPlatformLog, error) {
	var log open_platform.OpenPlatformLog
	if err := r.getDB(ctx).First(&log, id).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *openLogRepository) DeleteBatch(ctx context.Context, ids []uint64) error {
	return r.getDB(ctx).Delete(&open_platform.OpenPlatformLog{}, ids).Error
}

func (r *openLogRepository) Clear(ctx context.Context, days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	return r.getDB(ctx).Where("created_at < ?", cutoff).Delete(&open_platform.OpenPlatformLog{}).Error
}
