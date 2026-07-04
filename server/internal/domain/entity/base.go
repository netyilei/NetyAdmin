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

// HardModel 用于硬删除表（repository 用 Unscoped().Delete() 物理删除记录）。
// 与 Model 的区别：不含 DeletedAt 字段，对应 migration 不含 deleted_at 列，
// 避免 GORM 在 INSERT/SELECT/UPDATE 时自动注入 deleted_at 列导致冗余。
// 嵌入 HardModel 的表必须保证所有删除路径走 Unscoped().Delete()，
// 不允许出现 db.Delete()（会被 GORM 当作软删除，但表无 deleted_at 列会报错）。
type HardModel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

type Operator struct {
	CreatedBy uint `gorm:"column:created_by;comment:创建人ID" json:"createdBy"`
	UpdatedBy uint `gorm:"column:updated_by;comment:更新人ID" json:"updatedBy"`
}

type OperatorName interface {
	CreatorName() string
	UpdaterName() string
}
