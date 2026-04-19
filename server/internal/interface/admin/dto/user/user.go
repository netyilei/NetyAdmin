package user

// UserQuery 终端用户查询参数
type UserQuery struct {
	Current  int     `form:"current"`
	Size     int     `form:"size"`
	Username string  `form:"username"`
	Nickname string  `form:"nickname"`
	Gender   *string `form:"gender"`
	Phone    string  `form:"phone"`
	Email    string  `form:"email"`
	Status   *string `form:"status"`
}

// UpdateUserStatusReq 更新用户状态请求
type UpdateUserStatusReq struct {
	Status string `json:"status" binding:"required,oneof=0 1"`
}

// CreateUserReq 创建用户请求
type CreateUserReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Gender   string `json:"gender"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Status   string `json:"status"`
}

// UpdateUserReq 更新用户请求
type UpdateUserReq struct {
	Password string `json:"password"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Gender   string `json:"gender"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Status   string `json:"status"`
}
