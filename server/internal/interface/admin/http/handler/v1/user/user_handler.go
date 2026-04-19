package user

import (
	"github.com/gin-gonic/gin"

	userDto "NetyAdmin/internal/interface/admin/dto/user"
	"NetyAdmin/internal/domain/entity/user"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	userRepo "NetyAdmin/internal/repository/user"
	userSvc "NetyAdmin/internal/service/user"
)

type UserHandler struct {
	svc userSvc.UserService
}

func NewUserHandler(svc userSvc.UserService) *UserHandler {
	return &UserHandler{
		svc: svc,
	}
}

// List 获取用户列表
func (h *UserHandler) List(c *gin.Context) {
	var req userDto.UserQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	query := &userRepo.UserRepoQuery{
		Username: req.Username,
		Nickname: req.Nickname,
		Gender:   req.Gender,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   req.Status,
	}

	users, total, err := h.svc.List(c.Request.Context(), req.Current, req.Size, query)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, users)
}

// Autocomplete 查找用户自动补全
func (h *UserHandler) Autocomplete(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		response.Success(c, []any{})
		return
	}
	users, err := h.svc.SearchForAutocomplete(c.Request.Context(), keyword, 20)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, users)
}

// Create 创建用户
func (h *UserHandler) Create(c *gin.Context) {
	var req userDto.CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	u := &user.User{
		Username: req.Username,
		Password: req.Password,
		Nickname: req.Nickname,
		Avatar:   req.Avatar,
		Gender:   req.Gender,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   req.Status,
	}

	if err := h.svc.Create(c.Request.Context(), u); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// Update 更新用户
func (h *UserHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req userDto.UpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	u := &user.User{
		ID:       id,
		Password: req.Password,
		Nickname: req.Nickname,
		Avatar:   req.Avatar,
		Gender:   req.Gender,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   req.Status,
	}

	if err := h.svc.Update(c.Request.Context(), u); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// UpdateStatus 更新用户状态
func (h *UserHandler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")
	var req userDto.UpdateUserStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// Delete 删除用户
func (h *UserHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
