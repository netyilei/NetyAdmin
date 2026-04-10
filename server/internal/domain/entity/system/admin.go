package system

import "silentorder/internal/domain/entity"

type Admin struct {
	entity.Model
	entity.Operator
	Username      string  `gorm:"column:username;size:50;not null;uniqueIndex" json:"username"`
	Password      string  `gorm:"column:password;size:100;not null" json:"-"`
	Nickname      string  `gorm:"column:nickname;size:50" json:"nickname"`
	Phone         string  `gorm:"column:phone;size:20" json:"phone"`
	Email         string  `gorm:"column:email;size:100" json:"email"`
	Gender        string  `gorm:"column:gender;size:1;default:1" json:"gender"`
	Status        string  `gorm:"column:status;size:1;default:1" json:"status"`
	LastLoginAt   *string `gorm:"column:last_login_at;size:30" json:"lastLoginAt"`
	Roles         []*Role `gorm:"many2many:admin_user_roles;joinForeignKey:admin_user_id;joinReferences:admin_role_id" json:"roles"`
	CreatedByUser *Admin  `gorm:"foreignKey:CreatedBy;references:ID" json:"createdByUser"`
	UpdatedByUser *Admin  `gorm:"foreignKey:UpdatedBy;references:ID" json:"updatedByUser"`
}

func (Admin) TableName() string {
	return "admin_user"
}

func (a *Admin) IsEnabled() bool {
	return a.Status == AdminStatusEnabled
}

func (a *Admin) IsSuperAdmin() bool {
	for _, role := range a.Roles {
		if role.Code == SuperRoleCode {
			return true
		}
	}
	return false
}

func (a *Admin) RoleCodes() []string {
	codes := make([]string, 0, len(a.Roles))
	for _, role := range a.Roles {
		codes = append(codes, role.Code)
	}
	return codes
}

func (a *Admin) ButtonCodes() []string {
	var codes []string
	for _, role := range a.Roles {
		for _, button := range role.Buttons {
			codes = append(codes, button.Code)
		}
	}
	return codes
}

func (a *Admin) CreatorName() string {
	if a.CreatedByUser != nil {
		return a.CreatedByUser.Nickname
	}
	return ""
}

func (a *Admin) UpdaterName() string {
	if a.UpdatedByUser != nil {
		return a.UpdatedByUser.Nickname
	}
	return ""
}

const (
	AdminStatusEnabled  = "1"
	AdminStatusDisabled = "0"
)
