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

type UserItemVO struct {
	ID          string     `json:"id"`
	Username    string     `json:"userName"`
	Nickname    string     `json:"nickName"`
	Phone       string     `json:"phone"`
	Email       string     `json:"email"`
	Gender      string     `json:"gender"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"createTime"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
}
