package user

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	userEntity "NetyAdmin/internal/domain/entity/user"
	clientDto "NetyAdmin/internal/interface/client/dto/v1"

	userVO "NetyAdmin/internal/domain/vo/user"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	storagePkg "NetyAdmin/internal/pkg/storage"
	"NetyAdmin/internal/pkg/utils"
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
	GetUploadToken(ctx context.Context, userID string) (interface{}, error)

	// Admin API
	List(ctx context.Context, current, size int, query *userRepo.UserRepoQuery) ([]userEntity.User, int64, error)
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
}

func NewUserService(repo userRepo.UserRepository, jwtInstance *jwt.JWT, verifySvc VerificationService, configWatcher configsync.ConfigWatcher, storageMgr *storagePkg.Manager) UserService {
	return &userService{
		repo:          repo,
		jwt:           jwtInstance,
		verifySvc:     verifySvc,
		configWatcher: configWatcher,
		storageMgr:    storageMgr,
	}
}

func (s *userService) Register(ctx context.Context, req *clientDto.UserRegisterReq) (string, error) {
	target := req.Phone
	if target == "" {
		target = req.Email
	}
	if target == "" {
		return "", errorx.New(errorx.CodeInvalidParams, "手机号或邮箱必填其一")
	}

	verifyConfig, _ := s.verifySvc.GetVerifyConfig(ctx, SceneRegister)
	if verifyConfig != nil && verifyConfig.Enabled {
		if req.Code == "" {
			return "", errorx.New(errorx.CodeCaptchaRequired, "验证码必填")
		}
		ok, err := s.verifySvc.VerifyAndClearCode(ctx, SceneRegister, target, req.Code)
		if err != nil || !ok {
			return "", errorx.New(errorx.CodeCaptchaInvalid, "验证码错误或已过期")
		}
	}

	// 1. 检查唯一性
	exists, _ := s.repo.ExistsByUsername(ctx, req.Username)
	if exists {
		return "", errorx.New(errorx.CodeUserAlreadyExists, "用户名已存在")
	}
	if req.Phone != "" {
		exists, _ = s.repo.ExistsByPhone(ctx, req.Phone)
		if exists {
			return "", errorx.New(errorx.CodeUserAlreadyExists, "手机号已存在")
		}
	}
	if req.Email != "" {
		exists, _ = s.repo.ExistsByEmail(ctx, req.Email)
		if exists {
			return "", errorx.New(errorx.CodeUserAlreadyExists, "邮箱已存在")
		}
	}

	// 2. 密码加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", errorx.New(errorx.CodeInternalError, "密码加密失败")
	}

	// 3. 创建实体
	user := &userEntity.User{
		ID:       utils.NewULID(),
		Username: req.Username,
		Password: string(hashedPassword),
		Nickname: req.Nickname,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   userEntity.UserStatusEnabled,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return "", errorx.New(errorx.CodeInternalError, "创建用户失败")
	}

	return user.ID, nil
}

func (s *userService) Login(ctx context.Context, req *clientDto.UserLoginReq, ip string) (*userVO.UserLoginVO, error) {
	// 1. 查找用户
	user, err := s.repo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}

	if user.Status == userEntity.UserStatusDisabled {
		return nil, errorx.New(errorx.CodeUserDisabled, "账户已禁用")
	}

	// 2. 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errorx.New(errorx.CodePasswordWrong, "密码错误")
	}

	// 3. 更新登录信息
	now := time.Now()
	user.LastLoginAt = &now
	user.LastLoginIP = ip
	_ = s.repo.Update(ctx, user)

	// 4. 生成令牌
	claims := s.jwt.NewUserClaims(user.ID, req.Platform, jwt.AccessToken)
	token, err := s.jwt.GenerateToken(claims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "令牌生成失败")
	}

	refreshClaims := s.jwt.NewUserClaims(user.ID, req.Platform, jwt.RefreshToken)
	refreshToken, err := s.jwt.GenerateToken(refreshClaims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "刷新令牌生成失败")
	}

	// 5. 存储 Token 哈希 (用于后续主动拉黑或单端登录控制)
	tokenHash := s.computeHash(token)
	err = s.repo.CreateTokenHash(ctx, &userEntity.UserTokenHash{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiredAt: time.Unix(claims.ExpiresAt.Unix(), 0),
	})

	return &userVO.UserLoginVO{
		AccessToken:  token,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(claims.ExpiresAt.Unix() - time.Now().Unix()),
	}, nil
}

func (s *userService) RefreshToken(ctx context.Context, refreshToken string) (*userVO.UserLoginVO, error) {
	claims := &jwt.UserClaims{}
	if err := s.jwt.ParseToken(refreshToken, claims); err != nil {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌无效")
	}
	if claims.Subject != string(jwt.RefreshToken) {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌无效")
	}

	// 获取用户信息以刷新权限/状态
	user, err := s.repo.GetByID(ctx, claims.UID)
	if err != nil {
		return nil, errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}
	if user.Status == userEntity.UserStatusDisabled {
		return nil, errorx.New(errorx.CodeUserDisabled, "账户已禁用")
	}

	// 生成新令牌对
	newClaims := s.jwt.NewUserClaims(user.ID, claims.Platform, jwt.AccessToken)
	token, _ := s.jwt.GenerateToken(newClaims)

	newRefreshClaims := s.jwt.NewUserClaims(user.ID, claims.Platform, jwt.RefreshToken)
	newRefreshToken, _ := s.jwt.GenerateToken(newRefreshClaims)

	// 记录新 Token 哈希
	tokenHash := s.computeHash(token)
	_ = s.repo.CreateTokenHash(ctx, &userEntity.UserTokenHash{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiredAt: time.Unix(newClaims.ExpiresAt.Unix(), 0),
	})

	return &userVO.UserLoginVO{
		AccessToken:  token,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(newClaims.ExpiresAt.Unix() - time.Now().Unix()),
	}, nil
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

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return errorx.New(errorx.CodePasswordWrong, "原密码错误")
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	user.Password = string(hashedPassword)

	return s.repo.Update(ctx, user)
}

func (s *userService) Logout(ctx context.Context, userID string, token string) error {
	tokenHash := s.computeHash(token)
	return s.repo.DeleteTokenHash(ctx, userID, tokenHash)
}

func (s *userService) ResetPassword(ctx context.Context, req *clientDto.UserResetPasswordReq) error {
	verifyConfig, _ := s.verifySvc.GetVerifyConfig(ctx, SceneResetPassword)
	if verifyConfig != nil && verifyConfig.Enabled {
		if req.Code == "" {
			return errorx.New(errorx.CodeCaptchaRequired, "验证码必填")
		}
		ok, err := s.verifySvc.VerifyAndClearCode(ctx, SceneResetPassword, req.Target, req.Code)
		if err != nil || !ok {
			return errorx.New(errorx.CodeCaptchaInvalid, "验证码错误或已过期")
		}
	}

	var user *userEntity.User
	var err error
	if utils.IsEmail(req.Target) {
		user, err = s.repo.GetByEmail(ctx, req.Target)
	} else {
		user, err = s.repo.GetByPhone(ctx, req.Target)
	}

	if err != nil {
		return errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}

	// 3. 修改密码
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	user.Password = string(hashedPassword)

	// 4. 清理所有 Token (强制下线)
	_ = s.repo.DeleteAllTokenHashes(ctx, user.ID)

	return s.repo.Update(ctx, user)
}

func (s *userService) List(ctx context.Context, current, size int, query *userRepo.UserRepoQuery) ([]userEntity.User, int64, error) {
	query.Current = current
	query.Size = size
	return s.repo.List(ctx, query)
}

func (s *userService) Create(ctx context.Context, user *userEntity.User) error {
	// 1. 检查唯一性
	exists, _ := s.repo.ExistsByUsername(ctx, user.Username)
	if exists {
		return errorx.New(errorx.CodeUserAlreadyExists, "用户名已存在")
	}
	if user.Phone != "" {
		exists, _ = s.repo.ExistsByPhone(ctx, user.Phone)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "手机号已存在")
		}
	}
	if user.Email != "" {
		exists, _ = s.repo.ExistsByEmail(ctx, user.Email)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "邮箱已存在")
		}
	}

	// 2. 密码加密
	if user.Password != "" {
		if err := s.validatePasswordStrength(ctx, user.Password); err != nil {
			return err
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return errorx.New(errorx.CodeInternalError, "密码加密失败")
		}
		user.Password = string(hashedPassword)
	}

	// 3. 设置 ID 和默认状态
	if user.ID == "" {
		user.ID = utils.NewULID()
	}
	if user.Status == "" {
		user.Status = userEntity.UserStatusEnabled
	}

	return s.repo.Create(ctx, user)
}

func (s *userService) Update(ctx context.Context, user *userEntity.User) error {
	oldUser, err := s.repo.GetByID(ctx, user.ID)
	if err != nil {
		return errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}

	// 1. 检查唯一性
	if user.Username != "" && user.Username != oldUser.Username {
		exists, _ := s.repo.ExistsByUsername(ctx, user.Username)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "用户名已存在")
		}
		oldUser.Username = user.Username
	}
	if user.Phone != "" && user.Phone != oldUser.Phone {
		exists, _ := s.repo.ExistsByPhone(ctx, user.Phone, user.ID)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "手机号已存在")
		}
		oldUser.Phone = user.Phone
	}
	if user.Email != "" && user.Email != oldUser.Email {
		exists, _ := s.repo.ExistsByEmail(ctx, user.Email, user.ID)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "邮箱已存在")
		}
		oldUser.Email = user.Email
	}

	// 2. 处理密码更新
	if user.Password != "" {
		if err := s.validatePasswordStrength(ctx, user.Password); err != nil {
			return err
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return errorx.New(errorx.CodeInternalError, "密码加密失败")
		}
		oldUser.Password = string(hashedPassword)
		// 强制清理 Token
		_ = s.repo.DeleteAllTokenHashes(ctx, user.ID)
	}

	// 3. 更新其他字段
	if user.Nickname != "" {
		oldUser.Nickname = user.Nickname
	}
	if user.Avatar != "" {
		oldUser.Avatar = user.Avatar
	}
	if user.Gender != "" {
		oldUser.Gender = user.Gender
	}
	if user.Status != "" {
		oldUser.Status = user.Status
	}

	return s.repo.Update(ctx, oldUser)
}

func (s *userService) UpdateStatus(ctx context.Context, id string, status string) error {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	user.Status = status

	// 如果是禁用用户，则强制清理其所有在线会话
	if status == userEntity.UserStatusDisabled {
		_ = s.repo.DeleteAllTokenHashes(ctx, id)
	}

	return s.repo.Update(ctx, user)
}

func (s *userService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *userService) DeleteBatch(ctx context.Context, ids []string) error {
	return s.repo.DeleteBatch(ctx, ids)
}

func (s *userService) UpdateLastReadID(ctx context.Context, userID string, lastReadID uint64) error {
	return s.repo.UpdateFields(ctx, userID, map[string]interface{}{
		"last_read_announcement_id": lastReadID,
	})
}

func (s *userService) DeleteAccount(ctx context.Context, userID string) error {
	// 1. 清理 Token
	_ = s.repo.DeleteAllTokenHashes(ctx, userID)
	// 2. 软删除用户
	return s.repo.Delete(ctx, userID)
}

func (s *userService) GetUploadToken(ctx context.Context, userID string) (interface{}, error) {
	storageSource, _ := s.configWatcher.GetConfig("user_config", "storage_module")

	if storageSource != "" {
		configID, err := strconv.ParseUint(storageSource, 10, 32)
		if err == nil && configID > 0 {
			presignedURL, err := s.storageMgr.GetPresignedUploadURL(ctx, uint(configID), "user/"+userID+"/", "application/octet-stream", 15*time.Minute)
			if err != nil {
				return nil, errorx.New(errorx.CodeInternalError, "获取上传凭证失败")
			}
			return gin.H{"uploadUrl": presignedURL, "storageConfigId": configID}, nil
		}
	}

	driver, config, err := s.storageMgr.GetDefaultDriver()
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "未配置默认存储源")
	}

	presignedURL, err := driver.GetPresignedUploadURL(ctx, "user/"+userID+"/", "application/octet-stream", 15*time.Minute)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "获取上传凭证失败")
	}

	return gin.H{"uploadUrl": presignedURL, "storageConfigId": config.ID}, nil
}

func (s *userService) validatePasswordStrength(ctx context.Context, password string) error {
	minLengthStr, _ := s.configWatcher.GetConfig("user_config", "password_min_length")
	minLength := 8
	if v, err := strconv.Atoi(minLengthStr); err == nil && v > 0 {
		minLength = v
	}

	requireTypesStr, _ := s.configWatcher.GetConfig("user_config", "password_require_types")
	requireTypes := 2
	if v, err := strconv.Atoi(requireTypesStr); err == nil && v > 0 {
		requireTypes = v
	}

	if len(password) < minLength {
		return fmt.Errorf("密码长度不能少于 %d 位", minLength)
	}

	types := 0
	if matched, _ := regexp.MatchString(`[a-z]`, password); matched {
		types++
	}
	if matched, _ := regexp.MatchString(`[A-Z]`, password); matched {
		types++
	}
	if matched, _ := regexp.MatchString(`[0-9]`, password); matched {
		types++
	}
	if matched, _ := regexp.MatchString(`[^a-zA-Z0-9]`, password); matched {
		types++
	}

	if types < requireTypes {
		return fmt.Errorf("密码必须包含数字、大小写字母、特殊符号中的至少 %d 种", requireTypes)
	}

	return nil
}

func (s *userService) computeHash(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}
