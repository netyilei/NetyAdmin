package task

import (
	"time"

	"NetyAdmin/internal/domain/entity"
	logEntity "NetyAdmin/internal/domain/entity/log"
)

type TaskLog struct {
	entity.HardModel
	Name      string    `gorm:"column:name;size:100;not null;index" json:"name"`
	StartTime time.Time `gorm:"column:start_time;not null" json:"startTime"`
	EndTime   time.Time `gorm:"column:end_time;not null" json:"endTime"`
	Duration  float64   `gorm:"column:duration;not null" json:"duration"`
	Status    string    `gorm:"column:status;size:20;not null;index" json:"status"`
	Message   string    `gorm:"column:message;type:text" json:"message"`
	// RequestID 由 task manager 在 onFinish 回调中填入，对应 DB 列
	// sys_task_logs.request_id（建表迁移 0006 已含该列）。
	RequestID string `gorm:"column:request_id;size:50;comment:请求ID" json:"requestId"`
}

func (TaskLog) TableName() string {
	return "sys_task_logs"
}

func (l *TaskLog) GetLogType() logEntity.LogType {
	return logEntity.LogTypeTask
}

func (l *TaskLog) GetCreatedAt() time.Time {
	return l.CreatedAt
}

func (l *TaskLog) GetRequestID() string {
	return l.RequestID
}
