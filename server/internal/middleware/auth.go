package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/response"
	userService "NetyAdmin/internal/service/user"
	userRepoPkg "NetyAdmin/internal/repository/user"
	systemRepoPkg "NetyAdmin/internal/repository/system"

	"github.com/gin-gonic/gin"
)

var (
	jwtInstance *jwt.JWT
	userRepo    userRepoPkg.UserRepository
	tokenStore  userService.TokenStore
	adminRepo   systemRepoPkg.AdminRepository
)

func InitJWT(j *jwt.JWT, repo userRepoPkg.UserRepository, ts userService.TokenStore, ar systemRepoPkg.AdminRepository) {
	jwtInstance = j
	userRepo = repo
	tokenStore = ts
	adminRepo = ar
}

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.FailWithCode(c, errorx.CodeTokenInvalid, "令牌格式错误")
			c.Abort()
			return
		}

		token := parts[1]
		claims := &jwt.AdminClaims{}
		if err := jwtInstance.ParseToken(token, claims); err != nil {
			response.FailWithCode(c, errorx.CodeTokenInvalid, "令牌无效")
			c.Abort()
			return
		}

		if claims.Subject != string(jwt.AccessToken) {
			response.FailWithCode(c, errorx.CodeTokenInvalid, "令牌用途不正确")
			c.Abort()
			return
		}

		// 校验 token 是否仍在 tokenStore 中（支持改密码/禁用/登出后立即失效）
		if tokenStore != nil {
			tokenHash := sha256Hex(token)
			adminIDKey := "a:" + strconv.FormatUint(uint64(claims.UserID), 10)
			_, err := tokenStore.Get(c.Request.Context(), adminIDKey, tokenHash)
			if err != nil {
				response.FailWithCode(c, errorx.CodeUnauthorized, "会话已过期或已在别处登录")
				c.Abort()
				return
			}
		}

		// 校验管理员账户启用状态（禁用/删除后旧 token 立即失效）
		if adminRepo != nil {
			admin, err := adminRepo.GetByID(c.Request.Context(), claims.UserID)
			if err != nil || admin == nil || admin.Status != "1" {
				response.FailWithCode(c, errorx.CodeUserDisabled, "账户已被禁用或不存在")
				c.Abort()
				return
			}
		}

		c.Set("adminID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("roles", claims.Roles)
		c.Next()
	}
}

// sha256Hex 计算 token 的 sha256 十六进制哈希，与 service 层保持一致
func sha256Hex(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

func UserJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.FailWithCode(c, errorx.CodeTokenInvalid, "令牌格式错误")
			c.Abort()
			return
		}

		token := parts[1]
		claims := &jwt.UserClaims{}
		if err := jwtInstance.ParseToken(token, claims); err != nil {
			response.FailWithCode(c, errorx.CodeTokenInvalid, "令牌无效")
			c.Abort()
			return
		}

		if claims.Subject != string(jwt.AccessToken) {
			response.FailWithCode(c, errorx.CodeTokenInvalid, "令牌用途不正确")
			c.Abort()
			return
		}

		if tokenStore != nil {
			h := sha256.New()
			h.Write([]byte(token))
			tokenHash := hex.EncodeToString(h.Sum(nil))

			_, err := tokenStore.Get(c.Request.Context(), claims.UID, tokenHash)
			if err != nil {
				response.FailWithCode(c, errorx.CodeUnauthorized, "会话已过期或已在别处登录")
				c.Abort()
				return
			}
		}

		if userRepo != nil {
			user, err := userRepo.GetByID(c.Request.Context(), claims.UID)
			if err != nil || user == nil || user.Status != "1" {
				response.FailWithCode(c, errorx.CodeUserDisabled, "用户账户已被禁用或不存在")
				c.Abort()
				return
			}
		}

		c.Set("userID", claims.UID)
		c.Set("platform", claims.Platform)
		c.Next()
	}
}
