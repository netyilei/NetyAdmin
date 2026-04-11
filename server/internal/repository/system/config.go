package system

import (
	"context"
	"errors"

	"gorm.io/gorm"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/errorx"
)

type ConfigRepository interface {
	GetByGroupAndKey(ctx context.Context, groupName, configKey string) (*systemEntity.SysConfig, error)
	GetByGroup(ctx context.Context, groupName string) ([]*systemEntity.SysConfig, error)
	GetAll(ctx context.Context) ([]*systemEntity.SysConfig, error)
	Upsert(ctx context.Context, config *systemEntity.SysConfig) error
	Delete(ctx context.Context, id uint) error
}

type configRepository struct {
	db *gorm.DB
}

func NewConfigRepository(db *gorm.DB) ConfigRepository {
	return &configRepository{db: db}
}

func (r *configRepository) GetByGroupAndKey(ctx context.Context, groupName, configKey string) (*systemEntity.SysConfig, error) {
	var config systemEntity.SysConfig
	err := r.db.WithContext(ctx).
		Where("group_name = ? AND config_key = ?", groupName, configKey).
		First(&config).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeNotFound, "配置不存在")
		}
		return nil, err
	}
	return &config, nil
}

func (r *configRepository) GetByGroup(ctx context.Context, groupName string) ([]*systemEntity.SysConfig, error) {
	var configs []*systemEntity.SysConfig
	err := r.db.WithContext(ctx).
		Where("group_name = ?", groupName).
		Find(&configs).Error
	return configs, err
}

func (r *configRepository) GetAll(ctx context.Context) ([]*systemEntity.SysConfig, error) {
	var configs []*systemEntity.SysConfig
	err := r.db.WithContext(ctx).Find(&configs).Error
	return configs, err
}

func (r *configRepository) Upsert(ctx context.Context, config *systemEntity.SysConfig) error {
	var existing systemEntity.SysConfig
	err := r.db.WithContext(ctx).
		Where("group_name = ? AND config_key = ?", config.GroupName, config.ConfigKey).
		First(&existing).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.db.WithContext(ctx).Create(config).Error
		}
		return err
	}

	config.ID = existing.ID
	return r.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
		"config_value": config.ConfigValue,
		"value_type":   config.ValueType,
		"description":  config.Description,
		"updated_by":   config.UpdatedBy,
	}).Error
}

func (r *configRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&systemEntity.SysConfig{}, id).Error
}
