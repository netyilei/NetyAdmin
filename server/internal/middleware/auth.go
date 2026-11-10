package middleware

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	"NetyAdmin/internal/pkg/auth"
	"NetyAdmin/internal/pkg/cache"
	jwtPkg "NetyAdmin/internal/pkg/jwt"
	systemRepoPkg "NetyAdmin/internal/repository/system"
	userRepoPkg "NetyAdmin/internal/repository/user"
	userService "NetyAdmin/internal/service/user"
)

// adminAuthStateTTL 鉴权状态缓存 TTL。
// 取值权衡：
//   - 太长（>5min）：管理员被禁用后，旧 token 在 TTL 窗口内仍可通过鉴权（安全风险）
//   - 太短（<10s）：缓存命中率低，DB 压力未有效缓解
//   - 30s 平衡点：DB QPS 降低 30x+，禁用延迟最长 30s 内生效
//     TokenVersion 主动失效（IncrementTokenVersion + InvalidateByTags）作为兜底，
//     改密/禁用等敏感操作下旧 token 立即失效，不依赖 TTL 窗口。
const adminAuthStateTTL = 30 * time.Second

// AuthMiddleware 持有认证中间件的全部依赖，通过依赖注入构造，消除包级全局变量。
type AuthMiddleware struct {
	jwt           *jwtPkg.JWT
	userRepo      userRepoPkg.UserRepository
	userTokenRepo userRepoPkg.UserTokenRepository
	adminRepo     systemRepoPkg.AdminRepository
	tokenStore    userService.TokenStore
	cacheSlow     cache.SecurityCache
}

// NewAuthMiddleware 装配认证中间件依赖。j/userRepo/adminRepo 必须非空（fail-fast），
// tokenStore/cacheSlow/userTokenRepo 可为 nil（关闭相应能力）。
func NewAuthMiddleware(j *jwtPkg.JWT, repo userRepoPkg.UserRepository, utr userRepoPkg.UserTokenRepository, ts userService.TokenStore, ar systemRepoPkg.AdminRepository, cm cache.SecurityCache) *AuthMiddleware {
	if j == nil || repo == nil || ar == nil {
		panic("NewAuthMiddleware: j/userRepo/adminRepo 必须非空")
	}
	return &AuthMiddleware{
		jwt:           j,
		userRepo:      repo,
		userTokenRepo: utr,
		adminRepo:     ar,
		tokenStore:    ts,
		cacheSlow:     cm,
	}
}

// --- adminClaimsAccessor 实现 auth.ClaimsAccessor[*jwtPkg.AdminClaims] ---
//
// 把 admin 端的 claims 类型、tokenStore key、账户查询差异
// 注入到 auth.RequireAuth 通用骨架（重构清单 B-AUTH-11）。

type adminClaimsAccessor struct {
	mw *AuthMiddleware
}

func (adminClaimsAccessor) NewClaims() *jwtPkg.AdminClaims {
	return &jwtPkg.AdminClaims{}
}

func (adminClaimsAccessor) TokenStoreKey(claims *jwtPkg.AdminClaims) string {
	return auth.AdminTokenKey(claims.UserID)
}

// admin 端保留通用 tokenStore hash 校验（指向 admin_tokens 表）。
func (adminClaimsAccessor) SkipLegacyTokenStoreCheck() bool { return false }

func (a adminClaimsAccessor) LookupAccount(ctx context.Context, claims *jwtPkg.AdminClaims, _ string) (*auth.AccountCheckResult, error) {
	// 鉴权状态（token_version + status）走 L1+L2 缓存，DB QPS 降低 30x+。
	// 双写一致性：
	//   - 主动失效：Service 层 TM 事务（IncrementTokenVersion + Update/Delete）Commit 后调用
	//     invalidateAdminAuthStateCache 同步 InvalidateByTags(TagAdminAuthByID)
	//   - TTL 兜底：30s 过期保证极端情况下（PubSub 跨节点失效延迟）也能最终一致
	//   - TokenVersion 比较保证：即使 status 缓存有 30s 漂移，旧 token 因 claims.TokenVersion
	//     < DB.token_version 立即被拒，安全语义未被削弱
	//
	// cacheSlow 为 nil（缓存模块禁用）时降级为直查 DB，保持原 fail-closed 语义。
	var state *systemRepoPkg.AdminAuthState
	var err error
	if a.mw.cacheSlow != nil {
		key := cache.KeyAdminAuthState(claims.UserID)
		tags := []string{cache.TagAdminAuthByID(claims.UserID)}
		err = a.mw.cacheSlow.Fetch(ctx, key, "admin", tags, adminAuthStateTTL, &state, func() (interface{}, error) {
			return a.mw.adminRepo.GetAuthStateByID(ctx, claims.UserID)
		})
	} else {
		state, err = a.mw.adminRepo.GetAuthStateByID(ctx, claims.UserID)
	}
	if err != nil || state == nil {
		return nil, err
	}
	// TokenVersion 校验：旧 token 携带版本号 < 当前版本 → 视为会话失效
	if claims.TokenVersion < state.TokenVersion {
		return &auth.AccountCheckResult{Status: entity.StatusDisabled}, nil
	}
	return &auth.AccountCheckResult{
		Status: state.Status,
		SetContext: func(c *gin.Context) {
			c.Set("adminID", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("roles", claims.Roles)
		},
	}, nil
}

// --- userClaimsAccessor 实现 auth.ClaimsAccessor[*jwtPkg.UserClaims] ---

type userClaimsAccessor struct {
	mw *AuthMiddleware
	// expectedType 允许通过的 claims.Type（角色隔离锚点）：默认 user 路由 =
	// DefaultUserType；typed 路由 = 各自 userType（tech/merchant/rider…）——
	// 跨角色 token 在此被拒。经 UserClaimsAccessorFor / UserJWTAuth 构造，
	// 勿字面量零值直构（expectedType 为空会拒绝所有 token）。
	expectedType string
}

func (userClaimsAccessor) NewClaims() *jwtPkg.UserClaims {
	return &jwtPkg.UserClaims{}
}

func (userClaimsAccessor) TokenStoreKey(claims *jwtPkg.UserClaims) string {
	return claims.UID
}

// user 端会话已迁移到 user_tokens（按 platform 维度），通用 tokenStore 仍指向 admin_tokens
// （admin 专用），不能再走通用 hash 校验——否则所有 user 请求都因 admin_tokens 无记录被拒。
// user 端 hash 校验改由 LookupAccount 内部针对 user_tokens.access_hash 完成。
func (userClaimsAccessor) SkipLegacyTokenStoreCheck() bool { return true }

func (a userClaimsAccessor) LookupAccount(ctx context.Context, claims *jwtPkg.UserClaims, rawToken string) (*auth.AccountCheckResult, error) {
	// 角色类型隔离：accessor 只接受 type == expectedType 的 token。
	// 默认 UserJWTAuth 的 expectedType = DefaultUserType，只接受 type == "user"；
	// typed 角色（tech/merchant/rider…）各持独立 accessor（UserClaimsAccessorFor 构造），
	// token 不可跨角色互用：默认接口拒 typed token，typed 接口拒默认与其他 typed token。
	if claims.Type != a.expectedType {
		return &auth.AccountCheckResult{Status: entity.StatusDisabled}, nil
	}
	user, err := a.mw.userRepo.GetByID(ctx, claims.UID)
	if err != nil || user == nil {
		return nil, err
	}
	// 用户级 TokenVersion 校验：admin 后台敏感操作（改密/禁用/删除）递增 users.token_version
	// 后，旧 token 立即失效——顶掉该用户所有端的会话。
	if claims.TokenVersion < user.TokenVersion {
		return &auth.AccountCheckResult{Status: entity.StatusDisabled}, nil
	}
	// 端级 TokenVersion + hash 校验（user_tokens 表，按 platform 维度）：
	//   - 版本号校验：同 platform 重新登录后 token_version 被递增，旧 token 的 ptv < DB → 顶号。
	//   - hash 校验（纵深防御）：Logout 后 ClearHashes 清空 access_hash，下次请求 hash 不匹配 → 立即失效。
	// userTokenRepo 未注入（兼容老装配）→ 仅靠用户级版本号兜底（不阻断，便于灰度升级）。
	// 行不存在（旧基座签发的 token，无 user_tokens 记录）→ 同样跳过端级校验。
	if a.mw.userTokenRepo != nil && claims.Platform != "" {
		ut, utErr := a.mw.userTokenRepo.GetByPlatform(ctx, claims.UID, claims.Platform)
		if utErr == nil && ut != nil {
			if claims.PlatTokenVersion < ut.TokenVersion {
				return &auth.AccountCheckResult{Status: entity.StatusDisabled}, nil
			}
			// hash 比对（fail-closed）：行存在即必须精确匹配。
			// user_tokens 行由 Login 创建并立即写入非空 hash，空 hash 只会出现在
			// Logout 清空之后——此时必须拒绝而非视为"未设置"跳过
			// （跳过会使登出后的 access token 残留有效至 TTL，违背文档契约
			// "logout 失效当前 Token"）。
			if ut.AccessHash != auth.HashToken(rawToken) {
				return &auth.AccountCheckResult{Status: entity.StatusDisabled}, nil
			}
		} else if utErr != nil && !errors.Is(utErr, gorm.ErrRecordNotFound) {
			// DB 异常（非行不存在）→ fail-closed，避免故障窗口放过应被顶号的旧 token
			return nil, utErr
		}
	}
	return &auth.AccountCheckResult{
		Status: user.Status,
		SetContext: func(c *gin.Context) {
			c.Set("userID", claims.UID)
			c.Set("platform", claims.Platform)
		},
	}, nil
}

// JWTAuth 是 Admin 端 JWT 鉴权中间件。校验链详见 auth.RequireAuth 文档。
func (m *AuthMiddleware) JWTAuth() gin.HandlerFunc {
	return auth.RequireAuth(m.jwt, m.tokenStore, adminClaimsAccessor{mw: m})
}

// UserJWTAuth 是 Client 端用户 JWT 鉴权中间件。校验链同 JWTAuth。
func (m *AuthMiddleware) UserJWTAuth() gin.HandlerFunc {
	return auth.RequireAuth(m.jwt, m.tokenStore, m.UserClaimsAccessorFor(""))
}

// UserClaimsAccessorFor 返回指定 userType 的 user claims 访问器（typed 用户路由扩展，
// 配合 App.RegisterTypedAuthModule，见 docs/server-architecture.md §5.5）。
// 语义：仅接受 claims.Type == userType 的 token——与默认 user 及其他 typed 角色
// 严格隔离。userType 为空时等价于默认 user 类型（DefaultUserType）。
// LookupAccount 复用默认用户的账户校验链（users 表 + TokenVersion + user_tokens
// 端级校验）：下游角色复用 users 表体系时无需自写 accessor；自有账户体系的角色
// 请自行实现 auth.ClaimsAccessor[*jwtPkg.UserClaims] 并直传 TypedUserJWTAuth。
func (m *AuthMiddleware) UserClaimsAccessorFor(userType string) auth.ClaimsAccessor[*jwtPkg.UserClaims] {
	if userType == "" {
		userType = jwtPkg.DefaultUserType
	}
	return userClaimsAccessor{mw: m, expectedType: userType}
}

// TypedUserJWTAuth accepts an externally-provided ClaimsAccessor for custom user types
// (e.g. "tech", "merchant", "rider"). The base framework remains agnostic to the
// concrete userType semantics — the caller is responsible for account lookup and
// type validation within the accessor.
//
// This enables multi-role terminal authentication: each role gets its own accessor
// (querying its own table, enforcing its own status checks) while reusing the
// base framework's JWT parsing, tokenStore verification, and RequireAuth skeleton.
func (m *AuthMiddleware) TypedUserJWTAuth(accessor auth.ClaimsAccessor[*jwtPkg.UserClaims]) gin.HandlerFunc {
	return auth.RequireAuth(m.jwt, m.tokenStore, accessor)
}

// 编译期检查 accessor 实现 ClaimsAccessor 接口
var (
	_ auth.ClaimsAccessor[*jwtPkg.AdminClaims] = adminClaimsAccessor{}
	_ auth.ClaimsAccessor[*jwtPkg.UserClaims]  = userClaimsAccessor{}
)
