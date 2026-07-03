package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

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

// authDeps 是认证中间件的依赖集合，在装配期通过 InitJWT 注入。
//
// 设计原则（RULES.md §0.1 + §0.2）：
// 取代旧的包级全局可变变量（jwtInstance / userRepo / tokenStore / adminRepo），
// 依赖通过结构体持有，便于追踪、避免全局可变状态。
type authDeps struct {
	jwt        *jwtPkg.JWT
	userRepo   userRepoPkg.UserRepository
	adminRepo  systemRepoPkg.AdminRepository
	tokenStore userService.TokenStore
	cacheMgr   cache.LazyCacheManager
}

// deps 是装配期注入的依赖单例。全进程只读不改。
var deps *authDeps

// InitJWT 装配认证中间件的依赖。必须在 router 注册前调用一次。
// j/userRepo/adminRepo 必须非空（fail-fast），tokenStore/cacheMgr 可为 nil（关闭相应能力）。
func InitJWT(j *jwtPkg.JWT, repo userRepoPkg.UserRepository, ts userService.TokenStore, ar systemRepoPkg.AdminRepository, cm cache.LazyCacheManager) {
	if j == nil || repo == nil || ar == nil {
		panic("InitJWT: j/userRepo/adminRepo 必须非空")
	}
	deps = &authDeps{
		jwt:        j,
		userRepo:   repo,
		adminRepo:  ar,
		tokenStore: ts,
		cacheMgr:   cm,
	}
}

// --- adminClaimsAccessor 实现 auth.ClaimsAccessor[*jwtPkg.AdminClaims] ---
//
// 把 admin 端的 claims 类型、tokenStore key、账户查询差异
// 注入到 auth.RequireAuth 通用骨架（重构清单 B-AUTH-11）。

type adminClaimsAccessor struct{}

func (adminClaimsAccessor) NewClaims() *jwtPkg.AdminClaims {
	return &jwtPkg.AdminClaims{}
}

func (adminClaimsAccessor) TokenStoreKey(claims *jwtPkg.AdminClaims) string {
	return auth.AdminTokenKey(claims.UserID)
}

func (adminClaimsAccessor) LookupAccount(ctx context.Context, claims *jwtPkg.AdminClaims) (*auth.AccountCheckResult, error) {
	// 鉴权状态（token_version + status）走 L1+L2 缓存，DB QPS 降低 30x+。
	// 双写一致性：
	//   - 主动失效：invalidateAdminTokens 递增 token_version 时同步 InvalidateByTags
	//   - TTL 兜底：30s 过期保证极端情况下（PubSub 跨节点失效延迟）也能最终一致
	//   - TokenVersion 比较保证：即使 status 缓存有 30s 漂移，旧 token 因 claims.TokenVersion
	//     < DB.token_version 立即被拒，安全语义未被削弱
	//
	// cacheMgr 为 nil（缓存模块禁用）时降级为直查 DB，保持原 fail-closed 语义。
	var state *systemRepoPkg.AdminAuthState
	var err error
	if deps.cacheMgr != nil {
		key := cache.KeyAdminAuthState(claims.UserID)
		tags := []string{cache.TagAdminAuthByID(claims.UserID)}
		err = deps.cacheMgr.Fetch(ctx, key, "admin", tags, adminAuthStateTTL, &state, func() (interface{}, error) {
			return deps.adminRepo.GetAuthStateByID(ctx, claims.UserID)
		})
	} else {
		state, err = deps.adminRepo.GetAuthStateByID(ctx, claims.UserID)
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

type userClaimsAccessor struct{}

func (userClaimsAccessor) NewClaims() *jwtPkg.UserClaims {
	return &jwtPkg.UserClaims{}
}

func (userClaimsAccessor) TokenStoreKey(claims *jwtPkg.UserClaims) string {
	return claims.UID
}

func (userClaimsAccessor) LookupAccount(ctx context.Context, claims *jwtPkg.UserClaims) (*auth.AccountCheckResult, error) {
	user, err := deps.userRepo.GetByID(ctx, claims.UID)
	if err != nil || user == nil {
		return nil, err
	}
	// TokenVersion 校验
	if claims.TokenVersion < user.TokenVersion {
		return &auth.AccountCheckResult{Status: entity.StatusDisabled}, nil
	}
	return &auth.AccountCheckResult{
		Status: user.Status,
		SetContext: func(c *gin.Context) {
			c.Set("userID", claims.UID)
			c.Set("platform", claims.Platform)
		},
	}, nil
}

// 包级单例 accessor（无状态，复用）
var (
	adminClaimsAcc = adminClaimsAccessor{}
	userClaimsAcc  = userClaimsAccessor{}
)

// JWTAuth 是 Admin 端 JWT 鉴权中间件。校验链详见 auth.RequireAuth 文档。
func JWTAuth() gin.HandlerFunc {
	return auth.RequireAuth(deps.jwt, deps.tokenStore, adminClaimsAcc)
}

// UserJWTAuth 是 Client 端用户 JWT 鉴权中间件。校验链同 JWTAuth。
func UserJWTAuth() gin.HandlerFunc {
	return auth.RequireAuth(deps.jwt, deps.tokenStore, userClaimsAcc)
}

// 编译期检查 accessor 实现 ClaimsAccessor 接口
var (
	_ auth.ClaimsAccessor[*jwtPkg.AdminClaims] = adminClaimsAcc
	_ auth.ClaimsAccessor[*jwtPkg.UserClaims]  = userClaimsAcc
)
