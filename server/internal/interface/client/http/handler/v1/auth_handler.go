package v1

import (
	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/pkg/captcha"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	userRepo "NetyAdmin/internal/repository/user"
	userService "NetyAdmin/internal/service/user"
)

type AuthHandler struct {
	verifySvc userService.VerificationService
	captcha   *captcha.Manager
	watcher   configsync.ConfigWatcher
	userRepo  userRepo.UserRepository
}

func NewAuthHandler(verifySvc userService.VerificationService, captcha *captcha.Manager, watcher configsync.ConfigWatcher, userRepo userRepo.UserRepository) *AuthHandler {
	return &AuthHandler{
		verifySvc: verifySvc,
		captcha:   captcha,
		watcher:   watcher,
		userRepo:  userRepo,
	}
}

// Captcha 获取图形验证码
func (h *AuthHandler) Captcha(c *gin.Context) {
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

// SceneConfig 获取场景验证配置
// 一次请求返回图形验证码开关 + SMS/Email 验证开关及类型
func (h *AuthHandler) SceneConfig(c *gin.Context) {
	scene := c.Query("scene")
	if scene == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "scene 不能为空")
		return
	}

	var captchaEnabled bool
	switch scene {
	case "login":
		val, _ := h.watcher.GetConfig("captcha_config", "user_login_enabled")
		captchaEnabled = val == "true" || val == "1"
	case "register":
		val, _ := h.watcher.GetConfig("captcha_config", "user_register_enabled")
		captchaEnabled = val == "true" || val == "1"
	case "reset_password":
		val, _ := h.watcher.GetConfig("captcha_config", "user_reset_pwd_captcha_enabled")
		captchaEnabled = val == "true" || val == "1"
	default:
		response.FailWithCode(c, errorx.CodeInvalidParams, "不支持的业务场景")
		return
	}

	verifyConfig, _ := h.verifySvc.GetVerifyConfig(c.Request.Context(), scene)

	result := gin.H{
		"scene":          scene,
		"captchaEnabled": captchaEnabled,
	}

	if verifyConfig != nil {
		result["verifyEnabled"] = verifyConfig.Enabled
		result["verifyType"] = verifyConfig.VerifyType
	} else {
		result["verifyEnabled"] = false
		result["verifyType"] = ""
	}

	response.Success(c, result)
}

// SendCode 发送验证码
// 登录场景：使用 username 字段，后端自动查找用户绑定的 email/phone
// 注册/找回密码场景：使用 target 字段，直接发送到指定 email/phone
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

	var target string
	if req.Scene == "login" {
		if req.Username == "" {
			response.FailWithCode(c, errorx.CodeInvalidParams, "登录场景需提供 username")
			return
		}
		user, err := h.userRepo.GetByUsername(c.Request.Context(), req.Username)
		if err != nil {
			response.FailWithCode(c, errorx.CodeUserNotFound, "用户不存在")
			return
		}
		if user.Status == "0" {
			response.FailWithCode(c, errorx.CodeUserDisabled, "账户已禁用")
			return
		}

		verifyConfig, _ := h.verifySvc.GetVerifyConfig(c.Request.Context(), req.Scene)
		if verifyConfig == nil || !verifyConfig.Enabled {
			response.FailWithCode(c, errorx.CodeInvalidParams, "当前场景未启用消息验证")
			return
		}

		switch verifyConfig.VerifyType {
		case "email":
			if user.Email == "" {
				response.FailWithCode(c, errorx.CodeInvalidParams, "该用户未绑定邮箱")
				return
			}
			target = user.Email
		case "sms":
			if user.Phone == "" {
				response.FailWithCode(c, errorx.CodeInvalidParams, "该用户未绑定手机号")
				return
			}
			target = user.Phone
		default:
			response.FailWithCode(c, errorx.CodeInvalidParams, "未配置验证方式")
			return
		}
	} else {
		if req.Target == "" {
			response.FailWithCode(c, errorx.CodeInvalidParams, "需提供 target (手机号或邮箱)")
			return
		}
		target = req.Target
	}

	err := h.verifySvc.SendCode(c.Request.Context(), req.Scene, target, req.CaptchaKey, req.CaptchaCode)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
