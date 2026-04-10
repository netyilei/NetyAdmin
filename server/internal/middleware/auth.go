package middleware

import (
	"strings"

	"netyadmin/internal/pkg/errorx"
	"netyadmin/internal/pkg/jwt"
	"netyadmin/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

var jwtInstance *jwt.JWT

func InitJWT(j *jwt.JWT) {
	jwtInstance = j
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
		claims, err := jwtInstance.ParseToken(token)
		if err != nil {
			response.FailWithCode(c, errorx.CodeTokenInvalid, "令牌无效")
			c.Abort()
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("roles", claims.Roles)
		c.Next()
	}
}
