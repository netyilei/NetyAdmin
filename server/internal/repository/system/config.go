package system

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	systemEntity "silentorder/internal/domain/entity/system"
	"silentorder/internal/pkg/errorx"
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
	// 冲突时更新值和说明等
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "group_name"}, {Name: "config_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"config_value", "value_type", "description", "updated_by", "updated_at"}),
	}).Create(config).Error
}

func (r *configRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&systemEntity.SysConfig{}, id).Error
}
