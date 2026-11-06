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

// UserAutoCompleteVO 是管理端用户搜索下拉的条目：仅暴露选择所需最小字段，
// 手机号为掩码（autocomplete 曾直接序列化实体，是明文手机号泄露面）。
type UserAutoCompleteVO struct {
	ID       string `json:"id"`
	Username string `json:"userName"`
	Nickname string `json:"nickName"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
}
