package system

import (
	"context"
	"strconv"

	"NetyAdmin/internal/pkg/captcha"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/errorx"
)

// CaptchaService 验证码服务
// 收敛验证码生成参数读取 + 验证码开关判断业务逻辑，避免 Handler 持有 captchaMgr/watcher。
type CaptchaService interface {
	// Generate 生成验证码，scene 用于未来扩展（当前所有场景共享 captcha_config 配置）
	Generate(ctx context.Context, scene string) (id, b64s string, err error)
	// Verify 校验验证码：内部读取 `<scene>_enabled` 配置判断是否启用，启用则校验
	Verify(ctx context.Context, scene, captchaId, captchaValue string) error
}

type captchaService struct {
	captchaMgr *captcha.Manager
	watcher    configsync.ConfigWatcher
}

func NewCaptchaService(captchaMgr *captcha.Manager, watcher configsync.ConfigWatcher) CaptchaService {
	return &captchaService{
		captchaMgr: captchaMgr,
		watcher:    watcher,
	}
}

func (s *captchaService) Generate(ctx context.Context, scene string) (id, b64s string, err error) {
	configs := s.watcher.GetGroupConfigs("captcha_config")

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

	return s.captchaMgr.Generate(captchaType, width, height, length)
}

func (s *captchaService) Verify(ctx context.Context, scene, captchaId, captchaValue string) error {
	enabledKey := scene + "_enabled"
	if val, exists := s.watcher.GetConfig("captcha_config", enabledKey); exists && (val == "true" || val == "1") {
		if captchaId == "" || captchaValue == "" {
			return errorx.New(errorx.CodeCaptchaRequired, "验证码必填")
		}
		if !s.captchaMgr.Verify(captchaId, captchaValue, true) {
			return errorx.New(errorx.CodeCaptchaInvalid, "验证码错误")
		}
	}
	return nil
}
