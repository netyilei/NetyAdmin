package auth

import (
	"github.com/gin-gonic/gin"

	systemDto "netyadmin/internal/interface/admin/dto/system"
	"netyadmin/internal/pkg/errorx"
	"netyadmin/internal/pkg/response"
	systemService "netyadmin/internal/service/system"
)

type AuthHandler struct {
	adminService systemService.AdminService
}

func NewAuthHandler(adminService systemService.AdminService) *AuthHandler {
	return &AuthHandler{adminService: adminService}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		UserName string `json:"userName"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	username := body.Username
	if username == "" {
		username = body.UserName
	}
	if username == "" || body.Password == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	req := &systemDto.LoginReq{
		Username: username,
		Password: body.Password,
	}
	result, err := h.adminService.Login(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req systemDto.RefreshTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	result, err := h.adminService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}

func (h *AuthHandler) GetUserInfo(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	result, err := h.adminService.GetAdminInfo(c.Request.Context(), userID.(uint))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	result, err := h.adminService.GetProfile(c.Request.Context(), userID.(uint))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	var req systemDto.UpdateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.adminService.UpdateProfile(c.Request.Context(), userID.(uint), &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "资料修改成功", nil)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	var req systemDto.ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.adminService.ChangePassword(c.Request.Context(), userID.(uint), &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "密码修改成功", nil)
}
