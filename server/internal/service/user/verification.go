package user

import (
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/utils"
	msgSvc "NetyAdmin/internal/service/message"
	"context"
	"crypto/rand"
	"log/slog"
	"math/big"
	"strconv"
	"time"

	"github.com/mojocn/base64Captcha"
)

const (
	SceneRegister      = "register"
	SceneResetPassword = "reset_password"
	SceneLogin         = "login"
	SceneBind          = "bind"
	SceneChangeEmail   = "change_email" // 邮箱变更验证码场景
	SceneChangePhone   = "change_phone" // 手机变更验证码场景
)

// VerifyConfig 验证配置响应
type VerifyConfig struct {
	Enabled    bool   `json:"enabled"`
	VerifyType string `json:"verifyType"`
	Scene      string `json:"scene"`
}

// SceneCaptchaConfig 场景验证配置（图形验证码 + 消息验证二合一）
// 收敛 handler 跨层调用（spec B10）：handler 不再直接调 configsync.ConfigWatcher
type SceneCaptchaConfig struct {
	Scene          string `json:"scene"`
	CaptchaEnabled bool   `json:"captchaEnabled"` // 图形验证码开关 (captcha_config.{scene}_enabled)
	VerifyEnabled  bool   `json:"verifyEnabled"`  // 消息验证开关 (user_config.{scene}_verify)
	VerifyType     string `json:"verifyType"`     // 消息验证类型 email/sms (user_config.{scene}_verify_type)
}

type VerificationService interface {
	// GetVerifyConfig 获取验证配置
	GetVerifyConfig(ctx context.Context, scene string) (*VerifyConfig, error)
	// GetSceneCaptchaConfig 一次返回 scene 的图形验证码开关 + 消息验证开关/类型
	GetSceneCaptchaConfig(ctx context.Context, scene string) (*SceneCaptchaConfig, error)
	// SendCode 发送验证码 (自动判断是手机还是邮箱)
	// captchaKey 和 captchaCode 用于二次验证，防止接口被恶意轰炸
	SendCode(ctx context.Context, scene, target, captchaKey, captchaCode string) error
	// VerifyCode 校验验证码 (不清理，用于多步流程)
	VerifyCode(ctx context.Context, scene, target, code string) (bool, error)
	// VerifyAndClearCode 校验并清理验证码 (用于最终提交)
	VerifyAndClearCode(ctx context.Context, scene, target, code string) (bool, error)
}

type verificationService struct {
	cacheSlow    cache.SecurityCache
	msgSvc       msgSvc.MessageService
	watcher      configsync.ConfigWatcher
	captchaStore base64Captcha.Store
}

func NewVerificationService(cacheSlow cache.SecurityCache, msgSvc msgSvc.MessageService, watcher configsync.ConfigWatcher, captchaStore base64Captcha.Store) VerificationService {
	return &verificationService{
		cacheSlow:    cacheSlow,
		msgSvc:       msgSvc,
		watcher:      watcher,
		captchaStore: captchaStore,
	}
}

func (s *verificationService) GetVerifyConfig(ctx context.Context, scene string) (*VerifyConfig, error) {
	config := &VerifyConfig{Scene: scene}

	var enabledKey, typeKey string
	switch scene {
	case SceneRegister:
		enabledKey = "user_register_verify"
		typeKey = "user_register_verify_type"
	case SceneResetPassword:
		enabledKey = "user_reset_pwd_verify"
		typeKey = "user_reset_pwd_verify_type"
	case SceneLogin:
		enabledKey = "user_login_verify"
		typeKey = "user_login_verify_type"
	default:
		return nil, errorx.New(errorx.CodeInvalidParams, "不支持的业务场景")
	}

	enabledVal, _ := s.watcher.GetConfig("user_config", enabledKey)
	config.Enabled = enabledVal == "true" || enabledVal == "1"

	typeVal, _ := s.watcher.GetConfig("user_config", typeKey)
	if typeVal == "" {
		typeVal = "email" // 默认邮件
	}
	config.VerifyType = typeVal

	return config, nil
}

func (s *verificationService) GetSceneCaptchaConfig(ctx context.Context, scene string) (*SceneCaptchaConfig, error) {
	captchaKey := sceneCaptchaKey(scene)
	if captchaKey == "" {
		return nil, errorx.New(errorx.CodeInvalidParams, "不支持的业务场景")
	}

	// 图形验证码开关
	val, _ := s.watcher.GetConfig("captcha_config", captchaKey)
	captchaEnabled := val == "true" || val == "1"

	// 消息验证开关 + 类型（复用现有 GetVerifyConfig 逻辑）
	verifyConfig, _ := s.GetVerifyConfig(ctx, scene)

	result := &SceneCaptchaConfig{
		Scene:          scene,
		CaptchaEnabled: captchaEnabled,
	}
	if verifyConfig != nil {
		result.VerifyEnabled = verifyConfig.Enabled
		result.VerifyType = verifyConfig.VerifyType
	}
	return result, nil
}

func (s *verificationService) SendCode(ctx context.Context, scene, target, captchaKey, captchaCode string) error {
	// 0. 图形验证码二次校验 (Synergy)
	// 如果配置开启了图形验证码，则必须校验，防止接口轰炸。
	captchaEnabledKey := sceneCaptchaKey(scene)
	if captchaEnabledKey != "" {
		captchaVal, _ := s.watcher.GetConfig("captcha_config", captchaEnabledKey)
		captchaEnabled := captchaVal == "true" || captchaVal == "1"
		if captchaEnabled {
			if captchaKey == "" || captchaCode == "" {
				return errorx.New(errorx.CodeCaptchaRequired, "请输入图形验证码")
			}
			if !s.captchaStore.Verify(captchaKey, captchaCode, true) {
				return errorx.New(errorx.CodeCaptchaInvalid, "图形验证码错误")
			}
		}
	}

	// 1. 频率限制 (60秒内只能发送一次)
	limitKey := cache.KeyVerifyCodeLimit(scene, target)
	exists, _ := s.cacheSlow.Exists(ctx, limitKey)
	if exists {
		return errorx.New(errorx.CodeCaptchaSendTooFrequent, "验证码发送过于频繁，请稍后再试")
	}

	// 2. 生成 6 位随机验证码
	code, err := s.generateCode(6)
	if err != nil {
		return errorx.New(errorx.CodeInternalError, "验证码生成失败")
	}

	// 3. 存储验证码 (有效时长 10 分钟)
	cacheKey := cache.KeyVerificationCode(scene, target)
	if err := s.cacheSlow.Set(ctx, cacheKey, code, 10*time.Minute); err != nil {
		return errorx.New(errorx.CodeInternalError, "验证码存储失败")
	}

	// 4. 设置频率限制
	if err := s.cacheSlow.Set(ctx, limitKey, "1", 60*time.Second); err != nil {
		slog.Warn("set rate limit cache failed", "key", limitKey, "err", err)
	}

	// 5. 调用消息服务发送
	// 模板代码约定：VERIFY_CODE
	params := map[string]string{
		"code": code,
	}

	// 简单判断是邮箱还是手机号
	if utils.IsEmail(target) {
		return s.msgSvc.SendTemplate(ctx, "VERIFY_CODE_EMAIL", target, params)
	} else {
		return s.msgSvc.SendTemplate(ctx, "VERIFY_CODE_SMS", target, params)
	}
}

func (s *verificationService) VerifyCode(ctx context.Context, scene, target, code string) (bool, error) {
	if code == "" {
		return false, nil
	}

	// 尝试次数检查：超过 5 次自动失效验证码
	attemptKey := cache.KeyVerifyCodeAttempt(scene, target)
	var attemptStr string
	_ = s.cacheSlow.Get(ctx, attemptKey, &attemptStr)
	if n, err := strconv.Atoi(attemptStr); err == nil && n >= 5 {
		if dErr := s.cacheSlow.Delete(ctx, cache.KeyVerificationCode(scene, target)); dErr != nil {
			slog.Warn("delete verification code cache failed", "scene", scene, "target", target, "err", dErr)
		}
		return false, nil
	}

	cacheKey := cache.KeyVerificationCode(scene, target)
	var storedCode string
	err := s.cacheSlow.Get(ctx, cacheKey, &storedCode)
	if err != nil {
		return false, nil // 验证码不存在或已过期
	}

	if storedCode != code {
		n, _ := strconv.Atoi(attemptStr)
		n++
		if err := s.cacheSlow.Set(ctx, attemptKey, strconv.Itoa(n), 10*time.Minute); err != nil {
			slog.Warn("set attempt count cache failed", "key", attemptKey, "err", err)
		}
		return false, nil
	}

	return true, nil
}

func (s *verificationService) VerifyAndClearCode(ctx context.Context, scene, target, code string) (bool, error) {
	ok, err := s.VerifyCode(ctx, scene, target, code)
	if ok {
		cacheKey := cache.KeyVerificationCode(scene, target)
		if dErr := s.cacheSlow.Delete(ctx, cacheKey); dErr != nil {
			slog.Warn("delete verification code cache failed", "key", cacheKey, "err", dErr)
		}
	}
	return ok, err
}

func (s *verificationService) generateCode(length int) (string, error) {
	const charset = "0123456789"
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}

func sceneCaptchaKey(scene string) string {
	switch scene {
	case SceneRegister:
		return "user_register_enabled"
	case SceneResetPassword:
		return "user_reset_pwd_captcha_enabled"
	case SceneLogin:
		return "user_login_enabled"
	default:
		return ""
	}
}
