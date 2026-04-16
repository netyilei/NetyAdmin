package open_platform

import "time"

// OpenPlatformLog 开放平台调用日志实体
type OpenPlatformLog struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	AppID         string    `gorm:"size:26;not null;index" json:"appId"`
	AppKey        string    `gorm:"size:26;not null" json:"appKey"`
	ApiPath       string    `gorm:"size:255;not null" json:"apiPath"`
	ApiMethod     string    `gorm:"size:20;not null" json:"apiMethod"`
	ClientIP      string    `gorm:"size:50;not null" json:"clientIp"`
	StatusCode    int       `gorm:"not null" json:"statusCode"`
	Latency       int64     `gorm:"not null" json:"latency"` // 纳秒
	RequestHeader string    `gorm:"type:text" json:"requestHeader"`
	RequestBody   string    `gorm:"type:text" json:"requestBody"`
	ResponseBody  string    `gorm:"type:text" json:"responseBody"`
	ErrorMsg      string    `gorm:"type:text" json:"errorMsg"`
	CreatedAt     time.Time `gorm:"index" json:"createdAt"`
}

func (OpenPlatformLog) TableName() string {
	return "sys_open_platform_logs"
}
