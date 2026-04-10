package system

import (
	"NetyAdmin/internal/domain/entity"
)

type SysConfig struct {
	entity.Model
	entity.Operator
	GroupName   string `gorm:"size:100;not null;uniqueIndex:idx_sys_configs_group_key;index:idx_sys_configs_group" json:"groupName"`
	ConfigKey   string `gorm:"size:100;not null;uniqueIndex:idx_sys_configs_group_key" json:"configKey"`
	ConfigValue string `gorm:"type:text;not null" json:"configValue"`
	ValueType   string `gorm:"size:50;not null;default:'string'" json:"valueType"`
	Description string `gorm:"size:255" json:"description"`
	IsSystem    bool   `gorm:"not null;default:false" json:"isSystem"`
}

func (SysConfig) TableName() string {
	return "sys_configs"
}
