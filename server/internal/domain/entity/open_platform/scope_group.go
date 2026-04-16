package open_platform

import "time"

// AppScopeGroup 权限分组实体 (动态加载 + i18n 支持)
type AppScopeGroup struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string    `gorm:"size:50;not null;uniqueIndex:idx_scope_group_code,where:deleted_at = 0" json:"code"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	I18nKey     string    `gorm:"size:100;not null" json:"i18nKey"`
	Description string    `gorm:"type:text" json:"description"`
	Status      int       `gorm:"default:1" json:"status"` // 1: 启用, 0: 禁用
	CreatedAt   time.Time `json:"createdAt"`
	DeletedAt   int64     `gorm:"default:0" json:"-"`
}

func (AppScopeGroup) TableName() string {
	return "sys_app_scope_groups"
}
