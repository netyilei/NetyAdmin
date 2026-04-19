package open_platform

import "time"

type AppScopeGroup struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string    `gorm:"size:50;not null;uniqueIndex:idx_scope_group_code,where:deleted_at = 0" json:"code"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Status      int       `gorm:"default:1" json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	DeletedAt   int64     `gorm:"default:0" json:"-"`
}

func (AppScopeGroup) TableName() string {
	return "sys_app_scope_groups"
}
