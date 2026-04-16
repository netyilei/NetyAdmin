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
