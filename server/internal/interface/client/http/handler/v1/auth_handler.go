package v1

import (
	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	systemService "NetyAdmin/internal/service/system"
	userService "NetyAdmin/internal/service/user"
)

type AuthHandler struct {
	verifySvc  userService.VerificationService
	captchaSvc systemService.CaptchaService
	userSvc    userService.UserClientService
}

func NewAuthHandler(verifySvc userService.VerificationService, captchaSvc systemService.CaptchaService, userSvc userService.UserClientService) *AuthHandler {
	return &AuthHandler{
		verifySvc:  verifySvc,
		captchaSvc: captchaSvc,
		userSvc:    userSvc,
	}
}

// Captcha 获取图形验证码
// @Summary      获取图形验证码
// @Description  生成图形验证码，返回验证码ID与Base64编码的验证码图片
// @Tags         客户端-认证
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/auth/captcha [get]
func (h *AuthHandler) Captcha(c *gin.Context) {
	id, b64s, err := h.captchaSvc.Generate(c.Request.Context(), "user_login")
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, "验证码生成失败")
		return
	}
	response.Success(c, gin.H{
		"captchaId": id,
		"img":       b64s,
	})
}

// SceneConfig 获取场景验证配置
// 一次请求返回图形验证码开关 + SMS/Email 验证开关及类型
// @Summary      获取场景验证配置
// @Description  根据业务场景返回图形验证码开关及短信/邮箱验证开关与类型
// @Tags         客户端-认证
// @Accept       json
// @Produce      json
// @Param        scene query string true "业务场景"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/auth/scene-config [get]
func (h *AuthHandler) SceneConfig(c *gin.Context) {
	scene := c.Query("scene")
	if scene == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "scene 不能为空")
		return
	}

	config, err := h.verifySvc.GetSceneCaptchaConfig(c.Request.Context(), scene)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, config)
}

// SendCode 发送验证码
// 登录场景：使用 username 字段，后端自动查找用户绑定的 email/phone
// 注册/找回密码场景：使用 target 字段，直接发送到指定 email/phone
// @Summary      发送验证码
// @Description  根据场景发送短信或邮箱验证码，登录场景通过username查找绑定联系方式，注册/找回密码场景直接发送至target
// @Tags         客户端-认证
// @Accept       json
// @Produce      json
// @Param        req body object true "发送验证码请求"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/auth/send-code [post]
func (h *AuthHandler) SendCode(c *gin.Context) {
	var req struct {
		Target      string `json:"target"`
		Username    string `json:"username"`
		Scene       string `json:"scene" binding:"required"`
		CaptchaKey  string `json:"captchaKey"`
		CaptchaCode string `json:"captchaCode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数校验失败")
		return
	}

	// 收敛 Handler 跨层调用（spec B10）：通过 service.ResolveSendCodeTarget
	// 统一解析发送目标（登录场景查找用户绑定联系方式，注册/找回密码场景直接使用 target）。
	_, target, err := h.userSvc.ResolveSendCodeTarget(c.Request.Context(), req.Scene, req.Username, req.Target)
	if err != nil {
		response.Fail(c, err)
		return
	}

	err = h.verifySvc.SendCode(c.Request.Context(), req.Scene, target, req.CaptchaKey, req.CaptchaCode)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
