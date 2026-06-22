package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	ipacSvcPkg "NetyAdmin/internal/service/ipac"
)

// IPACAuth IP 访问控制中间件
func IPACAuth(ipacSvc ipacSvcPkg.IPACService) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// 尝试从上下文获取 appID (可能由前面的中间件设置)
		var appID *string
		if val, exists := c.Get("appID"); exists {
			if id, ok := val.(string); ok {
				appID = &id
			}
		}

		allowed, err := ipacSvc.CheckIP(c.Request.Context(), clientIP, appID)
		if err != nil {
			// fail-closed：IPAC 校验异常时拒绝请求，避免安全策略被绕过
			slog.Error("IPAC 校验异常，拒绝访问", "error", err, "clientIP", clientIP)
			response.FailWithCode(c, errorx.CodeIPBlocked, "访问校验服务异常，请稍后再试")
			c.Abort()
			return
		}

		if !allowed {
			response.FailWithCode(c, errorx.CodeIPBlocked, "您的 IP 访问受限")
			c.Abort()
			return
		}

		c.Next()
	}
}
