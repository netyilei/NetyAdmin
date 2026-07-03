package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	authPkg "NetyAdmin/internal/pkg/auth"
	"NetyAdmin/internal/domain/entity"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/response"
	userService "NetyAdmin/internal/service/user"
	userRepoPkg "NetyAdmin/internal/repository/user"
	systemRepoPkg "NetyAdmin/internal/repository/system"
)

// authDeps 是认证中间件的依赖集合。
//
// 设计原则（RULES.md §0.1 + §0.2）：
// 取代旧的包级全局变量（jwtInstance / userRepo / tokenStore / adminRepo），
// 依赖通过 InitJWT 在装配期一次性注入，调用方（router）仍按 middleware.JWTAuth() 注册，
// 但内部依赖由结构体持有，便于追踪、避免全局可变状态。
type authDeps struct {
	jwt       *jwt.JWT
	userRepo  userRepoPkg.UserRepository
	adminRepo systemRepoPkg.AdminRepository
	tokenStore userService.TokenStore
}

// deps 是装配期注入的依赖单例。
// 通过 InitJWT 设置，全进程只读不改。
var deps *authDeps

// InitJWT 装配认证中间件的依赖。必须在 router 注册前调用一次。
// 所有参数必须非空，违反即 panic（fail-fast，避免装配错误静默潜伏）。
func InitJWT(j *jwt.JWT, repo userRepoPkg.UserRepository, ts userService.TokenStore, ar systemRepoPkg.AdminRepository) {
	if j == nil || repo == nil || ar == nil {
		panic("InitJWT: 所有依赖必须非空")
	}
	deps = &authDeps{
		jwt:        j,
		userRepo:   repo,
		adminRepo:  ar,
		tokenStore: ts, // tokenStore 可为 nil（关闭会话存储时），中间件需相应跳过哈希校验
	}
}

// JWTAuth 是 Admin 端 JWT 鉴权中间件。
//
// 校验链：
//  1. Authorization: Bearer <token>
//  2. ParseToken（含 BUG #3 修复：校验签名方法为 HMAC）
//  3. Subject 必须为 access token
//  4. tokenStore 哈希校验（支持登出/改密后立即失效，tokenStore 关闭时跳过）
//  5. 账户启用状态校验（含 BUG #5 修复：Token 版本号校验）
//  6. 注入 adminID/username/roles 到 gin.Context
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c)
		if token == "" {
			response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
			c.Abort()
			return
		}

		claims := &jwt.AdminClaims{}
		if err := deps.jwt.ParseToken(token, claims); err != nil {
			response.FailWithCode(c, errorx.CodeTokenInvalid, "令牌无效")
			c.Abort()
			return
		}

		if claims.Subject != string(jwt.AccessToken) {
			response.FailWithCode(c, errorx.CodeTokenInvalid, "令牌用途不正确")
			c.Abort()
			return
		}

		// 会话哈希校验：tokenStore 关闭（nil）时跳过此步
		if deps.tokenStore != nil {
			adminKey := authPkg.AdminTokenKey(claims.UserID)
			if _, err := deps.tokenStore.Get(c.Request.Context(), adminKey, authPkg.HashToken(token)); err != nil {
				response.FailWithCode(c, errorx.CodeUnauthorized, "会话已过期或已在别处登录")
				c.Abort()
				return
			}
		}

		// 账户状态 + Token 版本号校验
		admin, err := deps.adminRepo.GetByID(c.Request.Context(), claims.UserID)
		if err != nil || admin == nil {
			response.FailWithCode(c, errorx.CodeUserDisabled, "账户不存在")
			c.Abort()
			return
		}
		if admin.Status != entity.StatusEnabled {
			response.FailWithCode(c, errorx.CodeUserDisabled, "账户已被禁用")
			c.Abort()
			return
		}
		// BUG #5：Token 版本号机制（纵深防御）
		// 敏感操作（改密/禁用/删除/角色变更）会递增 admin.TokenVersion；
		// 旧 token 携带的版本号严格小于当前版本 → 拒绝。
		if claims.TokenVersion < admin.TokenVersion {
			response.FailWithCode(c, errorx.CodeUnauthorized, "会话已失效，请重新登录")
			c.Abort()
			return
		}

		c.Set("adminID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("roles", claims.Roles)
		c.Next()
	}
}

// UserJWTAuth 是 Client 端用户 JWT 鉴权中间件。校验链同 JWTAuth。
func UserJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c)
		if token == "" {
			response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
			c.Abort()
			return
		}

		claims := &jwt.UserClaims{}
		if err := deps.jwt.ParseToken(token, claims); err != nil {
			response.FailWithCode(c, errorx.CodeTokenInvalid, "令牌无效")
			c.Abort()
			return
		}

		if claims.Subject != string(jwt.AccessToken) {
			response.FailWithCode(c, errorx.CodeTokenInvalid, "令牌用途不正确")
			c.Abort()
			return
		}

		if deps.tokenStore != nil {
			if _, err := deps.tokenStore.Get(c.Request.Context(), claims.UID, authPkg.HashToken(token)); err != nil {
				response.FailWithCode(c, errorx.CodeUnauthorized, "会话已过期或已在别处登录")
				c.Abort()
				return
			}
		}

		user, err := deps.userRepo.GetByID(c.Request.Context(), claims.UID)
		if err != nil || user == nil {
			response.FailWithCode(c, errorx.CodeUserDisabled, "用户账户不存在")
			c.Abort()
			return
		}
		if user.Status != entity.StatusEnabled {
			response.FailWithCode(c, errorx.CodeUserDisabled, "用户账户已被禁用")
			c.Abort()
			return
		}
		// BUG #5：Token 版本号机制
		if claims.TokenVersion < user.TokenVersion {
			response.FailWithCode(c, errorx.CodeUnauthorized, "会话已失效，请重新登录")
			c.Abort()
			return
		}

		c.Set("userID", claims.UID)
		c.Set("platform", claims.Platform)
		c.Next()
	}
}

// extractBearerToken 从 Authorization 头提取 Bearer token。
// 返回空字符串表示缺失或格式错误，由调用方决定如何响应。
func extractBearerToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return parts[1]
}
