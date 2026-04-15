package common

import (
	"strconv"

	"NetyAdmin/internal/pkg/captcha"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

type CommonHandler struct {
	captchaMgr *captcha.Manager
	watcher    configsync.ConfigWatcher
}

func NewCommonHandler(captchaMgr *captcha.Manager, watcher configsync.ConfigWatcher) *CommonHandler {
	return &CommonHandler{
		captchaMgr: captchaMgr,
		watcher:    watcher,
	}
}

// GetCaptcha 获取验证码
func (h *CommonHandler) GetCaptcha(c *gin.Context) {
	// 获取验证码配置
	configs := h.watcher.GetGroupConfigs("captcha_config")
	
	captchaType := configs["captcha_type"]
	if captchaType == "" {
		captchaType = "digit"
	}
	
	width, _ := strconv.Atoi(configs["captcha_width"])
	if width <= 0 {
		width = 240
	}
	
	height, _ := strconv.Atoi(configs["captcha_height"])
	if height <= 0 {
		height = 80
	}
	
	length, _ := strconv.Atoi(configs["captcha_length"])
	if length <= 0 {
		length = 4
	}

	id, b64s, err := h.captchaMgr.Generate(captchaType, width, height, length)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, gin.H{
		"captchaId":  id,
		"captchaImg": b64s,
	})
}
