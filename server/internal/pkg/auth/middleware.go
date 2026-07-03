package auth

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"

	"NetyAdmin/internal/pkg/errorx"
	jwtPkg "NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/response"
)

// AccountCheckResult 是 LookupAccount 的返回值，封装账户校验的全部结果。
type AccountCheckResult struct {
	// Status 是账户当前状态。非 StatusEnabled 时中间件返回 CodeUserDisabled。
	// 已包含 TokenVersion 校验：版本号不匹配时 accessor 应返回特殊状态（如 StatusSessionExpired）
	// 或直接返回 error。
	Status string
	// SetContext 是注入 userId/roles 等到 gin.Context 的回调。
	// 仅在 Status == StatusEnabled 时被调用。
	SetContext func(c *gin.Context)
}

// ClaimsAccessor 抽象 admin/user 两端 claims 的差异字段访问。
//
// 设计原则（RULES.md §0.1 + 重构清单 B-AUTH-11）：
// JWTAuth 与 UserJWTAuth 两个中间件 90% 同构，差异集中在 claims 类型、
// tokenStore key 生成、账户查询三个点。通过本接口注入差异，骨架收敛到 RequireAuth。
type ClaimsAccessor[C jwtv5.Claims] interface {
	// NewClaims 返回一个空的 claims 实例，供 ParseWithClaims 填充。
	NewClaims() C

	// TokenStoreKey 从已解析的 claims 中提取 tokenStore 的 userID key。
	// admin 端返回 AdminTokenKey(claims.UserID)，user 端返回 claims.UID。
	TokenStoreKey(claims C) string

	// LookupAccount 查询账户并完成全部业务校验（启用状态 + TokenVersion 版本号）。
	// 返回 AccountCheckResult，中间件据此决定放行或拒绝。
	// 错误语义：返回 error 表示账户不存在或查询失败，中间件统一返回 CodeUserDisabled。
	LookupAccount(ctx context.Context, claims C) (*AccountCheckResult, error)
}

// RequireAuth 是 admin/user 两端 JWT 鉴权的通用骨架。
//
// 校验链：
//  1. Authorization: Bearer <token>
//  2. ParseToken（含 alg confusion 防护）
//  3. Subject 必须为 access token
//  4. tokenStore 哈希校验（tokenStore 关闭时跳过）
//  5. 账户查询 + 启用状态校验 + TokenVersion 校验（由 accessor.LookupAccount 完成）
//  6. 注入上下文（由 accessor 返回的 SetContext 回调执行）
func RequireAuth[C jwtv5.Claims](
	j *jwtPkg.JWT,
	tokenStore UserServiceTokenStore,
	accessor ClaimsAccessor[C],
) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c)
		if token == "" {
			response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
			c.Abort()
			return
		}

		claims := accessor.NewClaims()
		if err := j.ParseToken(token, claims); err != nil {
			response.FailWithCode(c, errorx.CodeTokenInvalid, "令牌无效")
			c.Abort()
			return
		}

		// Subject 必须为 access token
		subject, _ := claims.GetSubject()
		if subject != string(jwtPkg.AccessToken) {
			response.FailWithCode(c, errorx.CodeTokenInvalid, "令牌用途不正确")
			c.Abort()
			return
		}

		// 会话哈希校验：tokenStore 关闭（nil）时跳过
		if tokenStore != nil {
			userIDKey := accessor.TokenStoreKey(claims)
			if _, err := tokenStore.Get(c.Request.Context(), userIDKey, HashToken(token)); err != nil {
				response.FailWithCode(c, errorx.CodeUnauthorized, "会话已过期或已在别处登录")
				c.Abort()
				return
			}
		}

		// 账户查询 + 启用状态 + TokenVersion 校验
		// 校验细节（包括版本号比较）由 accessor.LookupAccount 内部完成，
		// 中间件只关心最终结果（status + setContext）。
		result, err := accessor.LookupAccount(c.Request.Context(), claims)
		if err != nil || result == nil {
			response.FailWithCode(c, errorx.CodeUserDisabled, "账户不存在")
			c.Abort()
			return
		}
		if result.Status != StatusEnabled {
			response.FailWithCode(c, errorx.CodeUserDisabled, "账户已被禁用或会话已失效")
			c.Abort()
			return
		}

		result.SetContext(c)
		c.Next()
	}
}

// StatusEnabled 是账户启用状态常量，与 entity.StatusEnabled 一致。
// 在 auth 包独立声明以避免 pkg/auth 反向依赖 domain/entity。
const StatusEnabled = "1"

// extractBearerToken 从 Authorization 头提取 Bearer token。
// 返回空字符串表示缺失或格式错误。
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
