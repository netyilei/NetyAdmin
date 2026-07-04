package common

import (
	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/pkg/response"
	systemService "NetyAdmin/internal/service/system"
)

type CommonHandler struct {
	captchaSvc systemService.CaptchaService
}

func NewCommonHandler(captchaSvc systemService.CaptchaService) *CommonHandler {
	return &CommonHandler{captchaSvc: captchaSvc}
}

// @Summary      获取验证码
// @Description  生成图形验证码，返回验证码ID与Base64图片
// @Tags         通用接口
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "验证码信息"
// @Router       /admin/v1/common/captcha [get]
func (h *CommonHandler) GetCaptcha(c *gin.Context) {
	id, b64s, err := h.captchaSvc.Generate(c.Request.Context(), "admin_login")
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, gin.H{
		"captchaId":  id,
		"captchaImg": b64s,
	})
}
