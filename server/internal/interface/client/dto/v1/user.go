package v1

// UserLoginReq 用户登录请求
type UserLoginReq struct {
	Username    string `json:"userName" binding:"required"`
	Password    string `json:"password" binding:"required"`
	// Platform 登录平台标识，必填。用于多端会话隔离：
	//   - 同 platform 再次登录 → 顶掉该 platform 的旧会话（顶号）
	//   - 不同 platform → 各自独立会话，互不影响（多端并存）
	// 取值由客户端自定义（如 web/mobile/miniapp），基座不限制枚举。
	Platform    string `json:"platform" binding:"required"`
	CaptchaKey  string `json:"captchaKey"`
	CaptchaCode string `json:"captchaCode"`
	Code        string `json:"code"`
}

type UserRegisterReq struct {
	Username string `json:"userName" binding:"required,min=4,max=20"`
	Password string `json:"password" binding:"required,min=8,max=20"`
	Nickname string `json:"nickName" binding:"required"`
	Phone    string `json:"phone"`
	Email    string `json:"email" binding:"omitempty,email,max=100"`
	Code     string `json:"code"`
}

// UserResetPasswordReq 找回密码请求
type UserResetPasswordReq struct {
	Target      string `json:"target" binding:"required"` // phone or email
	Code        string `json:"code" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=20"`
}

// UserUpdateProfileReq 更新个人资料请求
type UserUpdateProfileReq struct {
	Nickname  string `json:"nickName"`
	Avatar    string `json:"avatar"`
	Gender    string `json:"gender" binding:"omitempty,oneof=0 1 2"`
	Email     string `json:"email" binding:"omitempty,email,max=100"`
	Phone     string `json:"phone"`
	EmailCode string `json:"emailCode"` // 邮箱变更验证码（变更邮箱时必填）
	PhoneCode string `json:"phoneCode"` // 手机变更验证码（变更手机时必填）
}

// UserChangePasswordReq 修改密码请求
type UserChangePasswordReq struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=20"`
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
	RecordID  uint   `json:"recordId" binding:"required"`
	Secret    string `json:"secret" binding:"required"`
	ObjectKey string `json:"objectKey"`
	FileURL   string `json:"fileUrl"`
	FileSize  int64  `json:"fileSize"`
	MimeType  string `json:"mimeType"`
	MD5       string `json:"md5"`
}
