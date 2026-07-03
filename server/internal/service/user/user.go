package user

// user.go - 用户服务核心：接口定义、结构体、构造函数，以及个人资料、改密、登出、密码校验等基础方法

import (
	"context"
	"strconv"

	"github.com/mojocn/base64Captcha"

	userEntity "NetyAdmin/internal/domain/entity/user"
	clientDto "NetyAdmin/internal/interface/client/dto/v1"

	authPkg "NetyAdmin/internal/pkg/auth"
	userVO "NetyAdmin/internal/domain/vo/user"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	passwordPkg "NetyAdmin/internal/pkg/password"
	storagePkg "NetyAdmin/internal/pkg/storage"
	userRepo "NetyAdmin/internal/repository/user"
)

type UserService interface {
	// Client API
	Register(ctx context.Context, req *clientDto.UserRegisterReq) (string, error)
	Login(ctx context.Context, req *clientDto.UserLoginReq, ip string) (*userVO.UserLoginVO, error)
	RefreshToken(ctx context.Context, refreshToken string) (*userVO.UserLoginVO, error)
	GetInfo(ctx context.Context, userID string) (*userVO.UserInfoVO, error)
	UpdateProfile(ctx context.Context, userID string, req *clientDto.UserUpdateProfileReq) error
	ChangePassword(ctx context.Context, userID string, req *clientDto.UserChangePasswordReq) error
	Logout(ctx context.Context, userID string, token string) error
	ResetPassword(ctx context.Context, req *clientDto.UserResetPasswordReq) error
	DeleteAccount(ctx context.Context, userID string) error

	// Admin API
	List(ctx context.Context, current, size int, query *userRepo.UserRepoQuery) ([]userEntity.User, int64, error)
	SearchForAutocomplete(ctx context.Context, keyword string, limit int) ([]userEntity.User, error)
	Create(ctx context.Context, user *userEntity.User) error
	Update(ctx context.Context, user *userEntity.User) error
	UpdateStatus(ctx context.Context, id string, status string) error
	Delete(ctx context.Context, id string) error
	DeleteBatch(ctx context.Context, ids []string) error

	// Watermark
	UpdateLastReadID(ctx context.Context, userID string, lastReadID uint64) error
}

type userService struct {
	repo          userRepo.UserRepository
	jwt           *jwt.JWT
	verifySvc     VerificationService
	configWatcher configsync.ConfigWatcher
	storageMgr    *storagePkg.Manager
	captchaStore  base64Captcha.Store
	tokenStore    TokenStore
	cacheMgr      cache.LazyCacheManager
}

func NewUserService(repo userRepo.UserRepository, jwtInstance *jwt.JWT, verifySvc VerificationService, configWatcher configsync.ConfigWatcher, storageMgr *storagePkg.Manager, captchaStore base64Captcha.Store, tokenStore TokenStore, cacheMgr cache.LazyCacheManager) UserService {
	return &userService{
		repo:          repo,
		jwt:           jwtInstance,
		verifySvc:     verifySvc,
		configWatcher: configWatcher,
		storageMgr:    storageMgr,
		captchaStore:  captchaStore,
		tokenStore:    tokenStore,
		cacheMgr:      cacheMgr,
	}
}

func (s *userService) GetInfo(ctx context.Context, userID string) (*userVO.UserInfoVO, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}

	return &userVO.UserInfoVO{
		ID:          user.ID,
		Username:    user.Username,
		Nickname:    user.Nickname,
		Avatar:      user.Avatar,
		Phone:       user.Phone,
		Email:       user.Email,
		Gender:      user.Gender,
		Status:      user.Status,
		LastLoginAt: user.LastLoginAt,
	}, nil
}

func (s *userService) UpdateProfile(ctx context.Context, userID string, req *clientDto.UserUpdateProfileReq) error {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}

	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}
	if req.Gender != "" {
		user.Gender = req.Gender
	}
	if req.Email != "" {
		exists, _ := s.repo.ExistsByEmail(ctx, req.Email, userID)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "邮箱已占用")
		}
		user.Email = req.Email
	}
	if req.Phone != "" {
		exists, _ := s.repo.ExistsByPhone(ctx, req.Phone, userID)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "手机号已占用")
		}
		user.Phone = req.Phone
	}

	return s.repo.Update(ctx, user)
}

func (s *userService) ChangePassword(ctx context.Context, userID string, req *clientDto.UserChangePasswordReq) error {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}

	if err := passwordPkg.Verify(user.Password, req.OldPassword); err != nil {
		return errorx.New(errorx.CodePasswordWrong, "原密码错误")
	}

	// 新旧密码不能相同，防止用户设置相同密码降低安全性
	if req.NewPassword == req.OldPassword {
		return errorx.New(errorx.CodeInvalidParams, "新密码不能与旧密码相同")
	}

	// 校验密码强度
	if err := s.validatePasswordStrength(ctx, req.NewPassword); err != nil {
		return err
	}

	hashedPassword, err := passwordPkg.Hash(req.NewPassword)
	if err != nil {
		return errorx.New(errorx.CodeInternalError, "密码加密失败")
	}
	user.Password = hashedPassword

	// 改密：失效旧 token + 递增版本号（fail-closed）
	if err := s.invalidateUserTokens(ctx, userID); err != nil {
		return errorx.New(errorx.CodeInternalError, "令牌失效失败")
	}

	return s.repo.Update(ctx, user)
}

func (s *userService) Logout(ctx context.Context, userID string, token string) error {
	if s.tokenStore == nil {
		return nil
	}
	tokenHash := authPkg.HashToken(token)
	return s.tokenStore.Delete(ctx, userID, tokenHash)
}

// validatePasswordStrength 校验用户密码强度（配置驱动）。
// 委托给 pkg/password.ValidateStrength，配置从 configWatcher 读取并覆盖默认值。
func (s *userService) validatePasswordStrength(ctx context.Context, password string) error {
	cfg := passwordPkg.DefaultUserStrengthConfig
	if v, err := strconv.Atoi(getConfig(s.configWatcher, "user_config", "password_min_length")); err == nil && v > 0 {
		cfg.MinLength = v
	}
	if v, err := strconv.Atoi(getConfig(s.configWatcher, "user_config", "password_require_types")); err == nil && v > 0 {
		cfg.RequireTypes = v
	}
	if err := passwordPkg.ValidateStrength(password, cfg); err != nil {
		return errorx.New(errorx.CodePasswordTooWeak, err.Error())
	}
	return nil
}

// getConfig 是 configWatcher.GetConfig 的便捷封装，屏蔽 error 返回值。
// 提取自多处重复的 `val, _ := s.configWatcher.GetConfig(...)` 模式。
func getConfig(watcher configsync.ConfigWatcher, group, key string) string {
	val, _ := watcher.GetConfig(group, key)
	return val
}

// invalidateUserTokens 全局失效用户的所有旧 token（BUG #5 + 鉴权方案 C）。
//
// 职责切分（鉴权方案 C）：
//   - TokenVersion 专职"用户粒度的全局失效"：改密/禁用/删除时递增，旧 token 中间件比较版本号即拒
//   - tokenStore 专职"单 token 粒度的精确失效"：仅登出单设备时用 Delete(单哈希)
//
// 因此本函数只递增 TokenVersion + 清登录锁缓存，不调 tokenStore.DeleteAll
// （版本号已全局兜底，DeleteAll 是冗余操作，且增加 tokenStore 故障耦合）。
//
// 失败语义（fail-closed）：IncrementTokenVersion 失败时返回 error 阻断当前敏感操作。
func (s *userService) invalidateUserTokens(ctx context.Context, userID string) error {
	s.clearLoginLockCache(ctx, userID)
	return s.repo.IncrementTokenVersion(ctx, userID)
}