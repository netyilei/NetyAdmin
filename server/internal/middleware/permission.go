package middleware

import (
	"context"
	"log/slog"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

type AuthVerifier interface {
	VerifyApiAuth(ctx context.Context, method, path string, roleCodes []string) (hasPermission bool, apiFound bool, err error)
}

var whiteListPaths = []string{
	"/admin/v1/auth/login",
	"/admin/v1/auth/refreshToken",
	"/admin/v1/auth/getUserInfo",
	"/admin/v1/auth/profile",
	"/admin/v1/auth/changePassword",
}

func isWhiteListPath(path string) bool {
	for _, p := range whiteListPaths {
		if path == p {
			return true
		}
	}
	return false
}

func PermissionAuth(authVerifier AuthVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		path := c.Request.URL.Path

		if isWhiteListPath(path) {
			c.Next()
			return
		}

		userID, exists := c.Get("userID")
		if !exists {
			response.FailWithCode(c, errorx.CodeUnauthorized, "未授权，请先登录")
			c.Abort()
			return
		}

		_ = userID

		roles, _ := c.Get("roles")
		roleCodes := roles.([]string)

		if len(roleCodes) == 0 {
			response.FailWithCode(c, errorx.CodeForbidden, "用户没有任何角色权限")
			c.Abort()
			return
		}

		for _, roleCode := range roleCodes {
			if roleCode == systemEntity.SuperRoleCode {
				c.Next()
				return
			}
		}

		if authVerifier == nil {
			slog.Error("AuthVerifier 未初始化，拒绝访问")
			response.FailWithCode(c, errorx.CodeInternalError, "系统权限配置错误")
			c.Abort()
			return
		}

		hasPermission, apiFound, err := authVerifier.VerifyApiAuth(c.Request.Context(), method, path, roleCodes)
		if err != nil {
			slog.Error("验证API权限失败", "error", err, "path", path, "method", method)
			response.FailWithCode(c, errorx.CodeInternalError, "验证权限失败")
			c.Abort()
			return
		}

		if !apiFound {
			response.FailWithCode(c, errorx.CodeForbidden, "该API未授权访问")
			c.Abort()
			return
		}

		if !hasPermission {
			response.FailWithCode(c, errorx.CodeForbidden, "没有权限访问此API")
			c.Abort()
			return
		}

		c.Next()
	}
}
