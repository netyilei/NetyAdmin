package open_platform

import "time"

type OpenApi struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Method      string    `gorm:"size:10;not null" json:"method"`
	Path        string    `gorm:"size:255;not null" json:"path"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Group       string    `gorm:"size:50;not null" json:"group"`
	Description string    `gorm:"type:text" json:"description"`
	Status      int       `gorm:"default:1" json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	DeletedAt   int64     `gorm:"default:0" json:"-"`
}

func (OpenApi) TableName() string {
	return "sys_open_apis"
}

type ScopeApi struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	ScopeID   uint64 `gorm:"not null;uniqueIndex:idx_scope_api,where:deleted_at = 0" json:"scopeId"`
	ApiID     uint64 `gorm:"not null;uniqueIndex:idx_scope_api,where:deleted_at = 0" json:"apiId"`
	DeletedAt int64  `gorm:"default:0" json:"-"`
}

func (ScopeApi) TableName() string {
	return "sys_scope_apis"
}
