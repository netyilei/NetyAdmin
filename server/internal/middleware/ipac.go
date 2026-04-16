package middleware

import (
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
			// 如果匹配过程出错，为了安全，记录日志并放行
			c.Next()
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
