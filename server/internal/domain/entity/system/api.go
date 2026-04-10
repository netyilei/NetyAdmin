package system

import "NetyAdmin/internal/domain/entity"

// API 表 admin_api 无 created_by/updated_by 列，不嵌入 entity.Operator
type API struct {
	entity.Model
	MenuID      uint   `gorm:"column:menu_id;not null" json:"menuId"`
	Menu        *Menu  `gorm:"foreignKey:MenuID" json:"menu"`
	Name        string `gorm:"column:name;size:100;not null" json:"name"`
	Path        string `gorm:"column:path;size:200;not null" json:"path"`
	Method      string `gorm:"column:method;size:10;not null" json:"method"`
	Auth        string `gorm:"column:auth;size:1;default:1" json:"auth"`
	Description string `gorm:"column:description;size:200" json:"description"`
}

func (API) TableName() string {
	return "admin_api"
}

const (
	APINotRequireAuth = "0"
	APIRequireAuth    = "1"
)
