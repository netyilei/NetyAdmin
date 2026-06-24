package storage

import (
	"context"
	"time"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	storageEntity "NetyAdmin/internal/domain/entity/storage"
	"NetyAdmin/internal/pkg/pagination"
)

type RecordRepository interface {
	Create(ctx context.Context, record *storageEntity.Record) error
	Update(ctx context.Context, record *storageEntity.Record) error
	UpdateSecret(ctx context.Context, id uint, secret string) error
	Delete(ctx context.Context, id uint) error
	DeleteMultiple(ctx context.Context, ids []uint) error
	GetByID(ctx context.Context, id uint) (*storageEntity.Record, error)
	// MarkUploaded 在事务内加行锁读取 + 条件翻转状态。
	// 仅当当前 status=pending 时才更新为 uploaded，返回 updated 表示是否真正翻转。
	// 行锁 + 条件更新保证并发提交只有一个成功，避免 TOCTOU。
	MarkUploaded(ctx context.Context, id uint, fileSize int64, md5, mimeType, fileURL string) (updated bool, err error)
	CleanupExpiredPending(ctx context.Context, before time.Time) (int64, error)
	GetByMD5(ctx context.Context, md5 string) (*storageEntity.Record, error)
	List(ctx context.Context, query *RecordQuery) ([]*storageEntity.Record, int64, error)
	GetByStorageConfigID(ctx context.Context, configID uint) ([]*storageEntity.Record, error)
	GetBySource(ctx context.Context, source storageEntity.UploadSource, sourceID string) ([]*storageEntity.Record, error)
	GetByBusiness(ctx context.Context, businessType string, businessID string) ([]*storageEntity.Record, error)
}

type RecordQuery struct {
	FileName        string
	Source          string
	SourceID        string
	BusinessType    string
	BusinessID      string
	MimeType        string
	StorageConfigID uint
	AppID           string
	StartTime       string
	EndTime         string
	Current         int
	Size            int
}

type recordRepository struct {
	db *gorm.DB
}

func NewRecordRepository(db *gorm.DB) RecordRepository {
	return &recordRepository{db: db}
}

func (r *recordRepository) Create(ctx context.Context, record *storageEntity.Record) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *recordRepository) Update(ctx context.Context, record *storageEntity.Record) error {
	return r.db.WithContext(ctx).Save(record).Error
}

// UpdateSecret 仅更新 secret 字段。
// 注意不能用 Save（会全列覆盖），recordID 在 Create 后才生成，签名依赖 ID 故需二次写。
func (r *recordRepository) UpdateSecret(ctx context.Context, id uint, secret string) error {
	return r.db.WithContext(ctx).Model(&storageEntity.Record{}).
		Where("id = ?", id).
		Update("secret", secret).Error
}

func (r *recordRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&storageEntity.Record{}, id).Error
}

func (r *recordRepository) DeleteMultiple(ctx context.Context, ids []uint) error {
	return r.db.WithContext(ctx).Delete(&storageEntity.Record{}, ids).Error
}

func (r *recordRepository) GetByID(ctx context.Context, id uint) (*storageEntity.Record, error) {
	var record storageEntity.Record
	err := r.db.WithContext(ctx).
		Preload("StorageConfig").
		First(&record, id).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// MarkUploaded 在单个事务内完成：行锁读取当前状态 + 仅当 status=pending 时翻转。
// 返回 updated=true 表示本次成功翻转；false 表示并发竞争或状态已变（调用方据此返回错误）。
func (r *recordRepository) MarkUploaded(ctx context.Context, id uint, fileSize int64, md5, mimeType, fileURL string) (bool, error) {
	var updated bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 行锁读取 id + status（SELECT 必须包含 id 才能正确判断记录存在性）
		var record storageEntity.Record
		if err := tx.
			Raw("SELECT id, status FROM upload_record WHERE id = ? AND deleted_at = 0 FOR UPDATE", id).
			Scan(&record).Error; err != nil {
			return err
		}
		if record.ID == 0 {
			return gorm.ErrRecordNotFound
		}
		// 2. 仅 pending 可翻转
		if record.Status != storageEntity.RecordStatusPending {
			return nil
		}

		updates := map[string]interface{}{
			"status":      storageEntity.RecordStatusUploaded,
			"uploaded_at": time.Now(),
		}
		if fileSize > 0 {
			updates["file_size"] = fileSize
		}
		if md5 != "" {
			updates["md5"] = md5
		}
		if mimeType != "" {
			updates["mime_type"] = mimeType
		}
		if fileURL != "" {
			updates["file_url"] = fileURL
		}
		res := tx.Model(&storageEntity.Record{}).Where("id = ?", id).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		updated = res.RowsAffected > 0
		return nil
	})
	return updated, err
}

// CleanupExpiredPending 将超期未通知的 pending 记录标记为 expired，返回受影响行数。
func (r *recordRepository) CleanupExpiredPending(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&storageEntity.Record{}).
		Where("status = ? AND expires_at IS NOT NULL AND expires_at < ?",
			storageEntity.RecordStatusPending, before).
		Update("status", storageEntity.RecordStatusExpired)
	return res.RowsAffected, res.Error
}

func (r *recordRepository) GetByMD5(ctx context.Context, md5 string) (*storageEntity.Record, error) {
	var record storageEntity.Record
	err := r.db.WithContext(ctx).
		Where("md5 = ?", md5).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *recordRepository) List(ctx context.Context, query *RecordQuery) ([]*storageEntity.Record, int64, error) {
	db := r.db.WithContext(ctx).Model(&storageEntity.Record{}).Preload("StorageConfig")

	if query.FileName != "" {
		db = db.Where("file_name LIKE ?", "%"+query.FileName+"%")
	}
	if query.Source != "" {
		db = db.Where("source = ?", query.Source)
	}
	if query.SourceID != "" {
		db = db.Where("source_id = ?", query.SourceID)
	}
	if query.BusinessType != "" {
		db = db.Where("business_type = ?", query.BusinessType)
	}
	if query.BusinessID != "" {
		db = db.Where("business_id = ?", query.BusinessID)
	}
	if query.MimeType != "" {
		db = db.Where("mime_type LIKE ?", query.MimeType+"%")
	}
	if query.StorageConfigID > 0 {
		db = db.Where("storage_config_id = ?", query.StorageConfigID)
	}
	if query.AppID != "" {
		db = db.Where("app_id = ?", query.AppID)
	}
	if query.StartTime != "" {
		if t, err := time.Parse("2006-01-02", query.StartTime); err == nil {
			db = db.Where("uploaded_at >= ?", t)
		}
	}
	if query.EndTime != "" {
		if t, err := time.Parse("2006-01-02", query.EndTime); err == nil {
			db = db.Where("uploaded_at <= ?", t.Add(24*time.Hour))
		}
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []*storageEntity.Record
	if query.Size <= 0 {
		query.Size = entity.DefaultPageSize
	}

	err := db.Order("uploaded_at DESC").
		Scopes(pagination.Paginate(query.Current, query.Size)).
		Find(&records).Error
	if err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

func (r *recordRepository) GetByStorageConfigID(ctx context.Context, configID uint) ([]*storageEntity.Record, error) {
	var records []*storageEntity.Record
	err := r.db.WithContext(ctx).
		Where("storage_config_id = ?", configID).
		Order("uploaded_at DESC").
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (r *recordRepository) GetBySource(ctx context.Context, source storageEntity.UploadSource, sourceID string) ([]*storageEntity.Record, error) {
	var records []*storageEntity.Record
	db := r.db.WithContext(ctx).Where("source = ?", source)
	if sourceID != "" {
		db = db.Where("source_id = ?", sourceID)
	}
	err := db.Order("uploaded_at DESC").Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (r *recordRepository) GetByBusiness(ctx context.Context, businessType string, businessID string) ([]*storageEntity.Record, error) {
	var records []*storageEntity.Record
	db := r.db.WithContext(ctx).Where("business_type = ?", businessType)
	if businessID != "" {
		db = db.Where("business_id = ?", businessID)
	}
	err := db.Order("uploaded_at DESC").Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}
