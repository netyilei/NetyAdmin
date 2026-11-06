package user

import "time"

type UserLoginVO struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
}

type UserInfoVO struct {
	ID          string     `json:"id"`
	Username    string     `json:"userName"`
	Nickname    string     `json:"nickName"`
	Avatar      string     `json:"avatar"`
	Phone       string     `json:"phone"`
	Email       string     `json:"email"`
	Gender      string     `json:"gender"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
}

type UploadTokenVO struct {
	UploadURL       string `json:"uploadUrl"`
	StorageConfigID uint   `json:"storageConfigId"`
	ObjectKey       string `json:"objectKey"`
	FinalURL        string `json:"finalUrl"`
	RecordID        uint   `json:"recordId"`
	Secret          string `json:"secret"`
}

// UserDetailVO 是管理端「用户详细资料」视图：与列表（掩码）不同，
// 详情是 PII 的完整展示出口——列表保持掩码（性能/展示），详情按权限查看完整值。
// 修改（编辑）也基于本接口回填，避免掩码值回写导致误判。
type UserDetailVO struct {
	ID          string     `json:"id"`
	Username    string     `json:"userName"`
	Nickname    string     `json:"nickName"`
	Avatar      string     `json:"avatar"`
	Gender      string     `json:"gender"`
	Status      string     `json:"status"`
	Phone       string     `json:"phone"`
	Email       string     `json:"email"`
	Remark      string     `json:"remark"`
	Locked      bool       `json:"locked"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
	LastLoginIP string     `json:"lastLoginIp"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// UserAutoCompleteVO 是管理端用户搜索下拉的条目：仅暴露选择所需最小字段，
// 手机号为掩码（autocomplete 曾直接序列化实体，是明文手机号泄露面）。
type UserAutoCompleteVO struct {
	ID       string `json:"id"`
	Username string `json:"userName"`
	Nickname string `json:"nickName"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
}
