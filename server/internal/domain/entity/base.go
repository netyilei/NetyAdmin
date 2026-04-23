package entity

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

const (
	StatusEnabled  = "1"
	StatusDisabled = "0"
)

const (
	DefaultPageSize = 20
)

type StatusInterface interface {
	GetStatus() string
	IsEnabled() bool
}

type Model struct {
	ID        uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time             `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time             `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;softDelete:milli;default:0" json:"-"`
}

type Operator struct {
	CreatedBy uint `gorm:"column:created_by;comment:创建人ID" json:"createdBy"`
	UpdatedBy uint `gorm:"column:updated_by;comment:更新人ID" json:"updatedBy"`
}

type OperatorName interface {
	CreatorName() string
	UpdaterName() string
}
