package storage

import (
	"context"

	"gorm.io/gorm"

	storageEntity "netyadmin/internal/domain/entity/storage"
)

type ConfigRepository interface {
	Create(ctx context.Context, config *storageEntity.Config) error
	Update(ctx context.Context, config *storageEntity.Config) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*storageEntity.Config, error)
	List(ctx context.Context, query *ConfigQuery) ([]*storageEntity.Config, int64, error)
	GetAllEnabled(ctx context.Context) ([]*storageEntity.Config, error)
	GetDefault(ctx context.Context) (*storageEntity.Config, error)
	SetDefault(ctx context.Context, id uint) error
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

func (r *configRepository) Create(ctx context.Context, config *storageEntity.Config) error {
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *configRepository) Update(ctx context.Context, config *storageEntity.Config) error {
	return r.db.WithContext(ctx).Save(config).Error
}

func (r *configRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&storageEntity.Config{}, id).Error
}

func (r *configRepository) GetByID(ctx context.Context, id uint) (*storageEntity.Config, error) {
	var config storageEntity.Config
	err := r.db.WithContext(ctx).First(&config, id).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *configRepository) List(ctx context.Context, query *ConfigQuery) ([]*storageEntity.Config, int64, error) {
	db := r.db.WithContext(ctx).Model(&storageEntity.Config{})

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var configs []*storageEntity.Config
	offset := (query.Current - 1) * query.Size
	if offset < 0 {
		offset = 0
	}
	if query.Size <= 0 {
		query.Size = 10
	}

	err := db.Order("is_default DESC, created_at DESC").
		Offset(offset).
		Limit(query.Size).
		Find(&configs).Error
	if err != nil {
		return nil, 0, err
	}

	return configs, total, nil
}

func (r *configRepository) GetAllEnabled(ctx context.Context) ([]*storageEntity.Config, error) {
	var configs []*storageEntity.Config
	err := r.db.WithContext(ctx).
		Where("status = ?", "1").
		Order("is_default DESC, created_at DESC").
		Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

func (r *configRepository) GetDefault(ctx context.Context) (*storageEntity.Config, error) {
	var config storageEntity.Config
	err := r.db.WithContext(ctx).
		Where("is_default = ? AND status = ?", true, "1").
		First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *configRepository) SetDefault(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&storageEntity.Config{}).
			Where("is_default = ?", true).
			Update("is_default", false).Error; err != nil {
			return err
		}

		return tx.Model(&storageEntity.Config{}).
			Where("id = ?", id).
			Update("is_default", true).Error
	})
}

func (r *configRepository) ExistsByName(ctx context.Context, name string, excludeID ...uint) (bool, error) {
	db := r.db.WithContext(ctx).Model(&storageEntity.Config{}).Where("name = ?", name)
	if len(excludeID) > 0 {
		db = db.Where("id != ?", excludeID[0])
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
