package v1

import (
	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/pkg/captcha"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	userService "NetyAdmin/internal/service/user"
)

type AuthHandler struct {
	verifySvc userService.VerificationService
	captcha   *captcha.Manager
}

func NewAuthHandler(verifySvc userService.VerificationService, captcha *captcha.Manager) *AuthHandler {
	return &AuthHandler{
		verifySvc: verifySvc,
		captcha:   captcha,
	}
}

// GetCaptcha 获取图形验证码
func (h *AuthHandler) GetCaptcha(c *gin.Context) {
	id, b64s, err := h.captcha.Generate("digit", 120, 40, 4)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, "验证码生成失败")
		return
	}
	response.Success(c, gin.H{
		"captchaId": id,
		"img":       b64s,
	})
}

// GetVerifyConfig 获取验证配置
func (h *AuthHandler) GetVerifyConfig(c *gin.Context) {
	scene := c.Query("scene")
	if scene == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "scene 不能为空")
		return
	}

	config, err := h.verifySvc.GetVerifyConfig(c.Request.Context(), scene)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, config)
}

// SendCode 发送验证码
func (h *AuthHandler) SendCode(c *gin.Context) {
	var req struct {
		Target      string `json:"target" binding:"required"`
		Scene       string `json:"scene" binding:"required"`
		CaptchaKey  string `json:"captchaKey"`
		CaptchaCode string `json:"captchaCode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数校验失败")
		return
	}

	err := h.verifySvc.SendCode(c.Request.Context(), req.Scene, req.Target, req.CaptchaKey, req.CaptchaCode)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
