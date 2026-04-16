package user

import (
	"github.com/gin-gonic/gin"

	userDto "NetyAdmin/internal/interface/admin/dto/user"
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
