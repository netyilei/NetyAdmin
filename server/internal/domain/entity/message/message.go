package message

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// MsgTemplate 消息模板实体
type MsgTemplate struct {
	ID            uint64                `gorm:"primaryKey;autoIncrement" json:"id"`
	Code          string                `gorm:"size:50;not null;uniqueIndex:idx_msg_tpl_code,where:deleted_at = 0" json:"code"`
	Name          string                `gorm:"size:100;not null" json:"name"`
	Channel       string                `gorm:"size:20;not null" json:"channel"` // sms, email, internal, push
	Title         string                `gorm:"size:200" json:"title"`
	Content       string                `gorm:"type:text;not null" json:"content"`
	ProviderTplID string                `gorm:"size:100" json:"providerTplId"`
	Status        int                   `gorm:"default:1" json:"status"` // 1:启用, 0:禁用
	CreatedAt     time.Time             `json:"createdAt"`
	UpdatedAt     time.Time             `json:"updatedAt"`
	DeletedAt     soft_delete.DeletedAt `gorm:"column:deleted_at;softDelete:milli;default:0" json:"-"`
}

func (MsgTemplate) TableName() string {
	return "msg_templates"
}

const (
	MsgStatusPending = 0
	MsgStatusSuccess = 1
	MsgStatusFailed  = 2

	MsgTplStatusEnabled = 1
	MsgTplStatusDisabled = 0

	MsgChannelInternal = "internal"
	MsgChannelSMS      = "sms"
	MsgChannelEmail    = "email"
	MsgChannelPush     = "push"

	MsgInternalTypeSystem  = 1
	MsgInternalTypePrivate = 2
)

// MsgRecord 消息记录实体
type MsgRecord struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     string    `gorm:"size:26;index" json:"userId"`
	Channel    string    `gorm:"size:20;not null" json:"channel"`
	Receiver   string    `gorm:"size:100;not null" json:"receiver"`
	Title      string    `gorm:"size:200" json:"title"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	Status     int       `gorm:"default:0" json:"status"` // 0:等待, 1:成功, 2:失败
	ErrorMsg   string    `gorm:"type:text" json:"errorMsg"`
	NodeID     string    `gorm:"size:50" json:"nodeId"`
	Priority   int       `gorm:"default:2" json:"priority"` // 1:高, 2:中, 3:低
	RetryCount int       `gorm:"default:0" json:"retryCount"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (MsgRecord) TableName() string {
	return "msg_records"
}

// MsgInternal 站内信扩展实体
type MsgInternal struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	MsgRecordID uint64 `gorm:"not null" json:"msgRecordId"`
	Type        int    `gorm:"default:1" json:"type"` // 1:系统公告, 2:私信
}

func (MsgInternal) TableName() string {
	return "msg_internal"
}

// MsgInternalRead 站内信已读记录
type MsgInternalRead struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	MsgInternalID uint64    `gorm:"not null" json:"msgInternalId"`
	UserID        string    `gorm:"size:26;not null" json:"userId"`
	ReadAt        time.Time `json:"readAt"`
}

func (MsgInternalRead) TableName() string {
	return "msg_internal_reads"
}
