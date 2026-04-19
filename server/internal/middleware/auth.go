package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/response"
	userRepoPkg "NetyAdmin/internal/repository/user"

	"github.com/gin-gonic/gin"
)

var (
	jwtInstance *jwt.JWT
	userRepo    userRepoPkg.UserRepository
)

func InitJWT(j *jwt.JWT, repo userRepoPkg.UserRepository) {
	jwtInstance = j
	userRepo = repo
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

		c.Set("adminID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("roles", claims.Roles)
		c.Next()
	}
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

		// 增强安全性：校验 Token 哈希是否存在于数据库 (支持登出/拉黑)
		if userRepo != nil {
			h := sha256.New()
			h.Write([]byte(token))
			tokenHash := hex.EncodeToString(h.Sum(nil))

			hash, err := userRepo.GetTokenHash(c.Request.Context(), claims.UID, tokenHash)
			if err != nil || hash == nil {
				response.FailWithCode(c, errorx.CodeUnauthorized, "会话已过期或已在别处登录")
				c.Abort()
				return
			}

			// 校验用户状态 (防止禁用后 Token 依然有效)
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
