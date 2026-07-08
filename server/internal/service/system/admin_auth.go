// admin_auth.go 管理员认证：登录、登出、刷新令牌。
package system

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

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
	if err := s.cacheSlow.Get(ctx, lockKey, &lockVal); err == nil && lockVal != "" {
		return nil, errorx.New(errorx.CodeUserLocked, "账户已被锁定，请稍后再试")
	}

	admin, err := s.adminRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 统一登录失败文案为「用户名或密码错误」，消除用户名枚举（Task 3.4）。
			// 仅 Login 路径返回该文案；其他业务路径（如 GetByID）保留各自原有错误消息。
			// 错误码保留 CodeUserNotFound，便于内部审计/日志区分根因。
			return nil, errorx.New(errorx.CodeUserNotFound, "用户名或密码错误")
		}
		slog.Error("adminRepo.GetByUsername failed", "username", req.Username, "err", err)
		return nil, fmt.Errorf("adminRepo.GetByUsername: %w", err)
	}

	if !admin.IsEnabled() {
		// 统一登录失败文案（Task 3.4）：禁用账户在 Login 路径返回「用户名或密码错误」，
		// 避免攻击者通过差异响应枚举存在的用户名。错误码仍为 CodeUserDisabled 便于审计。
		return nil, errorx.New(errorx.CodeUserDisabled, "用户名或密码错误")
	}

	if err := password.Verify(admin.Password, req.Password); err != nil {
		lockKey := cache.KeyAdminLoginLock(req.Username)
		retryKey := cache.KeyAdminLoginRetryCount(req.Username)
		locked, msg := authPkg.HandlePasswordWrong(ctx, s.cacheSlow, lockKey, retryKey, authPkg.LoginLockConfig{
			MaxRetry:     5,
			LockDuration: 15 * time.Minute,
			RetryTTL:     10 * time.Minute,
		})
		if locked {
			return nil, errorx.New(errorx.CodeUserLocked, msg)
		}
		// 统一登录失败文案（Task 3.4）：密码错误时也返回「用户名或密码错误」。
		// 错误码保留 CodePasswordWrong 便于审计；msg（含剩余尝试次数）被覆盖以避免泄露用户存在性。
		return nil, errorx.New(errorx.CodePasswordWrong, "用户名或密码错误")
	}

	// 登录成功，清除失败计数
	authPkg.ClearLoginRetry(ctx, s.cacheSlow, cache.KeyAdminLoginRetryCount(req.Username))

	now := time.Now().Format(time.DateTime)
	// 使用 UpdateColumn 仅更新 last_login_at，避免 Save 覆盖并发修改及 Preload 关联
	if err := s.adminRepo.UpdateLastLoginAt(ctx, admin.ID, now); err != nil {
		slog.Warn("update last login at failed", "adminID", admin.ID, "err", err)
	}

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

// Logout 退出登录：删旧 access token hash + 将 refresh token 加入黑名单（TTL 为其剩余有效期）。
// 修复 P0-1 BUG：原实现仅删除 access token hash，refresh token 仍可用于换取新 access token。
// 此处将 refresh token 写入黑名单（与 RefreshToken 中相同黑名单 key），使后续 RefreshToken 调用被拒绝。
func (s *adminService) Logout(ctx context.Context, adminID uint, accessToken, refreshToken string) error {
	// 删旧 access token hash（tokenStore 为空时跳过，与原行为一致）
	if s.tokenStore != nil {
		tokenHash := authPkg.HashToken(accessToken)
		if err := s.tokenStore.Delete(ctx, authPkg.AdminTokenKey(adminID), tokenHash); err != nil {
			// 删除失败不阻断，继续处理 refresh token 黑名单
			slog.Warn("logout: delete access token hash failed", "adminID", adminID, "err", err)
		}
	}
	// 将 refresh token 写入黑名单，TTL 为其剩余有效期
	// 解析 refresh token 拿到 ExpiresAt（校验签名，仅取过期时间用于 TTL）；
	// ParseToken 失败（无效 token）则不写黑名单——反正无效 token 也用不了
	if refreshToken != "" {
		// Logout 黑名单写入失败不阻断：Logout 已删除 access token hash，
		// refresh blacklist 是纵深防御层；若 Logout 也 fail-closed 会导致用户无法退出。
		// RefreshToken 则必须 fail-closed：避免旧 refresh token 重放刷新。
		claims := &jwt.AdminClaims{}
		if err := s.jwt.ParseToken(refreshToken, claims); err == nil {
			remainingTTL := time.Until(time.Unix(claims.ExpiresAt.Unix(), 0))
			if remainingTTL > 0 {
				blacklistKey := cache.KeyAuthBlacklistRefreshToken(refreshToken)
				if err := s.cacheSlow.Set(ctx, blacklistKey, "1", remainingTTL); err != nil {
					slog.Error("logout: set refresh blacklist failed", "adminID", adminID, "err", err)
				}
			}
		} else {
			slog.Warn("logout: parse refresh token failed, skip blacklist",
				"adminID", adminID, "err", err)
		}
	}
	return nil
}

func (s *adminService) RefreshToken(ctx context.Context, refreshToken string) (*systemVO.LoginVO, error) {
	claims := &jwt.AdminClaims{}
	if err := s.jwt.ParseToken(refreshToken, claims); err != nil {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌无效")
	}
	if claims.Subject != string(jwt.RefreshToken) {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌无效")
	}

	// 检查 RefreshToken 是否在黑名单中（fail-closed：Exists 错误视为校验异常，拒绝刷新）
	blacklistKey := cache.KeyAuthBlacklistRefreshToken(refreshToken)
	exists, err := s.cacheSlow.Exists(ctx, blacklistKey)
	if err != nil {
		return nil, errorx.New(errorx.CodeUnauthorized, "会话校验异常，请重新登录")
	}
	if exists {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌已失效，请重新登录")
	}

	admin, err := s.adminRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeUserNotFound, "用户不存在")
		}
		slog.Error("adminRepo.GetByID failed", "adminID", claims.UserID, "err", err)
		return nil, fmt.Errorf("adminRepo.GetByID: %w", err)
	}
	if !admin.IsEnabled() {
		return nil, errorx.New(errorx.CodeUserDisabled)
	}

	// TokenVersion 校验（最严格方案）：改密/禁用/删除/角色变更等敏感操作会递增 DB.TokenVersion，
	// 任何持有旧 refresh token 的设备都会被拒绝，需重新登录。这是安全增强，避免旧会话绕过版本号。
	if claims.TokenVersion < admin.TokenVersion {
		return nil, errorx.New(errorx.CodeUnauthorized, "刷新令牌已失效，请重新登录")
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

	// 将旧的 RefreshToken 标记为作废（加入黑名单，TTL 对齐其原始过期时间，避免黑名单提前失效或长期驻留）
	// fail-closed：黑名单写入失败必须阻断刷新，否则旧 refresh token 仍可重放刷新（P1-1 修复）。
	blacklistKey = cache.KeyAuthBlacklistRefreshToken(refreshToken)
	remainingTTL := time.Until(time.Unix(claims.ExpiresAt.Unix(), 0))
	if remainingTTL > 0 {
		if err := s.cacheSlow.Set(ctx, blacklistKey, "1", remainingTTL); err != nil {
			slog.Error("blacklist refresh token failed, abort refresh to prevent replay", "err", err, "adminID", admin.ID)
			return nil, errorx.New(errorx.CodeInternalError, "会话状态异常，请重新登录")
		}
	}

	// 刷新令牌：仅删除当前会话的旧 refresh hash，再写入新 access + refresh token hash。
	// 不调用 DeleteAll——多设备登录场景下，刷新一个 token 不应踢掉该管理员其他设备的合法会话（P1-A 修复）。
	// 旧 access hash 不删：当前入参仅含旧 refresh token，无法定位旧 access hash；
	// 旧 access 由其自然过期或下次 Logout 清理，不影响其他设备。
	// 注：refresh 不递增 TokenVersion（不应失效其他设备合法会话）；版本号校验已在上方完成，
	// 仅作废当前 refresh token 即可，其他设备的合法 refresh token 不受影响。
	userIDKey := authPkg.AdminTokenKey(admin.ID)
	if err := authPkg.DeleteAndReplaceSession(ctx, s.tokenStore, userIDKey, refreshToken, token, newRefreshToken,
		time.Unix(newClaims.ExpiresAt.Unix(), 0), time.Unix(refreshClaims.ExpiresAt.Unix(), 0)); err != nil {
		return nil, errorx.New(errorx.CodeInternalError, "令牌存储失败")
	}

	return &systemVO.LoginVO{
		Token:        token,
		RefreshToken: newRefreshToken,
	}, nil
}
