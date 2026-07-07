package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/pkg/auth"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	ipacSvcPkg "NetyAdmin/internal/service/ipac"
)

// IPACAuth IP 访问控制中间件
func IPACAuth(ipacSvc ipacSvcPkg.IPACService) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// 尝试从 AppContext 获取 appID（由前面的 OpenPlatformAuth 中间件设置）。
		// Round 7：原 c.Get("appID") 遗留 key 已迁移至 currentAppContext.ID。
		var appID *string
		if val, exists := c.Get("currentAppContext"); exists {
			if appCtx, ok := val.(*auth.AppContext); ok && appCtx != nil {
				id := appCtx.ID
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
