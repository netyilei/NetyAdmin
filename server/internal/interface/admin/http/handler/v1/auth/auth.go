package auth

import (
	"strings"

	"github.com/gin-gonic/gin"

	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	systemService "NetyAdmin/internal/service/system"
)

// LoginRequest admin login request body
type LoginRequest struct {
	Username     string `json:"username" example:"admin"`
	UserName     string `json:"userName" example:"admin"`
	Password     string `json:"password" example:"admin123"`
	CaptchaId    string `json:"captchaId"`
	CaptchaValue string `json:"captchaValue"`
}

type AuthHandler struct {
	adminService systemService.AdminService
	captchaSvc   systemService.CaptchaService
}

func NewAuthHandler(adminService systemService.AdminService, captchaSvc systemService.CaptchaService) *AuthHandler {
	return &AuthHandler{
		adminService: adminService,
		captchaSvc:   captchaSvc,
	}
}

// @Summary      管理员登录
// @Description  使用用户名/密码登录管理员账号，可选验证码校验，返回访问令牌与刷新令牌
// @Tags         认证管理
// @Accept       json
// @Produce      json
// @Param        body body LoginRequest true "登录参数"
// @Success      200 {object} response.Response "登录成功"
// @Router       /admin/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var body LoginRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	// 验证码校验（业务逻辑下沉到 CaptchaService）
	if err := h.captchaSvc.Verify(c.Request.Context(), "admin_login", body.CaptchaId, body.CaptchaValue); err != nil {
		response.Fail(c, err)
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

// @Summary      刷新令牌
// @Description  使用刷新令牌换取新的访问令牌与刷新令牌
// @Tags         认证管理
// @Accept       json
// @Produce      json
// @Param        req body system.RefreshTokenReq true "刷新令牌请求"
// @Success      200 {object} response.Response "刷新成功"
// @Router       /admin/v1/auth/refreshToken [post]
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

// @Summary      获取当前登录用户信息
// @Description  根据上下文中的管理员ID获取当前登录用户的详细信息
// @Tags         认证管理
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "用户信息"
// @Security    ApiKeyAuth
// @Router       /admin/v1/auth/getUserInfo [get]
func (h *AuthHandler) GetUserInfo(c *gin.Context) {
	adminID := c.GetUint("adminID")
	if adminID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	result, err := h.adminService.GetAdminInfo(c.Request.Context(), adminID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}

// @Summary      获取个人资料
// @Description  获取当前登录管理员的个人资料信息
// @Tags         认证管理
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "个人资料"
// @Security    ApiKeyAuth
// @Router       /admin/v1/auth/profile [get]
func (h *AuthHandler) GetProfile(c *gin.Context) {
	adminID := c.GetUint("adminID")
	if adminID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	result, err := h.adminService.GetProfile(c.Request.Context(), adminID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}

// @Summary      更新个人资料
// @Description  更新当前登录管理员的个人资料，例如昵称、手机号、邮箱、性别
// @Tags         认证管理
// @Accept       json
// @Produce      json
// @Param        req body system.UpdateProfileReq true "个人资料信息"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/auth/profile [put]
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	adminID := c.GetUint("adminID")
	if adminID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	var req systemDto.UpdateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.adminService.UpdateProfile(c.Request.Context(), adminID, &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "资料修改成功", nil)
}

// @Summary      修改密码
// @Description  当前登录管理员通过旧密码修改登录密码
// @Tags         认证管理
// @Accept       json
// @Produce      json
// @Param        req body system.ChangePasswordReq true "修改密码请求"
// @Success      200 {object} response.Response "修改成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/auth/changePassword [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	adminID := c.GetUint("adminID")
	if adminID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	var req systemDto.ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.adminService.ChangePassword(c.Request.Context(), adminID, &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "密码修改成功", nil)
}

// @Summary      退出登录
// @Description  当前登录管理员退出登录，注销访问令牌（同时将 refresh token 加入黑名单）
// @Tags         认证管理
// @Accept       json
// @Produce      json
// @Param        X-Refresh-Token header string true "刷新令牌（必填，用于加入黑名单）"
// @Success      200 {object} response.Response "退出成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	adminID := c.GetUint("adminID")
	if adminID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	authHeader := c.GetHeader("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	// 提取 refresh token，用于加入黑名单（防止登出后 refresh token 仍可换取新 access token）
	refreshToken := c.GetHeader("X-Refresh-Token")
	if refreshToken == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "缺少刷新令牌")
		return
	}

	if err := h.adminService.Logout(c.Request.Context(), adminID, token, refreshToken); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "退出登录成功", nil)
}
