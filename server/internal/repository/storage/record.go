package storage

import (
	"context"
	"time"

	"gorm.io/gorm"

	storageEntity "NetyAdmin/internal/domain/entity/storage"
)

type RecordRepository interface {
	Create(ctx context.Context, record *storageEntity.Record) error
	Update(ctx context.Context, record *storageEntity.Record) error
	Delete(ctx context.Context, id uint) error
	DeleteMultiple(ctx context.Context, ids []uint) error
	GetByID(ctx context.Context, id uint) (*storageEntity.Record, error)
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
	offset := (query.Current - 1) * query.Size
	if offset < 0 {
		offset = 0
	}
	if query.Size <= 0 {
		query.Size = 10
	}

	err := db.Order("uploaded_at DESC").
		Offset(offset).
		Limit(query.Size).
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
