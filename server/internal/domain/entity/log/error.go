package log

import (
	"time"

	"netyadmin/internal/domain/entity"
)

type Error struct {
	entity.Model
	Level           string    `gorm:"column:level;size:20;not null;comment:日志级别" json:"level"`
	Message         string    `gorm:"column:message;type:text;not null;comment:错误信息" json:"message"`
	Stack           string    `gorm:"column:stack;type:text;comment:堆栈信息" json:"stack"`
	RequestID       string    `gorm:"column:request_id;size:50;comment:请求ID" json:"requestId"`
	Path            string    `gorm:"column:path;size:200;comment:请求路径" json:"path"`
	Method          string    `gorm:"column:method;size:10;comment:请求方法" json:"method"`
	UserID          uint      `gorm:"column:user_id;comment:用户ID" json:"userId"`
	IP              string    `gorm:"column:ip;size:50;comment:IP地址" json:"ip"`
	UserAgent       string    `gorm:"column:user_agent;size:500;comment:User-Agent" json:"userAgent"`
	Hash            string    `gorm:"column:hash;size:64;index;comment:错误指纹" json:"hash"`
	GroupID         int64     `gorm:"column:group_id;default:0;comment:分组ID" json:"groupId"`
	OccurrenceCount int64     `gorm:"column:occurrence_count;default:1;comment:发生次数" json:"occurrenceCount"`
	LastOccurredAt  time.Time `gorm:"column:last_occurred_at;comment:最后发生时间" json:"lastOccurredAt"`
	Resolved        bool      `gorm:"column:resolved;default:false;comment:是否已解决" json:"resolved"`
	ResolvedAt      string    `gorm:"column:resolved_at;size:30;comment:解决时间" json:"resolvedAt"`
	ResolvedBy      uint      `gorm:"column:resolved_by;comment:解决人ID" json:"resolvedBy"`
}

func (Error) TableName() string {
	return "admin_error_log"
}

const (
	LogLevelPanic = "PANIC"
	LogLevelError = "ERROR"
	LogLevelWarn  = "WARN"
)
