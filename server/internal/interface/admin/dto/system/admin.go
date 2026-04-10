package system

import "NetyAdmin/internal/interface/admin/dto"

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshTokenReq struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type ChangePasswordReq struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

type UpdateProfileReq struct {
	Nickname string `json:"nickname"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Gender   string `json:"gender"`
}

type AdminQuery struct {
	dto.PageQuery
	Username string  `form:"username" json:"username"`
	Nickname string  `form:"nickname" json:"nickname"`
	Phone    string  `form:"phone" json:"phone"`
	Email    string  `form:"email" json:"email"`
	Status   *string `form:"status" json:"status"`
	Gender   *string `form:"gender" json:"gender"`
}

type CreateAdminReq struct {
	Username string   `json:"username" binding:"required,min=3,max=50"`
	Password string   `json:"password" binding:"required,min=6"`
	Nickname string   `json:"nickname"`
	Phone    string   `json:"phone"`
	Email    string   `json:"email"`
	Gender   string   `json:"gender"`
	Status   string   `json:"status" binding:"required,oneof=0 1"`
	Roles    []string `json:"roles"`
}

type UpdateAdminReq struct {
	ID       uint     `json:"id" binding:"required"`
	Username string   `json:"username" binding:"required,min=3,max=50"`
	Nickname string   `json:"nickname"`
	Phone    string   `json:"phone"`
	Email    string   `json:"email"`
	Gender   string   `json:"gender"`
	Status   string   `json:"status" binding:"required,oneof=0 1"`
	Roles    []string `json:"roles"`
}
