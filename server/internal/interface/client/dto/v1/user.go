package v1

// UserLoginReq 用户登录请求
type UserLoginReq struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	Platform    string `json:"platform"`
	CaptchaKey  string `json:"captchaKey"`
	CaptchaCode string `json:"captchaCode"`
	Code        string `json:"code"`
}

// UserRegisterReq 用户注册请求
type UserRegisterReq struct {
	Username string `json:"username" binding:"required,min=4,max=20"`
	Password string `json:"password" binding:"required,min=6,max=20"`
	Nickname string `json:"nickname" binding:"required"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Code     string `json:"code"`
}

// UserResetPasswordReq 找回密码请求
type UserResetPasswordReq struct {
	Target      string `json:"target" binding:"required"` // phone or email
	Code        string `json:"code" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6,max=20"`
}

// UserUpdateProfileReq 更新个人资料请求
type UserUpdateProfileReq struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Gender   string `json:"gender"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

// UserChangePasswordReq 修改密码请求
type UserChangePasswordReq struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6,max=20"`
}

// UserTokenVO 登录返回的 Token
type UserTokenVO struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
}

// UserInfoVO 用户信息
type UserInfoVO struct {
	ID          string `json:"id"`
	Username    string `json:"userName"`
	Nickname    string `json:"nickName"`
	Avatar      string `json:"avatar"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Gender      string `json:"gender"`
	Status      string `json:"status"`
	LastLoginAt string `json:"lastLoginAt,omitempty"`
}

type CreateUserUploadRecordReq struct {
	FileName     string `json:"fileName" binding:"required"`
	ObjectKey    string `json:"objectKey" binding:"required"`
	FileSize     int64  `json:"fileSize"`
	MimeType     string `json:"mimeType"`
	MD5          string `json:"md5"`
	StorageConfigID uint `json:"storageConfigId"`
	BusinessType string `json:"businessType"`
	BusinessID   string `json:"businessId"`
}
