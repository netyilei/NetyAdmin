package storage

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	storageEntity "NetyAdmin/internal/domain/entity/storage"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/pagination"
)

type ConfigRepository interface {
	Create(ctx context.Context, config *storageEntity.Config) error
	Update(ctx context.Context, config *storageEntity.Config) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*storageEntity.Config, error)
	List(ctx context.Context, query *ConfigQuery) ([]*storageEntity.Config, int64, error)
	GetAllEnabled(ctx context.Context) ([]*storageEntity.Config, error)
	GetDefault(ctx context.Context) (*storageEntity.Config, error)
	UnsetDefault(ctx context.Context) error
	SetDefaultByID(ctx context.Context, id uint) error
	ExistsByName(ctx context.Context, name string, excludeID ...uint) (bool, error)
}

type ConfigQuery struct {
	Current int
	Size    int
}

type configRepository struct {
	db *gorm.DB
}

func NewConfigRepository(db *gorm.DB) ConfigRepository {
	return &configRepository{db: db}
}

// getDB 从 context 中获取事务 DB，若不存在则回退到默认 db。
func (r *configRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *configRepository) Create(ctx context.Context, config *storageEntity.Config) error {
	return r.getDB(ctx).Create(config).Error
}

func (r *configRepository) Update(ctx context.Context, config *storageEntity.Config) error {
	return r.getDB(ctx).Save(config).Error
}

func (r *configRepository) Delete(ctx context.Context, id uint) error {
	return r.getDB(ctx).Delete(&storageEntity.Config{}, id).Error
}

func (r *configRepository) GetByID(ctx context.Context, id uint) (*storageEntity.Config, error) {
	var config storageEntity.Config
	err := r.getDB(ctx).First(&config, id).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *configRepository) List(ctx context.Context, query *ConfigQuery) ([]*storageEntity.Config, int64, error) {
	if query.Current <= 0 {
		query.Current = 1
	}
	if query.Size <= 0 {
		query.Size = entity.DefaultPageSize
	}

	db := r.getDB(ctx).Model(&storageEntity.Config{})

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var configs []*storageEntity.Config

	err := db.Order("is_default DESC, created_at DESC").
		Scopes(pagination.Paginate(query.Current, query.Size)).
		Find(&configs).Error
	if err != nil {
		return nil, 0, err
	}

	return configs, total, nil
}

func (r *configRepository) GetAllEnabled(ctx context.Context) ([]*storageEntity.Config, error) {
	var configs []*storageEntity.Config
	err := r.getDB(ctx).
		Where("status = ?", entity.StatusEnabled).
		Order("is_default DESC, created_at DESC").
		Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

func (r *configRepository) GetDefault(ctx context.Context) (*storageEntity.Config, error) {
	var config storageEntity.Config
	err := r.getDB(ctx).
		Where("is_default = ? AND status = ?", true, entity.StatusEnabled).
		First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// UnsetDefault 清除所有 storage_config 的 is_default 标记
func (r *configRepository) UnsetDefault(ctx context.Context) error {
	return r.getDB(ctx).Model(&storageEntity.Config{}).
		Where("is_default = ?", true).
		Update("is_default", false).Error
}

// SetDefaultByID 将指定 storage_config 设为默认
func (r *configRepository) SetDefaultByID(ctx context.Context, id uint) error {
	return r.getDB(ctx).Model(&storageEntity.Config{}).
		Where("id = ?", id).
		Update("is_default", true).Error
}

func (r *configRepository) ExistsByName(ctx context.Context, name string, excludeID ...uint) (bool, error) {
	db := r.getDB(ctx).Model(&storageEntity.Config{}).Where("name = ?", name)
	if len(excludeID) > 0 {
		db = db.Where("id != ?", excludeID[0])
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
