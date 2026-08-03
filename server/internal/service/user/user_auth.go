package user

// user_auth.go - 客户端用户服务：UserClientService 接口、userClientService 实现，
// 以及全部 client 端方法（注册、登录、刷新令牌、个人资料、改密、登出、注销、找回密码、发送验证码目标解析）。
//
// 仅 import client/dto/v1，不 import admin/dto/user，保证 BFF 端隔离（spec D4）。
// 共享横切逻辑（密码强度校验、登录锁清理）通过嵌入 userBase 复用。

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"gorm.io/gorm"

	userEntity "NetyAdmin/internal/domain/entity/user"
	clientDto "NetyAdmin/internal/interface/client/dto/v1"

	"NetyAdmin/internal/domain/entity"
	userVO "NetyAdmin/internal/domain/vo/user"
	authPkg "NetyAdmin/internal/pkg/auth"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	passwordPkg "NetyAdmin/internal/pkg/password"
	"NetyAdmin/internal/pkg/utils"
)

// UserClientService 是 client 端用户服务接口。
// 仅包含 client handler 调用的方法，入参为 client DTO（禁止 entity 入参，spec D4）。
type UserClientService interface {
	Register(ctx context.Context, req *clientDto.UserRegisterReq) (string, error)
	Login(ctx context.Context, req *clientDto.UserLoginReq, ip string) (*userVO.UserLoginVO, error)
	RefreshToken(ctx context.Context, refreshToken string) (*userVO.UserLoginVO, error)
	GetInfo(ctx context.Context, userID string) (*userVO.UserInfoVO, error)
	UpdateProfile(ctx context.Context, userID string, req *clientDto.UserUpdateProfileReq) error
	ChangePassword(ctx context.Context, userID string, req *clientDto.UserChangePasswordReq) error
	// Logout 退出登录：删除 access token hash，并将 refresh token 写入黑名单（TTL 为其剩余有效期），
	// 防止登出后 refresh token 仍可用于换取新 access token（P0-1 BUG 修复）。
	Logout(ctx context.Context, userID string, accessToken, refreshToken string) error
	ResetPassword(ctx context.Context, req *clientDto.UserResetPasswordReq) error
	DeleteAccount(ctx context.Context, userID string) error
	// ResolveSendCodeTarget 解析发送验证码目标。
	// 登录场景：根据 username 查找用户，校验状态，根据 verifyConfig.VerifyType 返回 email/phone 作为 target
	// 注册/找回密码场景：直接使用传入的 target，不查找用户
	// 返回值：verifyConfig（用于 handler 判断 enabled/type），target（用于实际发送），error
	ResolveSendCodeTarget(ctx context.Context, scene, username, target string) (*VerifyConfig, string, error)
}

type userClientService struct {
	userBase
}

// NewUserClientService 基于 userBase 构造 client 端用户服务。
func NewUserClientService(base userBase) UserClientService {
	return &userClientService{userBase: base}
}

func (s *userClientService) Register(ctx context.Context, req *clientDto.UserRegisterReq) (string, error) {
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
	// Repo 错误仅 Warn 不阻断：DB 真正不可用时后续 Create 会失败兜底，
	// DB 间歇故障时唯一性约束（DB 层 UNIQUE index）仍能在 Create 阶段拦截重复。
	// 不再静默吞错 `_ = ...`：失败需可观测，便于排查 DB 间歇故障。
	exists, existsErr := s.repo.ExistsByUsername(ctx, req.Username)
	if existsErr != nil {
		slog.Warn("ExistsByUsername query failed (rely on DB unique constraint as fallback)",
			"username", req.Username, "error", existsErr)
	}
	if exists {
		return "", errorx.New(errorx.CodeUserAlreadyExists, "用户名已存在")
	}
	if req.Phone != "" {
		exists, existsErr = s.repo.ExistsByPhone(ctx, req.Phone)
		if existsErr != nil {
			slog.Warn("ExistsByPhone query failed (rely on DB unique constraint as fallback)",
				"phone", req.Phone, "error", existsErr)
		}
		if exists {
			return "", errorx.New(errorx.CodeUserAlreadyExists, "手机号已存在")
		}
	}
	if req.Email != "" {
		exists, existsErr = s.repo.ExistsByEmail(ctx, req.Email)
		if existsErr != nil {
			slog.Warn("ExistsByEmail query failed (rely on DB unique constraint as fallback)",
				"email", req.Email, "error", existsErr)
		}
		if exists {
			return "", errorx.New(errorx.CodeUserAlreadyExists, "邮箱已存在")
		}
	}

	// 2. 校验密码强度
	if err := s.validatePasswordStrength(ctx, req.Password); err != nil {
		return "", err
	}

	// 3. 密码加密
	hashedPassword, err := passwordPkg.Hash(req.Password)
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

func (s *userClientService) Login(ctx context.Context, req *clientDto.UserLoginReq, ip string) (*userVO.UserLoginVO, error) {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 统一登录失败文案为「用户名或密码错误」，消除用户名枚举（Task 3.4）。
			// 仅 Login 路径返回该文案；RefreshToken/GetInfo 等其他路径保留各自原有错误消息。
			// 错误码保留 CodeUserNotFound 便于内部审计/日志区分根因。
			return nil, errorx.New(errorx.CodeUserNotFound, "用户名或密码错误")
		}
		slog.Error("repo.GetByUsername failed", "username", req.Username, "err", err)
		return nil, fmt.Errorf("repo.GetByUsername: %w", err)
	}

	if user.Status == entity.StatusDisabled {
		// 统一登录失败文案（Task 3.4）：禁用账户在 Login 路径返回「用户名或密码错误」，
		// 避免攻击者通过差异响应枚举存在的用户名。错误码仍为 CodeUserDisabled 便于审计。
		return nil, errorx.New(errorx.CodeUserDisabled, "用户名或密码错误")
	}

	// 2.5 登录锁定检查
	lockKey := cache.KeyLoginLock(user.ID)
	var lockVal string
	if err := s.cacheSlow.Get(ctx, lockKey, &lockVal); err == nil && lockVal != "" {
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
	if err := passwordPkg.Verify(user.Password, req.Password); err != nil {
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

		lockKey := cache.KeyLoginLock(user.ID)
		retryKey := cache.KeyLoginRetryCount(user.ID)
		locked, msg := authPkg.HandlePasswordWrong(ctx, s.cacheSlow, lockKey, retryKey, authPkg.LoginLockConfig{
			MaxRetry:     maxRetry,
			LockDuration: time.Duration(lockDuration) * time.Second,
			RetryTTL:     time.Duration(lockDuration) * time.Second,
		})
		if locked {
			return nil, errorx.New(errorx.CodeUserLocked, msg)
		}
		// 统一登录失败文案（Task 3.4）：密码错误时返回「用户名或密码错误」，
		// 避免攻击者通过差异响应枚举存在的用户名。错误码保留 CodePasswordWrong 便于审计；
		// msg（含剩余尝试次数）被覆盖以避免泄露用户存在性。
		return nil, errorx.New(errorx.CodePasswordWrong, "用户名或密码错误")
	}

	authPkg.ClearLoginRetry(ctx, s.cacheSlow, cache.KeyLoginRetryCount(user.ID))

	// 5. 更新登录信息
	// 注意：此处必须用 UpdateFields（列级 Updates），不能用 repo.Update（Save 全字段）。
	// 原因：Login 与 admin 端禁用/改密/删除等敏感操作可能并发——
	// 敏感操作通过 IncrementTokenVersion 递增 token_version 后，若 Login 仍走 Save 全字段，
	// 会用查询时拿到的旧 token_version / status 覆盖 DB 已提交的新值，
	// 导致「禁用用户被 Login 覆盖回启用态」或「版本号递增被覆盖回旧值」的并发覆盖 BUG。
	// 仅更新 last_login_at / last_login_ip 两个字段即可满足登录记录需求。
	// user.LastLoginAt / user.LastLoginIP 仍保留赋值，用于后续 VO 返回或日志输出。
	now := time.Now()
	user.LastLoginAt = &now
	user.LastLoginIP = ip
	if err := s.repo.UpdateFields(ctx, user.ID, map[string]interface{}{
		"last_login_at": now,
		"last_login_ip": ip,
	}); err != nil {
		slog.Warn("update user login info failed", "userID", user.ID, "err", err)
	}

	// 6-7. 端级顶号 + 令牌签发 + hash 落库（同一 TM 事务，避免版本号递增与 hash 写入部分失败）。
	//    UPSERT 原子递增 (user_id, platform) 的 token_version：
	//      - 同 platform 再次登录 → 版本号 +1 → 旧 token claims.ptv < 新版本 → 中间件拒绝（顶号）
	//      - 不同 platform → 各自独立行，互不影响（多端并存）
	//    事务边界：UpsertAndIncrement + UpdateHashes 必须同事务——
	//    若分开提交，第 6 步成功（版本号已 bump）但第 7 步失败（hash 未落库），
	//    会导致新 token 携带正确 ptv 但 DB 无 hash，Logout 后纵深防御失效（§5.2 红线）。
	txCtx, tx := s.tm.Begin(ctx)
	platTV, err := s.userTokenRepo.UpsertAndIncrement(txCtx, &userEntity.UserToken{
		UserID:   user.ID,
		Platform: req.Platform,
	})
	if err != nil {
		s.tm.Rollback(tx)
		slog.Error("UpsertAndIncrement user_tokens failed", "userID", user.ID, "platform", req.Platform, "err", err)
		return nil, errorx.New(errorx.CodeInternalError, "会话初始化失败")
	}

	claims := s.jwt.NewUserClaims(user.ID, req.Platform, jwt.DefaultUserType, jwt.AccessToken, user.TokenVersion, platTV)
	token, err := s.jwt.GenerateToken(claims)
	if err != nil {
		s.tm.Rollback(tx)
		return nil, errorx.New(errorx.CodeInternalError, "令牌生成失败")
	}

	refreshClaims := s.jwt.NewUserClaims(user.ID, req.Platform, jwt.DefaultUserType, jwt.RefreshToken, user.TokenVersion, platTV)
	refreshToken, err := s.jwt.GenerateToken(refreshClaims)
	if err != nil {
		s.tm.Rollback(tx)
		return nil, errorx.New(errorx.CodeInternalError, "刷新令牌生成失败")
	}

	// 回写 token hash（不递增版本——版本号已在 UpsertAndIncrement 递增过）。
	// 中间件 LookupAccount 比对 user_tokens.access_hash 做纵深防御（Logout 后立即失效）。
	accessExp := time.Unix(claims.ExpiresAt.Unix(), 0)
	refreshExp := time.Unix(refreshClaims.ExpiresAt.Unix(), 0)
	if err := s.userTokenRepo.UpdateHashes(txCtx, user.ID, req.Platform,
		authPkg.HashToken(token), authPkg.HashToken(refreshToken), accessExp, refreshExp); err != nil {
		s.tm.Rollback(tx)
		slog.Error("UpdateHashes user_tokens failed", "userID", user.ID, "platform", req.Platform, "err", err)
		return nil, errorx.New(errorx.CodeInternalError, "令牌存储失败")
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("commit user_tokens login transaction failed", "userID", user.ID, "platform", req.Platform, "err", err)
		return nil, errorx.New(errorx.CodeInternalError, "令牌存储失败")
	}

	return &userVO.UserLoginVO{
		AccessToken:  token,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(claims.ExpiresAt.Unix() - time.Now().Unix()),
	}, nil
}

func (s *userClientService) RefreshToken(ctx context.Context, refreshToken string) (*userVO.UserLoginVO, error) {
	claims := &jwt.UserClaims{}
	if err := s.jwt.ParseToken(refreshToken, claims); err != nil {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌无效")
	}
	if claims.Subject != string(jwt.RefreshToken) {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌无效")
	}

	blacklistKey := cache.KeyAuthBlacklistRefreshToken(refreshToken)
	exists, err := s.cacheSlow.Exists(ctx, blacklistKey)
	if err != nil {
		// fail-closed：缓存查询异常时拒绝刷新，避免失效令牌被重新签发
		return nil, errorx.New(errorx.CodeUnauthorized, "会话校验异常，请重新登录")
	}
	if exists {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌已失效，请重新登录")
	}

	user, err := s.repo.GetByID(ctx, claims.UID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeUserNotFound, "用户不存在")
		}
		slog.Error("repo.GetByID failed", "userID", claims.UID, "err", err)
		return nil, fmt.Errorf("repo.GetByID: %w", err)
	}
	if user.Status == entity.StatusDisabled {
		return nil, errorx.New(errorx.CodeUserDisabled, "账户已禁用")
	}
	// 最严格方案：比较 token 携带的版本号与 DB 当前版本号，版本号过期即拒（spec A1）。
	// 改密/禁用/删除等敏感操作递增 TokenVersion 后，旧 refresh token 立即失效。
	if claims.TokenVersion < user.TokenVersion {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌已失效，请重新登录")
	}

	// 端级版本校验：同 platform 重新登录后旧 refresh token 立即失效（顶号）。
	// user_tokens 行不存在（旧版本基座签发的 token / 首次登录前）→ 跳过端级校验，仅靠用户级版本兜底。
	ut, utErr := s.userTokenRepo.GetByPlatform(ctx, user.ID, claims.Platform)
	if utErr == nil && ut != nil {
		if claims.PlatTokenVersion < ut.TokenVersion {
			return nil, errorx.New(errorx.CodeUnauthorized, "该设备已有新登录，请重新登录")
		}
	} else if !errors.Is(utErr, gorm.ErrRecordNotFound) {
		// 非「行不存在」的 DB 错误 → fail-closed 拒绝刷新，避免故障窗口放过旧 token
		slog.Error("GetByPlatform user_tokens failed", "userID", user.ID, "platform", claims.Platform, "err", utErr)
		return nil, errorx.New(errorx.CodeInternalError, "会话校验异常，请重新登录")
	}

	newClaims := s.jwt.NewUserClaims(user.ID, claims.Platform, jwt.DefaultUserType, jwt.AccessToken, user.TokenVersion, claims.PlatTokenVersion)
	token, err := s.jwt.GenerateToken(newClaims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成令牌失败")
	}

	newRefreshClaims := s.jwt.NewUserClaims(user.ID, claims.Platform, jwt.DefaultUserType, jwt.RefreshToken, user.TokenVersion, claims.PlatTokenVersion)
	newRefreshToken, err := s.jwt.GenerateToken(newRefreshClaims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "刷新令牌失败")
	}

	// fail-closed：黑名单写入失败必须阻断刷新，否则旧 refresh token 仍可重放刷新（P1-1 修复）。
	// 顺序：先 Set 黑名单（fail-closed），后 DeleteAndReplaceSession 写新 token，
	// 与 admin_auth.go 保持一致——避免 Redis 抖动导致 Set 失败时新 token 已写入 tokenStore
	// （孤儿数据）而旧 refresh token 未进黑名单仍可重放。
	remainingTTL := time.Until(time.Unix(claims.ExpiresAt.Unix(), 0))
	if remainingTTL > 0 {
		if err := s.cacheSlow.Set(ctx, blacklistKey, "1", remainingTTL); err != nil {
			slog.Error("blacklist refresh token failed, abort refresh to prevent replay", "err", err, "userID", user.ID)
			return nil, errorx.New(errorx.CodeInternalError, "会话状态异常，请重新登录")
		}
	}

	// 刷新令牌：更新 user_tokens 当前 platform 行的 access/refresh hash（不递增版本号，会话延续语义）。
	// 不递增 token_version——版本号递增仅由 Login 负责（同 platform 顶号）；刷新是同一会话的延续。
	// 旧 access hash 不单独删：当前入参仅含旧 refresh token，无法定位旧 access hash；
	// 旧 access 由其自然过期或下次 Logout 清理，不影响其他设备。
	if err := s.userTokenRepo.UpdateHashes(ctx, user.ID, claims.Platform,
		authPkg.HashToken(token), authPkg.HashToken(newRefreshToken),
		time.Unix(newClaims.ExpiresAt.Unix(), 0), time.Unix(newRefreshClaims.ExpiresAt.Unix(), 0)); err != nil {
		slog.Warn("UpdateHashes on refresh failed", "userID", user.ID, "platform", claims.Platform, "err", err)
	}

	return &userVO.UserLoginVO{
		AccessToken:  token,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(newClaims.ExpiresAt.Unix() - time.Now().Unix()),
	}, nil
}

func (s *userClientService) GetInfo(ctx context.Context, userID string) (*userVO.UserInfoVO, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeUserNotFound, "用户不存在")
		}
		slog.Error("repo.GetByID failed", "userID", userID, "err", err)
		return nil, fmt.Errorf("repo.GetByID: %w", err)
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

func (s *userClientService) UpdateProfile(ctx context.Context, userID string, req *clientDto.UserUpdateProfileReq) error {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeUserNotFound, "用户不存在")
		}
		slog.Error("repo.GetByID failed", "userID", userID, "err", err)
		return fmt.Errorf("repo.GetByID: %w", err)
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

	// 邮箱变更需验证码（仅当邮箱真正变更时才校验，避免未变更也要求验证码）
	if req.Email != "" && req.Email != user.Email {
		if req.EmailCode == "" {
			return errorx.New(errorx.CodeCaptchaRequired, "邮箱变更需提供验证码")
		}
		ok, err := s.verifySvc.VerifyAndClearCode(ctx, SceneChangeEmail, req.Email, req.EmailCode)
		if err != nil || !ok {
			return errorx.New(errorx.CodeCaptchaInvalid, "邮箱验证码错误或已过期")
		}
		exists, _ := s.repo.ExistsByEmail(ctx, req.Email, userID)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "邮箱已占用")
		}
		user.Email = req.Email
	}

	// 手机变更需验证码（仅当手机真正变更时才校验，避免未变更也要求验证码）
	if req.Phone != "" && req.Phone != user.Phone {
		if req.PhoneCode == "" {
			return errorx.New(errorx.CodeCaptchaRequired, "手机变更需提供验证码")
		}
		ok, err := s.verifySvc.VerifyAndClearCode(ctx, SceneChangePhone, req.Phone, req.PhoneCode)
		if err != nil || !ok {
			return errorx.New(errorx.CodeCaptchaInvalid, "手机验证码错误或已过期")
		}
		exists, _ := s.repo.ExistsByPhone(ctx, req.Phone, userID)
		if exists {
			return errorx.New(errorx.CodeUserAlreadyExists, "手机号已占用")
		}
		user.Phone = req.Phone
	}

	return s.repo.Update(ctx, user)
}

func (s *userClientService) ChangePassword(ctx context.Context, userID string, req *clientDto.UserChangePasswordReq) error {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeUserNotFound, "用户不存在")
		}
		slog.Error("repo.GetByID failed", "userID", userID, "err", err)
		return fmt.Errorf("repo.GetByID: %w", err)
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

	// 改密：TM 单事务原子完成「递增 token_version + 更新用户」（fail-closed）。
	// 任一步失败整体回滚，避免「密码已改但版本号未递增」或「版本号递增但密码未改」的中间态（spec A3）。
	// clearLoginLockCache 在事务前调用（Redis 操作不进事务）。
	s.clearLoginLockCache(ctx, userID)
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.repo.IncrementTokenVersion(txCtx, userID); err != nil {
		slog.Error("user change password: increment token version failed", "userID", userID, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "令牌失效失败")
	}
	if err := s.repo.Update(txCtx, user); err != nil {
		slog.Error("user change password: update user failed", "userID", userID, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "用户更新失败")
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("user change password: commit failed", "userID", userID, "err", err)
		return errorx.New(errorx.CodeInternalError, "事务提交失败")
	}
	return nil
}

// Logout 退出登录：清空 user_tokens 当前 platform 的 hash + 将 refresh token 加入黑名单。
// 修复 P0-1 BUG：原实现仅删除 access token hash，refresh token 仍可用于换取新 access token。
// 此处将 refresh token 写入黑名单（与 RefreshToken 中相同黑名单 key），使后续 RefreshToken 调用被拒绝。
func (s *userClientService) Logout(ctx context.Context, userID string, accessToken, refreshToken string) error {
	// 从 accessToken 解析 platform，清空 user_tokens 该 platform 行的 hash。
	// hash 清空后中间件 hash 校验会拒绝后续携带该 token 的请求（纵深防御）。
	claims := &jwt.UserClaims{}
	if err := s.jwt.ParseToken(accessToken, claims); err == nil && claims.Platform != "" {
		if err := s.userTokenRepo.ClearHashes(ctx, userID, claims.Platform); err != nil {
			// 清空失败不阻断，继续处理 refresh token 黑名单（access token 也会自然过期）
			slog.Warn("logout: clear user_tokens hashes failed", "userID", userID, "platform", claims.Platform, "err", err)
		}
	}
	// 将 refresh token 写入黑名单，TTL 为其剩余有效期
	// 解析 refresh token 拿到 ExpiresAt（校验签名，仅取过期时间用于 TTL）；
	// ParseToken 失败（无效 token）则不写黑名单——反正无效 token 也用不了
	if refreshToken != "" {
		// Logout 黑名单写入失败不阻断：Logout 已删除 access token hash，
		// refresh blacklist 是纵深防御层；若 Logout 也 fail-closed 会导致用户无法退出。
		// RefreshToken 则必须 fail-closed：避免旧 refresh token 重放刷新。
		claims := &jwt.UserClaims{}
		if err := s.jwt.ParseToken(refreshToken, claims); err == nil {
			remainingTTL := time.Until(time.Unix(claims.ExpiresAt.Unix(), 0))
			if remainingTTL > 0 {
				blacklistKey := cache.KeyAuthBlacklistRefreshToken(refreshToken)
				if err := s.cacheSlow.Set(ctx, blacklistKey, "1", remainingTTL); err != nil {
					slog.Error("logout: set refresh blacklist failed", "userID", userID, "err", err)
				}
			}
		} else {
			slog.Warn("logout: parse refresh token failed, skip blacklist",
				"userID", userID, "err", err)
		}
	}
	return nil
}

func (s *userClientService) ResetPassword(ctx context.Context, req *clientDto.UserResetPasswordReq) error {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeUserNotFound, "用户不存在")
		}
		slog.Error("repo.GetByEmail/GetByPhone failed", "target", req.Target, "err", err)
		return fmt.Errorf("repo.GetByEmail/GetByPhone: %w", err)
	}

	if user.Status == entity.StatusDisabled {
		return errorx.New(errorx.CodeUserDisabled, "账户已禁用，无法找回密码")
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

	// 重置密码：TM 单事务原子完成「递增 token_version + 更新用户」（fail-closed）。
	// 任一步失败整体回滚，避免「密码已改但版本号未递增」或「版本号递增但密码未改」的中间态（spec A3）。
	// clearLoginLockCache 在事务前调用（Redis 操作不进事务）。
	s.clearLoginLockCache(ctx, user.ID)
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.repo.IncrementTokenVersion(txCtx, user.ID); err != nil {
		slog.Error("user reset password: increment token version failed", "userID", user.ID, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "令牌失效失败")
	}
	if err := s.repo.Update(txCtx, user); err != nil {
		slog.Error("user reset password: update user failed", "userID", user.ID, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "用户更新失败")
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("user reset password: commit failed", "userID", user.ID, "err", err)
		return errorx.New(errorx.CodeInternalError, "事务提交失败")
	}
	return nil
}

// DeleteAccount 注销账号：TM 单事务原子完成「递增 token_version + 软删除」（fail-closed）。
// 任一步失败整体回滚，避免「版本号已变但主数据未删」或「主数据已删但版本号未变」的中间态。
// clearLoginLockCache 在事务前调用（Redis 操作不进事务）。
func (s *userClientService) DeleteAccount(ctx context.Context, userID string) error {
	s.clearLoginLockCache(ctx, userID)
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.repo.IncrementTokenVersion(txCtx, userID); err != nil {
		slog.Error("user delete account: increment token version failed", "userID", userID, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "令牌失效失败")
	}
	if err := s.repo.Delete(txCtx, userID); err != nil {
		slog.Error("user delete account: delete user failed", "userID", userID, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "账号注销失败")
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("user delete account: commit failed", "userID", userID, "err", err)
		return errorx.New(errorx.CodeInternalError, "事务提交失败")
	}
	return nil
}

// ResolveSendCodeTarget 解析发送验证码目标（spec B10：handler 不再跨层调用 repository）。
//
// 业务逻辑（迁移自 client/handler/v1/auth_handler.go SendCode）：
//   - 登录场景（scene == "login"）：根据 username 查找用户 → 校验状态 → 获取 verifyConfig
//     → 据 verifyConfig.VerifyType 选择 user.Email 或 user.Phone 作为 target
//   - 注册/找回密码场景：直接使用传入的 target（手机号或邮箱），不查找用户
//
// 调用方职责：拿到 verifyConfig 与 target 后，调用 verifySvc.SendCode(scene, target, ...) 实际发送。
func (s *userClientService) ResolveSendCodeTarget(ctx context.Context, scene, username, target string) (*VerifyConfig, string, error) {
	if scene == SceneLogin {
		if username == "" {
			return nil, "", errorx.New(errorx.CodeInvalidParams, "登录场景需提供 username")
		}
		user, err := s.repo.GetByUsername(ctx, username)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, "", errorx.New(errorx.CodeUserNotFound, "用户不存在")
			}
			slog.Error("repo.GetByUsername failed", "username", username, "err", err)
			return nil, "", fmt.Errorf("repo.GetByUsername: %w", err)
		}
		if user.Status == entity.StatusDisabled {
			return nil, "", errorx.New(errorx.CodeUserDisabled, "账户已禁用")
		}

		verifyConfig, _ := s.verifySvc.GetVerifyConfig(ctx, scene)
		if verifyConfig == nil || !verifyConfig.Enabled {
			return nil, "", errorx.New(errorx.CodeInvalidParams, "当前场景未启用消息验证")
		}

		switch verifyConfig.VerifyType {
		case "email":
			if user.Email == "" {
				return nil, "", errorx.New(errorx.CodeInvalidParams, "该用户未绑定邮箱")
			}
			return verifyConfig, user.Email, nil
		case "sms":
			if user.Phone == "" {
				return nil, "", errorx.New(errorx.CodeInvalidParams, "该用户未绑定手机号")
			}
			return verifyConfig, user.Phone, nil
		default:
			return nil, "", errorx.New(errorx.CodeInvalidParams, "未配置验证方式")
		}
	}

	// 注册/找回密码场景：直接使用 target（手机号或邮箱）
	if target == "" {
		return nil, "", errorx.New(errorx.CodeInvalidParams, "需提供 target (手机号或邮箱)")
	}
	// 仍需返回 verifyConfig，便于 handler 决定是否要求图形验证码等
	verifyConfig, _ := s.verifySvc.GetVerifyConfig(ctx, scene)
	return verifyConfig, target, nil
}
