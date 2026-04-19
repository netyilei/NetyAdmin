package auth

import (
	"github.com/gin-gonic/gin"

	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/captcha"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	systemService "NetyAdmin/internal/service/system"
)

type AuthHandler struct {
	adminService systemService.AdminService
	captchaMgr   *captcha.Manager
	watcher      configsync.ConfigWatcher
}

func NewAuthHandler(adminService systemService.AdminService, captchaMgr *captcha.Manager, watcher configsync.ConfigWatcher) *AuthHandler {
	return &AuthHandler{
		adminService: adminService,
		captchaMgr:   captchaMgr,
		watcher:      watcher,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var body struct {
		Username     string `json:"username"`
		UserName     string `json:"userName"`
		Password     string `json:"password"`
		CaptchaId    string `json:"captchaId"`
		CaptchaValue string `json:"captchaValue"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	// 验证码校验逻辑
	if val, exists := h.watcher.GetConfig("captcha_config", "admin_login_enabled"); exists && (val == "true" || val == "1") {
		if body.CaptchaId == "" || body.CaptchaValue == "" {
			response.FailWithCode(c, errorx.CodeCaptchaRequired, "")
			return
		}
		if !h.captchaMgr.Verify(body.CaptchaId, body.CaptchaValue, true) {
			response.FailWithCode(c, errorx.CodeCaptchaInvalid, "")
			return
		}
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
	adminID, exists := c.Get("adminID")
	if !exists {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	result, err := h.adminService.GetAdminInfo(c.Request.Context(), adminID.(uint))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	adminID, exists := c.Get("adminID")
	if !exists {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	result, err := h.adminService.GetProfile(c.Request.Context(), adminID.(uint))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	adminID, exists := c.Get("adminID")
	if !exists {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	var req systemDto.UpdateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.adminService.UpdateProfile(c.Request.Context(), adminID.(uint), &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "资料修改成功", nil)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	adminID, exists := c.Get("adminID")
	if !exists {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	var req systemDto.ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.adminService.ChangePassword(c.Request.Context(), adminID.(uint), &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "密码修改成功", nil)
}
