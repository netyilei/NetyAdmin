package system

import "silentorder/internal/domain/entity"

type Button struct {
	entity.Model
	entity.Operator
	MenuID uint   `gorm:"column:menu_id;not null" json:"menuId"`
	Label  string `gorm:"column:label;size:50;not null" json:"label"`
	Code   string `gorm:"column:code;size:100;not null" json:"code"`
}

func (Button) TableName() string {
	return "admin_button"
}
