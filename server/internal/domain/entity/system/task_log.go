package system

import (
	"netyadmin/internal/domain/entity"
	"time"
)

type TaskLog struct {
	entity.Model
	Name      string    `gorm:"column:name;size:100;not null;index" json:"name"`
	StartTime time.Time `gorm:"column:start_time;not null" json:"startTime"`
	EndTime   time.Time `gorm:"column:end_time;not null" json:"endTime"`
	Duration  float64   `gorm:"column:duration;not null" json:"duration"` // 秒
	Status    string    `gorm:"column:status;size:20;not null;index" json:"status"`
	Message   string    `gorm:"column:message;type:text" json:"message"`
}

func (TaskLog) TableName() string {
	return "sys_task_logs"
}
