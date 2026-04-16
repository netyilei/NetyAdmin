package user

import (
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/utils"
	msgSvc "NetyAdmin/internal/service/message"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/mojocn/base64Captcha"
)

const (
	SceneRegister      = "register"
	SceneResetPassword = "reset_password"
	SceneLogin         = "login"
	SceneBind          = "bind"
)

// VerifyConfig 验证配置响应
type VerifyConfig struct {
	Enabled    bool   `json:"enabled"`
	VerifyType string `json:"verifyType"`
	Scene      string `json:"scene"`
}

type VerificationService interface {
	// GetVerifyConfig 获取验证配置
	GetVerifyConfig(ctx context.Context, scene string) (*VerifyConfig, error)
	// SendCode 发送验证码 (自动判断是手机还是邮箱)
	// captchaKey 和 captchaCode 用于二次验证，防止接口被恶意轰炸
	SendCode(ctx context.Context, scene, target, captchaKey, captchaCode string) error
	// VerifyCode 校验验证码 (不清理，用于多步流程)
	VerifyCode(ctx context.Context, scene, target, code string) (bool, error)
	// VerifyAndClearCode 校验并清理验证码 (用于最终提交)
	VerifyAndClearCode(ctx context.Context, scene, target, code string) (bool, error)
}

type verificationService struct {
	cacheMgr     cache.LazyCacheManager
	msgSvc       msgSvc.MessageService
	watcher      configsync.ConfigWatcher
	captchaStore base64Captcha.Store
}

func NewVerificationService(cacheMgr cache.LazyCacheManager, msgSvc msgSvc.MessageService, watcher configsync.ConfigWatcher, captchaStore base64Captcha.Store) VerificationService {
	return &verificationService{
		cacheMgr:     cacheMgr,
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

func (s *verificationService) SendCode(ctx context.Context, scene, target, captchaKey, captchaCode string) error {
	// 0. 图形验证码二次校验 (Synergy)
	// 如果开启了验证码，则必须校验。目前逻辑：只要传了就校验，防止接口轰炸。
	if captchaKey != "" && captchaCode != "" {
		if !s.captchaStore.Verify(captchaKey, captchaCode, true) {
			return errorx.New(errorx.CodeCaptchaInvalid, "图形验证码错误")
		}
	}

	// 1. 频率限制 (60秒内只能发送一次)
	limitKey := fmt.Sprintf("auth:limit:%s:%s", scene, target)
	exists, _ := s.cacheMgr.Exists(ctx, limitKey)
	if exists {
		return errorx.New(errorx.CodeTooManyRequest, "验证码发送过于频繁，请稍后再试")
	}

	// 2. 生成 6 位随机验证码
	code := s.generateCode(6)

	// 3. 存储验证码 (有效时长 10 分钟)
	cacheKey := cache.KeyVerificationCode(scene, target)
	if err := s.cacheMgr.Set(ctx, cacheKey, code, 10*time.Minute); err != nil {
		return errorx.New(errorx.CodeInternalError, "验证码存储失败")
	}

	// 4. 设置频率限制
	_ = s.cacheMgr.Set(ctx, limitKey, "1", 60*time.Second)

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
	cacheKey := cache.KeyVerificationCode(scene, target)
	var storedCode string
	err := s.cacheMgr.Get(ctx, cacheKey, &storedCode)
	if err != nil {
		return false, nil // 验证码不存在或已过期
	}

	return storedCode == code, nil
}

func (s *verificationService) VerifyAndClearCode(ctx context.Context, scene, target, code string) (bool, error) {
	ok, err := s.VerifyCode(ctx, scene, target, code)
	if ok {
		cacheKey := cache.KeyVerificationCode(scene, target)
		_ = s.cacheMgr.Delete(ctx, cacheKey)
	}
	return ok, err
}

func (s *verificationService) generateCode(length int) string {
	const charset = "0123456789"
	result := make([]byte, length)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}
