package user

// user_auth.go - 客户端认证相关方法：注册、登录、刷新令牌、重置密码

import (
	"context"
	"strconv"
	"time"

	userEntity "NetyAdmin/internal/domain/entity/user"
	clientDto "NetyAdmin/internal/interface/client/dto/v1"

	"NetyAdmin/internal/domain/entity"
	authPkg "NetyAdmin/internal/pkg/auth"
	userVO "NetyAdmin/internal/domain/vo/user"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/password"
	"NetyAdmin/internal/pkg/utils"
)

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

	// 2. 校验密码强度
	if err := s.validatePasswordStrength(ctx, req.Password); err != nil {
		return "", err
	}

	// 3. 密码加密
	hashedPassword, err := password.Hash(req.Password)
	if err != nil {
		return "", errorx.New(errorx.CodeInternalError, "密码加密失败")
	}

	// 4. 创建实体
	user := &userEntity.User{
		ID:       utils.NewULID(),
		Username: req.Username,
		Password: hashedPassword,
		Nickname: req.Nickname,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   entity.StatusEnabled,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return "", errorx.New(errorx.CodeInternalError, "创建用户失败")
	}

	return user.ID, nil
}

func (s *userService) Login(ctx context.Context, req *clientDto.UserLoginReq, ip string) (*userVO.UserLoginVO, error) {
	// 1. 图形验证码校验 (captcha_config → user_login_enabled)
	captchaVal, _ := s.configWatcher.GetConfig("captcha_config", "user_login_enabled")
	captchaEnabled := captchaVal == "true" || captchaVal == "1"
	if captchaEnabled {
		if req.CaptchaKey == "" || req.CaptchaCode == "" {
			return nil, errorx.New(errorx.CodeCaptchaRequired, "验证码必填")
		}
		if !s.captchaStore.Verify(req.CaptchaKey, req.CaptchaCode, true) {
			return nil, errorx.New(errorx.CodeCaptchaInvalid, "验证码错误")
		}
	}

	// 2. 查找用户
	user, err := s.repo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}

	if user.Status == entity.StatusDisabled {
		return nil, errorx.New(errorx.CodeUserDisabled, "账户已禁用")
	}

	// 2.5 登录锁定检查
	lockKey := cache.KeyLoginLock(user.ID)
	var lockVal string
	if err := s.cacheMgr.Get(ctx, lockKey, &lockVal); err == nil && lockVal != "" {
		return nil, errorx.New(errorx.CodeUserLocked, "账户已锁定，请稍后再试")
	}

	// 3. 短信/邮箱验证码校验 (user_config → user_login_verify)
	verifyConfig, _ := s.verifySvc.GetVerifyConfig(ctx, SceneLogin)
	if verifyConfig != nil && verifyConfig.Enabled {
		if req.Code == "" {
			return nil, errorx.New(errorx.CodeCaptchaRequired, "验证码必填")
		}
		target := ""
		if verifyConfig.VerifyType == "email" && user.Email != "" {
			target = user.Email
		} else if verifyConfig.VerifyType == "sms" && user.Phone != "" {
			target = user.Phone
		}
		if target != "" {
			ok, _ := s.verifySvc.VerifyAndClearCode(ctx, SceneLogin, target, req.Code)
			if !ok {
				return nil, errorx.New(errorx.CodeCaptchaInvalid, "验证码错误或已过期")
			}
		}
	}

	// 4. 验证密码
	if err := password.Verify(user.Password, req.Password); err != nil {
		maxRetryStr, _ := s.configWatcher.GetConfig("user_config", "login_max_retry")
		lockDurationStr, _ := s.configWatcher.GetConfig("user_config", "login_lock_duration")
		maxRetry, _ := strconv.Atoi(maxRetryStr)
		lockDuration, _ := strconv.Atoi(lockDurationStr)
		if maxRetry <= 0 {
			maxRetry = 5
		}
		if lockDuration <= 0 {
			lockDuration = 3600
		}

		retryKey := cache.KeyLoginRetryCount(user.ID)
		var retryCount int
		var retryVal string
		if err := s.cacheMgr.Get(ctx, retryKey, &retryVal); err == nil && retryVal != "" {
			retryCount, _ = strconv.Atoi(retryVal)
		}
		retryCount++

		if retryCount >= maxRetry {
			lockKey := cache.KeyLoginLock(user.ID)
			_ = s.cacheMgr.Set(ctx, lockKey, "1", time.Duration(lockDuration)*time.Second)
			_ = s.cacheMgr.Delete(ctx, retryKey)
			return nil, errorx.New(errorx.CodeUserLocked, "密码错误次数过多，账户已锁定")
		}

		_ = s.cacheMgr.Set(ctx, retryKey, strconv.Itoa(retryCount), time.Duration(lockDuration)*time.Second)

		return nil, errorx.New(errorx.CodePasswordWrong, "密码错误")
	}

	retryKey := cache.KeyLoginRetryCount(user.ID)
	_ = s.cacheMgr.Delete(ctx, retryKey)

	// 5. 更新登录信息
	now := time.Now()
	user.LastLoginAt = &now
	user.LastLoginIP = ip
	_ = s.repo.Update(ctx, user)

	// 6. 生成令牌
	claims := s.jwt.NewUserClaims(user.ID, req.Platform, jwt.AccessToken, user.TokenVersion)
	token, err := s.jwt.GenerateToken(claims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "令牌生成失败")
	}

	refreshClaims := s.jwt.NewUserClaims(user.ID, req.Platform, jwt.RefreshToken, user.TokenVersion)
	refreshToken, err := s.jwt.GenerateToken(refreshClaims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "刷新令牌生成失败")
	}

	// 7. 存储 Token 哈希 (用于后续主动拉黑或单端登录控制)
	tokenHash := authPkg.HashToken(token)
	if err := s.tokenStore.Create(ctx, &userEntity.UserTokenHash{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiredAt: time.Unix(claims.ExpiresAt.Unix(), 0),
	}); err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "令牌存储失败")
	}

	refreshTokenHash := authPkg.HashToken(refreshToken)
	if err := s.tokenStore.Create(ctx, &userEntity.UserTokenHash{
		UserID:    user.ID,
		TokenHash: refreshTokenHash,
		ExpiredAt: time.Unix(refreshClaims.ExpiresAt.Unix(), 0),
	}); err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "刷新令牌存储失败")
	}

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

	blacklistKey := cache.KeyAuthBlacklistRefreshToken(refreshToken)
	exists, _ := s.cacheMgr.Exists(ctx, blacklistKey)
	if exists {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌已失效，请重新登录")
	}

	user, err := s.repo.GetByID(ctx, claims.UID)
	if err != nil {
		return nil, errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}
	if user.Status == entity.StatusDisabled {
		return nil, errorx.New(errorx.CodeUserDisabled, "账户已禁用")
	}

	newClaims := s.jwt.NewUserClaims(user.ID, claims.Platform, jwt.AccessToken, user.TokenVersion)
	token, err := s.jwt.GenerateToken(newClaims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成令牌失败")
	}

	newRefreshClaims := s.jwt.NewUserClaims(user.ID, claims.Platform, jwt.RefreshToken, user.TokenVersion)
	newRefreshToken, err := s.jwt.GenerateToken(newRefreshClaims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "刷新令牌失败")
	}

	// 刷新令牌：先失效该用户所有旧 token hash（含旧 access token，防止泄露后被继续使用），
	// 再写入新 access + refresh hash。
	// 注意：此处只清 tokenStore 哈希，不递增 TokenVersion——
	// refresh 不应失效其他设备的合法会话（版本号递增会波及所有设备）。
	if s.tokenStore != nil {
		_ = s.tokenStore.DeleteAll(ctx, user.ID)
	}
	tokenHash := authPkg.HashToken(token)
	if err := s.tokenStore.Create(ctx, &userEntity.UserTokenHash{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiredAt: time.Unix(newClaims.ExpiresAt.Unix(), 0),
	}); err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "令牌存储失败")
	}

	refreshTokenHash := authPkg.HashToken(newRefreshToken)
	if err := s.tokenStore.Create(ctx, &userEntity.UserTokenHash{
		UserID:    user.ID,
		TokenHash: refreshTokenHash,
		ExpiredAt: time.Unix(newRefreshClaims.ExpiresAt.Unix(), 0),
	}); err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "刷新令牌存储失败")
	}

	remainingTTL := time.Until(time.Unix(claims.ExpiresAt.Unix(), 0))
	if remainingTTL > 0 {
		_ = s.cacheMgr.Set(ctx, blacklistKey, "1", remainingTTL)
	}

	return &userVO.UserLoginVO{
		AccessToken:  token,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(newClaims.ExpiresAt.Unix() - time.Now().Unix()),
	}, nil
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

	if user.Status == entity.StatusDisabled {
		return errorx.New(errorx.CodeUserDisabled, "账户已禁用，无法找回密码")
	}

	// 校验密码强度
	if err := s.validatePasswordStrength(ctx, req.NewPassword); err != nil {
		return err
	}

	hashedPassword, err := password.Hash(req.NewPassword)
	if err != nil {
		return errorx.New(errorx.CodeInternalError, "密码加密失败")
	}
	user.Password = hashedPassword

	// 重置密码：失效旧 token + 递增版本号（fail-closed）
	if err := s.invalidateUserTokens(ctx, user.ID); err != nil {
		return errorx.New(errorx.CodeInternalError, "令牌失效失败")
	}

	return s.repo.Update(ctx, user)
}
