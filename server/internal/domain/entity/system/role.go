package system

import "netyadmin/internal/domain/entity"

type Role struct {
	entity.Model
	entity.Operator
	Name          string    `gorm:"column:name;size:50;not null" json:"name"`
	Code          string    `gorm:"column:code;size:50;not null;uniqueIndex" json:"code"`
	Description   string    `gorm:"column:description;size:200" json:"description"`
	Status        string    `gorm:"column:status;size:1;default:1" json:"status"`
	HomeMenuID    uint      `gorm:"column:home_menu_id" json:"homeMenuId"`
	HomeMenu      *Menu     `gorm:"foreignKey:HomeMenuID" json:"homeMenu"`
	Menus         []*Menu   `gorm:"many2many:admin_role_menus;joinForeignKey:admin_role_id;joinReferences:admin_menu_id" json:"menus"`
	Buttons       []*Button `gorm:"many2many:admin_role_buttons;joinForeignKey:admin_role_id;joinReferences:admin_button_id" json:"buttons"`
	Apis          []*API    `gorm:"many2many:admin_role_apis;joinForeignKey:admin_role_id;joinReferences:admin_api_id" json:"apis"`
	CreatedByUser *Admin    `gorm:"foreignKey:CreatedBy;references:ID" json:"createdByUser"`
	UpdatedByUser *Admin    `gorm:"foreignKey:UpdatedBy;references:ID" json:"updatedByUser"`
}

func (Role) TableName() string {
	return "admin_role"
}

func (r *Role) CreatorName() string {
	if r.CreatedByUser != nil {
		return r.CreatedByUser.Nickname
	}
	return ""
}

func (r *Role) UpdaterName() string {
	if r.UpdatedByUser != nil {
		return r.UpdatedByUser.Nickname
	}
	return ""
}

const (
	RoleStatusEnabled  = "1"
	RoleStatusDisabled = "0"
	SuperRoleCode      = "R_SUPER"
)
