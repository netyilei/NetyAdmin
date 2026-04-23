package open_platform

import (
	"context"
	"time"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity/open_platform"
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

func (r *openLogRepository) Create(ctx context.Context, log *open_platform.OpenPlatformLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *openLogRepository) BatchCreate(ctx context.Context, logs []*open_platform.OpenPlatformLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

func (r *openLogRepository) List(ctx context.Context, query *OpenLogRepoQuery) ([]*open_platform.OpenPlatformLog, int64, error) {
	var list []*open_platform.OpenPlatformLog
	var total int64
	db := r.db.WithContext(ctx).Model(&open_platform.OpenPlatformLog{})

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

	if query.Page > 0 && query.PageSize > 0 {
		db = db.Scopes(pagination.Paginate(query.Page, query.PageSize))
	}

	err := db.Order("created_at DESC").Find(&list).Error
	return list, total, err
}

func (r *openLogRepository) GetByID(ctx context.Context, id uint64) (*open_platform.OpenPlatformLog, error) {
	var log open_platform.OpenPlatformLog
	err := r.db.WithContext(ctx).First(&log, id).Error
	return &log, err
}

func (r *openLogRepository) DeleteBatch(ctx context.Context, ids []uint64) error {
	return r.db.WithContext(ctx).Delete(&open_platform.OpenPlatformLog{}, ids).Error
}

func (r *openLogRepository) Clear(ctx context.Context, days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	return r.db.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&open_platform.OpenPlatformLog{}).Error
}
