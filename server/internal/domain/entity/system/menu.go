package system

import "NetyAdmin/internal/domain/entity"

type Menu struct {
	entity.Model
	entity.Operator
	ParentID        uint      `gorm:"column:parent_id;default:0" json:"parentId"`
	Type            string    `gorm:"column:type;size:1;not null" json:"type"`
	Name            string    `gorm:"column:name;size:50;not null" json:"name"`
	RouteName       string    `gorm:"column:route_name;size:100;not null" json:"routeName"`
	RoutePath       string    `gorm:"column:route_path;size:200" json:"routePath"`
	Component       string    `gorm:"column:component;size:100" json:"component"`
	I18nKey         string    `gorm:"column:i18_n_key;size:100" json:"i18nKey"`
	Icon            string    `gorm:"column:icon;size:100" json:"icon"`
	IconType        string    `gorm:"column:icon_type;size:1;default:1" json:"iconType"`
	Status          string    `gorm:"column:status;size:1;default:1" json:"status"`
	Order           int       `gorm:"column:order_by;default:0" json:"order"`
	Hidden          bool      `gorm:"column:hide_in_menu;default:false" json:"hideInMenu"`
	KeepAlive       bool      `gorm:"column:keep_alive;default:true" json:"keepAlive"`
	Constant        bool      `gorm:"column:constant;default:false" json:"constant"`
	ActiveMenu      string    `gorm:"column:active_menu;size:100" json:"activeMenu"`
	MultiTab        bool      `gorm:"column:multi_tab;default:false" json:"multiTab"`
	FixedIndexInTab *int      `gorm:"column:fixed_index_in_tab" json:"fixedIndexInTab"`
	Query           string    `gorm:"column:query" json:"query"`
	Href            string    `gorm:"column:href;size:200" json:"href"`
	Children        []*Menu   `gorm:"-" json:"children"`
	Buttons         []*Button `gorm:"foreignKey:MenuID" json:"buttons"`
	Apis            []*API    `gorm:"foreignKey:MenuID" json:"apis"`
	Roles           []*Role   `gorm:"many2many:admin_role_menus;joinForeignKey:admin_menu_id;joinReferences:admin_role_id" json:"-"`
}

func (Menu) TableName() string {
	return "admin_menu"
}

const (
	MenuTypeCatalog = "1"
	MenuTypePage    = "2"
	MenuTypeButton  = "3"

	MenuStatusEnabled  = "1"
	MenuStatusDisabled = "0"
)
