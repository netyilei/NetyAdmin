package ipac

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

const (
	IPACTypeAllow = 1 // 放行
	IPACTypeDeny  = 2 // 封禁
)

const (
	IPACStatusDisabled = 0 // 禁用
	IPACStatusEnabled  = 1 // 启用
)

// IPAccessControl IP 访问控制实体
type IPAccessControl struct {
	ID        uint                  `gorm:"primaryKey;autoIncrement" json:"id"`
	AppID     *string               `gorm:"size:26;index" json:"appId"` // 所属应用 ID (NULL 为全局)
	IPAddr    string                `gorm:"size:50;not null;uniqueIndex:idx_ipac_app_ip,where:deleted_at = 0" json:"ipAddr"`
	Type      int                   `gorm:"default:2" json:"type"`         // 1: Allow, 2: Deny
	Reason    string                `gorm:"size:255" json:"reason"`        // 封禁原因
	ExpiredAt *time.Time            `json:"expiredAt"`                     // 过期时间
	Status    int                   `gorm:"default:1;index" json:"status"` // 1: 启用, 0: 禁用
	CreatedBy uint                  `json:"createdBy"`
	UpdatedBy uint                  `json:"updatedBy"`
	CreatedAt time.Time             `json:"createdAt"`
	UpdatedAt time.Time             `json:"updatedAt"`
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;softDelete:milli;default:0" json:"-"`
}

func (IPAccessControl) TableName() string {
	return "sys_ip_access_control"
}

// IsExpired 检查规则是否已过期
func (i *IPAccessControl) IsExpired() bool {
	if i.ExpiredAt == nil {
		return false
	}
	return i.ExpiredAt.Before(time.Now())
}

// IsEffective 检查规则是否生效 (启用且未过期)
func (i *IPAccessControl) IsEffective() bool {
	return i.Status == IPACStatusEnabled && !i.IsExpired()
}
