package user

import (
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/utils"
	msgSvc "NetyAdmin/internal/service/message"
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"math/big"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

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

// sceneSpec 描述一个验证码业务场景的配置来源。
// 场景是否允许发送/校验、图形验证码开关键、消息验证配置键统一在此登记，
// 避免 GetVerifyConfig / sceneCaptchaKey / 发送白名单三份 switch 各自漂移。
type sceneSpec struct {
	captchaConfigKey string // captcha_config 图形验证码开关；空 = 该场景不校验图形验证码
	verifyEnabledKey string // user_config 消息验证开关；空 = 该场景不使用开关式验证配置
	verifyTypeKey    string // user_config 消息验证类型（email/sms）
}

var sceneSpecs = map[string]sceneSpec{
	SceneRegister:      {"user_register_enabled", "user_register_verify", "user_register_verify_type"},
	SceneResetPassword: {"user_reset_pwd_captcha_enabled", "user_reset_pwd_verify", "user_reset_pwd_verify_type"},
	SceneLogin:         {"user_login_enabled", "user_login_verify", "user_login_verify_type"},
	// 资料变更场景由已认证用户主动触发，无场景级开关/图形验证码配置
	SceneChangeEmail: {},
	SceneChangePhone: {},
	SceneBind:        {},
}

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
	spec, ok := sceneSpecs[scene]
	if !ok || spec.verifyEnabledKey == "" {
		return nil, errorx.New(errorx.CodeInvalidParams, "不支持的业务场景")
	}
	config := &VerifyConfig{Scene: scene}

	enabledVal, _ := s.watcher.GetConfig("user_config", spec.verifyEnabledKey)
	config.Enabled = utils.IsTruthy(enabledVal)

	typeVal, _ := s.watcher.GetConfig("user_config", spec.verifyTypeKey)
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
	captchaEnabled := utils.IsTruthy(val)

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
	// 场景白名单：未知 scene 会拆分频率限制键并允许任意模板发送，必须在入口拒绝
	// （与 GetVerifyConfig 的 default 分支 “不支持的业务场景” 语义一致）。
	if !isSupportedScene(scene) {
		return errorx.New(errorx.CodeInvalidParams, "不支持的业务场景")
	}
	// 0. 图形验证码二次校验 (Synergy)
	// 如果配置开启了图形验证码，则必须校验，防止接口轰炸。
	captchaEnabledKey := sceneCaptchaKey(scene)
	if captchaEnabledKey != "" {
		captchaVal, _ := s.watcher.GetConfig("captcha_config", captchaEnabledKey)
		captchaEnabled := utils.IsTruthy(captchaVal)
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
	// fail-closed：限流键查询异常（如 Redis 故障）时拒绝发送而非放行，
	// 避免故障窗口内验证码轰炸
	limitKey := cache.KeyVerifyCodeLimit(scene, target)
	exists, err := s.cacheSlow.Exists(ctx, limitKey)
	if err != nil {
		return errorx.New(errorx.CodeInternalError, "发送服务暂不可用，请稍后重试")
	}
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

// VerifyAndClearCode 原子校验并消费验证码（一次性语义）。
//
// 用 GETDEL 原子"取出+比对"替代原 Verify→Delete 两步：两步之间并发请求
// 可用同一验证码各通过一次。错误尝试仍保留 VerifyCode 的计数语义
// （5 次上限），校验失败时回写验证码以维持多次尝试窗口。
func (s *verificationService) VerifyAndClearCode(ctx context.Context, scene, target, code string) (bool, error) {
	// 尝试次数检查：超过 5 次验证码作废
	attemptKey := cache.KeyVerifyCodeAttempt(scene, target)
	var attemptStr string
	_ = s.cacheSlow.Get(ctx, attemptKey, &attemptStr)
	if n, err := strconv.Atoi(attemptStr); err == nil && n >= 5 {
		_ = s.cacheSlow.Delete(ctx, cache.KeyVerificationCode(scene, target))
		return false, nil
	}

	// 原子消费：取出即删除。
	// 区分"不存在"（redis.Nil：未发送/已消费/已过期 → 验证失败）
	// 与"故障"（其他错误 → fail-closed 返回错误，不能伪装成验证码错误，
	// 与 SendCode 的故障语义保持一致）
	var storedCode string
	if err := s.cacheSlow.GetAndDelete(ctx, cache.KeyVerificationCode(scene, target), &storedCode); err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil // 验证码不存在或已被消费
		}
		slog.Error("verify code: cache getdel failed", "scene", scene, "target", target, "err", err)
		return false, errorx.New(errorx.CodeInternalError, "验证服务暂不可用，请稍后重试")
	}

	if storedCode != code {
		// 错误尝试：计数 +1 并回写验证码（GETDEL 已删），维持 5 次尝试窗口。
		// 回写 TTL 取原有效期上限，最坏情况是该次错误尝试顺延过期时间。
		n, _ := strconv.Atoi(attemptStr)
		n++
		if err := s.cacheSlow.Set(ctx, attemptKey, strconv.Itoa(n), 10*time.Minute); err != nil {
			slog.Warn("set attempt count cache failed", "key", attemptKey, "err", err)
		}
		if err := s.cacheSlow.Set(ctx, cache.KeyVerificationCode(scene, target), storedCode, 10*time.Minute); err != nil {
			slog.Warn("restore verification code after wrong attempt failed", "scene", scene, "target", target, "err", err)
		}
		return false, nil
	}

	return true, nil
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

// sceneCaptchaKey 返回场景的图形验证码开关配置键；不支持或无配置时返回空串。
func sceneCaptchaKey(scene string) string {
	return sceneSpecs[scene].captchaConfigKey
}

// isSupportedScene 报告场景是否在注册表中（允许发送/校验验证码）。
func isSupportedScene(scene string) bool {
	_, ok := sceneSpecs[scene]
	return ok
}
