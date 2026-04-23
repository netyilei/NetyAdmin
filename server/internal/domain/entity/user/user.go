package user

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// User 终端用户实体
type User struct {
	ID          string                `gorm:"primaryKey;size:26" json:"id"`
	CreatedAt   time.Time             `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt   time.Time             `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
	DeletedAt   soft_delete.DeletedAt `gorm:"column:deleted_at;softDelete:milli;default:0" json:"-"`
	Username    string                `gorm:"column:username;size:50;not null;uniqueIndex" json:"userName"`
	Password    string                `gorm:"column:password;size:100;not null" json:"-"`
	Nickname    string                `gorm:"column:nickname;size:50" json:"nickName"`
	Phone       string                `gorm:"column:phone;size:20" json:"phone"`
	Email       string                `gorm:"column:email;size:100" json:"email"`
	Avatar      string                `gorm:"column:avatar;size:255" json:"avatar"`
	Gender      string                `gorm:"column:gender;size:1;default:0" json:"gender"` // 0: 未知, 1: 男, 2: 女
	Status      string                `gorm:"column:status;size:1;default:1" json:"status"` // 1: 正常, 0: 禁用
	LastLoginAt *time.Time            `gorm:"column:last_login_at" json:"lastLoginAt"`
	LastLoginIP string                `gorm:"column:last_login_ip;size:50" json:"lastLoginIp"`
	Remark      string                `gorm:"column:remark" json:"remark"`
	LastReadID  uint64                `gorm:"column:last_read_announcement_id;default:0" json:"lastReadAnnouncementId"`
}

func (User) TableName() string {
	return "users"
}

// UserTokenHash 用户登录凭证哈希实体
type UserTokenHash struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"column:user_id;size:26;not null;index" json:"userId"`
	TokenHash string    `gorm:"column:token_hash;size:64;not null" json:"tokenHash"`
	ExpiredAt time.Time `gorm:"column:expired_at;not null;index" json:"expiredAt"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

func (UserTokenHash) TableName() string {
	return "user_token_hashes"
}

const (
	UserGenderUnknown = "0"
	UserGenderMale    = "1"
	UserGenderFemale  = "2"
)
