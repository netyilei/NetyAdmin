// admin_auth.go 管理员认证：登录、登出、刷新令牌。
package system

import (
	"context"
	"time"

	systemVO "NetyAdmin/internal/domain/vo/system"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"

	authPkg "NetyAdmin/internal/pkg/auth"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/password"
)

func (s *adminService) Login(ctx context.Context, req *systemDto.LoginReq) (*systemVO.LoginVO, error) {
	// 检查账户是否被锁定
	lockKey := cache.KeyAdminLoginLock(req.Username)
	var lockVal string
	if err := s.cacheMgr.Get(ctx, lockKey, &lockVal); err == nil && lockVal != "" {
		return nil, errorx.New(errorx.CodeUserLocked, "账户已被锁定，请稍后再试")
	}

	admin, err := s.adminRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}

	if !admin.IsEnabled() {
		return nil, errorx.New(errorx.CodeUserDisabled)
	}

	if err := password.Verify(admin.Password, req.Password); err != nil {
		lockKey := cache.KeyAdminLoginLock(req.Username)
		retryKey := cache.KeyAdminLoginRetryCount(req.Username)
		locked, msg := authPkg.HandlePasswordWrong(ctx, s.cacheMgr, lockKey, retryKey, authPkg.LoginLockConfig{
			MaxRetry:     5,
			LockDuration: 15 * time.Minute,
			RetryTTL:     10 * time.Minute,
		})
		if locked {
			return nil, errorx.New(errorx.CodeUserLocked, msg)
		}
		return nil, errorx.New(errorx.CodePasswordWrong, msg)
	}

	// 登录成功，清除失败计数
	authPkg.ClearLoginRetry(ctx, s.cacheMgr, cache.KeyAdminLoginRetryCount(req.Username))

	now := time.Now().Format(time.DateTime)
	// 使用 UpdateColumn 仅更新 last_login_at，避免 Save 覆盖并发修改及 Preload 关联
	_ = s.adminRepo.UpdateLastLoginAt(ctx, admin.ID, now)

	roles := admin.RoleCodes()
	claims := s.jwt.NewAdminClaims(admin.ID, admin.Username, roles, jwt.AccessToken, admin.TokenVersion)
	token, err := s.jwt.GenerateToken(claims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成令牌失败")
	}

	refreshClaims := s.jwt.NewAdminClaims(admin.ID, admin.Username, roles, jwt.RefreshToken, admin.TokenVersion)
	refreshToken, err := s.jwt.GenerateToken(refreshClaims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成刷新令牌失败")
	}

	// 写入 token hash 到 tokenStore，使中间件可校验会话有效性，支持改密码/禁用/登出后立即失效
	userIDKey := authPkg.AdminTokenKey(admin.ID)
	if err := authPkg.StoreSessionPair(ctx, s.tokenStore, userIDKey, token, refreshToken,
		time.Unix(claims.ExpiresAt.Unix(), 0), time.Unix(refreshClaims.ExpiresAt.Unix(), 0)); err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "令牌存储失败")
	}

	// 注：登录成功后不再主动写入角色的硬编码 Redis，
	// 我们将在权限拦截器里使用 LazyCacheManager 进行透明加载 (Fetch)

	return &systemVO.LoginVO{
		Token:        token,
		RefreshToken: refreshToken,
	}, nil
}

func (s *adminService) Logout(ctx context.Context, adminID uint, token string) error {
	if s.tokenStore == nil {
		return nil
	}
	tokenHash := authPkg.HashToken(token)
	return s.tokenStore.Delete(ctx, authPkg.AdminTokenKey(adminID), tokenHash)
}

func (s *adminService) RefreshToken(ctx context.Context, refreshToken string) (*systemVO.LoginVO, error) {
	claims := &jwt.AdminClaims{}
	if err := s.jwt.ParseToken(refreshToken, claims); err != nil {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌无效")
	}
	if claims.Subject != string(jwt.RefreshToken) {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌无效")
	}

	// 检查 RefreshToken 是否在黑名单中
	blacklistKey := cache.KeyAuthBlacklistRefreshToken(refreshToken)
	exists, _ := s.cacheMgr.Exists(ctx, blacklistKey)
	if exists {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌已失效，请重新登录")
	}

	admin, err := s.adminRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, errorx.New(errorx.CodeUserNotFound, "用户不存在")
	}
	if !admin.IsEnabled() {
		return nil, errorx.New(errorx.CodeUserDisabled)
	}

	roles := admin.RoleCodes()
	newClaims := s.jwt.NewAdminClaims(admin.ID, admin.Username, roles, jwt.AccessToken, admin.TokenVersion)
	token, err := s.jwt.GenerateToken(newClaims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成令牌失败")
	}

	refreshClaims := s.jwt.NewAdminClaims(admin.ID, admin.Username, roles, jwt.RefreshToken, admin.TokenVersion)
	newRefreshToken, err := s.jwt.GenerateToken(refreshClaims)
	if err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "生成刷新令牌失败")
	}

	// 将旧的 RefreshToken 标记为作废（加入黑名单，24小时过期）
	blacklistKey = cache.KeyAuthBlacklistRefreshToken(refreshToken)
	_ = s.cacheMgr.Set(ctx, blacklistKey, "1", 24*time.Hour)

	// 刷新令牌：失效该管理员的所有旧 token（包括旧 AccessToken），然后写入新 access + refresh token hash
	// 这样可保证旧 AccessToken 在刷新后立即失效，防止 Token 泄露后被继续使用
	// 注意：refresh 不递增 TokenVersion（不应失效其他设备合法会话）
	userIDKey := authPkg.AdminTokenKey(admin.ID)
	if err := authPkg.ReplaceSessionForRefresh(ctx, s.tokenStore, userIDKey, token, newRefreshToken,
		time.Unix(newClaims.ExpiresAt.Unix(), 0), time.Unix(refreshClaims.ExpiresAt.Unix(), 0)); err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "令牌存储失败")
	}

	return &systemVO.LoginVO{
		Token:        token,
		RefreshToken: newRefreshToken,
	}, nil
}
