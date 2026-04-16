package open_platform

import (
	"time"

	"gorm.io/gorm"
)

const (
	AppTypeInternal = 1 // 官方内部
	AppTypeExternal = 2 // 外部合作伙伴
)

const (
	AppStatusDisabled = 0
	AppStatusEnabled  = 1
)

const (
	IPStrategyBlacklist = 1
	IPStrategyWhitelist = 2
)

// AppQuotaConfig 应用限流配置
type AppQuotaConfig struct {
	Rate     int `json:"rate"`     // 每秒请求数
	Capacity int `json:"capacity"` // 桶容量
}

// 开放平台预定义 Scopes
const (
	ScopeUserBase    = "user_base"    // 用户基础 (注册/登录)
	ScopeUserProfile = "user_profile" // 用户资料 (修改/注销)
	ScopeMsgSend     = "msg_send"     // 消息发送 (SMS/Email)
	ScopeContentView = "content_view" // 内容查看
)

// App 开放平台应用实体
type App struct {
	ID          string         `gorm:"primaryKey;size:26" json:"id"` // ULID
	AppKey      string         `gorm:"size:26;not null;uniqueIndex:idx_apps_key,where:deleted_at = 0" json:"appKey"`
	AppSecret   string         `gorm:"size:255;not null" json:"-"` // AES 加密存储
	Name        string         `gorm:"size:100;not null" json:"name"`
	Type        int            `gorm:"default:1" json:"type"`        // 1: Internal, 2: External
	Status      int            `gorm:"default:1;index" json:"status"` // 1: Enabled, 0: Disabled
	IPStrategy  int            `gorm:"default:1" json:"ipStrategy"`  // 1: Blacklist, 2: Whitelist
	QuotaConfig string         `gorm:"type:jsonb" json:"quotaConfig"`
	Remark      string         `gorm:"size:255" json:"remark"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Scopes 不落库，仅用于 API 返回
	Scopes []string `gorm:"-" json:"scopes,omitempty"`
}

func (App) TableName() string {
	return "sys_apps"
}

// AppScope 应用权限范围实体
type AppScope struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	AppID     string    `gorm:"size:26;not null;uniqueIndex:idx_app_scopes_unique" json:"appId"`
	Scope     string    `gorm:"size:50;not null;uniqueIndex:idx_app_scopes_unique" json:"scope"`
	CreatedAt time.Time `json:"createdAt"`
}

func (AppScope) TableName() string {
	return "sys_app_scopes"
}
