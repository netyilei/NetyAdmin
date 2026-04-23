package ipac

import (
	"time"

	"NetyAdmin/internal/domain/entity"
)

const (
	IPACTypeAllow = 1
	IPACTypeDeny  = 2

	IPACStatusDisabled = 0
	IPACStatusEnabled  = 1
)

type IPAccessControl struct {
	entity.Model
	entity.Operator
	AppID     *string    `gorm:"size:26;index" json:"appId"`
	IPAddr    string     `gorm:"size:50;not null;uniqueIndex:idx_ipac_app_ip,where:deleted_at = 0" json:"ipAddr"`
	Type      int        `gorm:"default:2" json:"type"`
	Reason    string     `gorm:"size:255" json:"reason"`
	ExpiredAt *time.Time `json:"expiredAt"`
	Status    int        `gorm:"default:1;index" json:"status"`
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
