package system

import (
	"context"
	"errors"

	"gorm.io/gorm"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/database"
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

// getDB 取当前 context 下的 *gorm.DB：若 ctx 中存在事务句柄则复用事务，否则回退到 r.db。
func (r *configRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *configRepository) GetByGroupAndKey(ctx context.Context, groupName, configKey string) (*systemEntity.SysConfig, error) {
	var config systemEntity.SysConfig
	err := r.getDB(ctx).
		Where("group_name = ? AND config_key = ?", groupName, configKey).
		First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *configRepository) GetByGroup(ctx context.Context, groupName string) ([]*systemEntity.SysConfig, error) {
	var configs []*systemEntity.SysConfig
	err := r.getDB(ctx).
		Where("group_name = ?", groupName).
		Find(&configs).Error
	return configs, err
}

func (r *configRepository) GetAll(ctx context.Context) ([]*systemEntity.SysConfig, error) {
	var configs []*systemEntity.SysConfig
	err := r.getDB(ctx).Find(&configs).Error
	return configs, err
}

func (r *configRepository) Upsert(ctx context.Context, config *systemEntity.SysConfig) error {
	var existing systemEntity.SysConfig
	err := r.getDB(ctx).
		Where("group_name = ? AND config_key = ?", config.GroupName, config.ConfigKey).
		First(&existing).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.getDB(ctx).Create(config).Error
		}
		return err
	}

	config.ID = existing.ID
	return r.getDB(ctx).Model(&existing).Updates(map[string]interface{}{
		"config_value": config.ConfigValue,
		"value_type":   config.ValueType,
		"description":  config.Description,
		"updated_by":   config.UpdatedBy,
	}).Error
}

func (r *configRepository) Delete(ctx context.Context, id uint) error {
	return r.getDB(ctx).Unscoped().Delete(&systemEntity.SysConfig{}, id).Error
}
