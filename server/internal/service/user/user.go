package user

// user.go - 用户服务共享底层：userBase 结构体、构造函数，以及 admin/client 两端共用的横切方法
// （密码强度校验、登录锁清理、配置读取）。
//
// 拆分背景（spec D4）：原 UserService 同时依赖 admin/dto/user 和 client/dto/v1，
// 违反 BFF 多端隔离红线。现拆为 UserAdminService + UserClientService 两个独立接口，
// 各自仅 import 本端 DTO；二者共享 userBase 复用底层依赖与横切逻辑，避免重复代码。

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/mojocn/base64Captcha"

	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	passwordPkg "NetyAdmin/internal/pkg/password"
	storagePkg "NetyAdmin/internal/pkg/storage"
	userRepo "NetyAdmin/internal/repository/user"
)

// userBase 是 UserAdminService 与 UserClientService 共享的底层依赖与辅助方法集合。
// 通过嵌入复用密码强度校验、登录锁清理等横切逻辑，避免在两个 service 间复制粘贴。
// 业务方法（admin/client）不应直接定义在 userBase 上，而应分别定义在
// userAdminService / userClientService 上，以保持 BFF 端隔离的编译期可观测性。
type userBase struct {
	repo          userRepo.UserRepository
	userTokenRepo userRepo.UserTokenRepository
	jwt           *jwt.JWT
	verifySvc     VerificationService
	configWatcher configsync.ConfigWatcher
	storageMgr    *storagePkg.Manager
	captchaStore  base64Captcha.Store
	tokenStore    TokenStore
	cacheSlow     cache.SecurityCache
	tm            database.TxManager
}

// NewUserBase 构造共享底层依赖。由 wire 层调用一次，分别传给 NewUserAdminService / NewUserClientService。
// 返回未导出类型 userBase：调用方（wire）使用 := 推断持有，不能再命名声明，但可透传给同包导出构造函数。
func NewUserBase(
	repo userRepo.UserRepository,
	userTokenRepo userRepo.UserTokenRepository,
	jwtInstance *jwt.JWT,
	verifySvc VerificationService,
	configWatcher configsync.ConfigWatcher,
	storageMgr *storagePkg.Manager,
	captchaStore base64Captcha.Store,
	tokenStore TokenStore,
	cacheSlow cache.SecurityCache,
	tm database.TxManager,
) userBase {
	return userBase{
		repo:          repo,
		userTokenRepo: userTokenRepo,
		jwt:           jwtInstance,
		verifySvc:     verifySvc,
		configWatcher: configWatcher,
		storageMgr:    storageMgr,
		captchaStore:  captchaStore,
		tokenStore:    tokenStore,
		cacheSlow:     cacheSlow,
		tm:            tm,
	}
}

// validatePasswordStrength 校验用户密码强度（配置驱动）。
// 委托给 pkg/password.ValidateStrength，配置从 configWatcher 读取并覆盖默认值。
// admin（Create/Update）与 client（Register/ChangePassword/ResetPassword）共用。
func (b *userBase) validatePasswordStrength(ctx context.Context, password string) error {
	cfg := passwordPkg.DefaultUserStrengthConfig
	if v, err := strconv.Atoi(getConfig(b.configWatcher, "user_config", "password_min_length")); err == nil && v > 0 {
		cfg.MinLength = v
	}
	if v, err := strconv.Atoi(getConfig(b.configWatcher, "user_config", "password_require_types")); err == nil && v > 0 {
		cfg.RequireTypes = v
	}
	if err := passwordPkg.ValidateStrength(password, cfg); err != nil {
		// 不暴露内部校验细节，统一返回友好提示（spec B11）
		return errorx.New(errorx.CodePasswordTooWeak)
	}
	return nil
}

// clearLoginLockCache 清理用户登录锁定/重试计数缓存（Redis 操作，不进 DB 事务）。
// admin（Update/UpdateStatus/UnlockUser/Delete/DeleteBatch）与
// client（ChangePassword/ResetPassword/DeleteAccount）共用。
// 提取此 helper 消除多处复制粘贴（RULES.md §0.1 / 重构清单 B-AUTH-4）。
func (b *userBase) clearLoginLockCache(ctx context.Context, userID string) {
	lockKey := cache.KeyLoginLock(userID)
	if err := b.cacheSlow.Delete(ctx, lockKey); err != nil {
		slog.Warn("delete cache failed", "key", lockKey, "err", err)
	}
	retryKey := cache.KeyLoginRetryCount(userID)
	if err := b.cacheSlow.Delete(ctx, retryKey); err != nil {
		slog.Warn("delete cache failed", "key", retryKey, "err", err)
	}
}

// getConfig 是 configWatcher.GetConfig 的便捷封装，屏蔽 error 返回值。
// 提取自多处重复的 `val, _ := watcher.GetConfig(...)` 模式。
func getConfig(watcher configsync.ConfigWatcher, group, key string) string {
	val, _ := watcher.GetConfig(group, key)
	return val
}

// checkUserUnique 用户名/手机/邮箱唯一性预检（Register 与 admin Create 共享，原先两份 ~30 行同构）。
// Repo 错误仅 Warn 不阻断：DB 层唯一索引（0018 users email/phone + username）
// 会在 Create 阶段兜底拦截并发竞态，service 层捕获 23505 转业务码。
func (b *userBase) checkUserUnique(ctx context.Context, username, phone, email string) error {
	exists, err := b.repo.ExistsByUsername(ctx, username)
	if err != nil {
		slog.Warn("ExistsByUsername query failed (rely on DB unique constraint as fallback)",
			"username", username, "error", err)
	}
	if exists {
		return errorx.New(errorx.CodeUserAlreadyExists)
	}
	if phone != "" {
		exists, err = b.repo.ExistsByPhone(ctx, phone)
		if err != nil {
			slog.Warn("ExistsByPhone query failed (rely on DB unique constraint as fallback)",
				"phone", phone, "error", err)
		}
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "手机号已存在")
		}
	}
	if email != "" {
		exists, err = b.repo.ExistsByEmail(ctx, email)
		if err != nil {
			slog.Warn("ExistsByEmail query failed (rely on DB unique constraint as fallback)",
				"email", email, "error", err)
		}
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "邮箱已存在")
		}
	}
	return nil
}
