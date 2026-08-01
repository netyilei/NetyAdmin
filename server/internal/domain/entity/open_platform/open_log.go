package open_platform

import (
	"time"

	logEntity "NetyAdmin/internal/domain/entity/log"
)

type OpenPlatformLog struct {
	ID            uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	AppID         string `gorm:"size:26;not null;index" json:"appId"`
	AppKey        string `gorm:"size:26;not null" json:"appKey"`
	ApiPath       string `gorm:"size:255;not null" json:"apiPath"`
	ApiMethod     string `gorm:"size:20;not null" json:"apiMethod"`
	ClientIP      string `gorm:"size:50;not null" json:"clientIp"`
	StatusCode    int    `gorm:"not null" json:"statusCode"`
	Latency       int64  `gorm:"not null" json:"latency"`
	RequestHeader string `gorm:"type:text" json:"requestHeader"`
	RequestBody   string `gorm:"type:text" json:"requestBody"`
	ResponseBody  string `gorm:"type:text" json:"responseBody"`
	ErrorMsg      string `gorm:"type:text" json:"errorMsg"`
	// RequestID 由 OpenPlatformAuth 中间件在调用 logSvc.Record 时填入，
	// 对应 DB 列 sys_open_platform_logs.request_id（迁移 0060 新增）。
	RequestID string    `gorm:"column:request_id;size:50;comment:请求ID" json:"requestId"`
	CreatedAt time.Time `gorm:"index" json:"createdAt"`
}

func (OpenPlatformLog) TableName() string {
	return "sys_open_platform_logs"
}

func (l *OpenPlatformLog) GetLogType() logEntity.LogType {
	return logEntity.LogTypeOpen
}

func (l *OpenPlatformLog) GetCreatedAt() time.Time {
	return l.CreatedAt
}

func (l *OpenPlatformLog) GetRequestID() string {
	return l.RequestID
}
