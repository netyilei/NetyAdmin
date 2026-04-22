package log

import (
	"NetyAdmin/internal/domain/entity"
	"time"
)

type Operation struct {
	entity.Model
	AdminID   uint   `gorm:"column:admin_id;not null;comment:操作人ID" json:"adminId"`
	Username  string `gorm:"column:username;size:50;not null;comment:操作人名称" json:"username"`
	Action    string `gorm:"column:action;size:100;not null;comment:操作动作" json:"action"`
	Resource  string `gorm:"column:resource;size:200;not null;comment:操作资源" json:"resource"`
	Detail    string `gorm:"column:detail;type:text;comment:操作详情" json:"detail"`
	IP        string `gorm:"column:ip;size:50;comment:IP地址" json:"ip"`
	UserAgent string `gorm:"column:user_agent;size:500;comment:User-Agent" json:"userAgent"`
}

func (Operation) TableName() string {
	return "admin_operation_log"
}

func (o *Operation) GetLogType() LogType {
	return LogTypeOperation
}

func (o *Operation) GetCreatedAt() time.Time {
	return o.CreatedAt
}
