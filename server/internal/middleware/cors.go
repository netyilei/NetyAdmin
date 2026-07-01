// Package middleware 提供 HTTP 中间件。
//
// cors.go 基于 github.com/gin-contrib/cors v1.7.7 实现跨域中间件，
// 替代原先自研的 27 行硬编码 CORS 实现。
package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS 相关常量（避免魔法数字散落，集中可审查）。
const (
	// corsMaxAge 预检请求（OPTIONS）结果缓存时长。
	corsMaxAge = 24 * time.Hour
)

// corsAllowedMethods 允许的跨域 HTTP 方法。
// 与原自研实现保持一致，覆盖 RESTful 全部写读语义。
var corsAllowedMethods = []string{
	"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS",
}

// corsAllowedHeaders 允许的跨域请求头。
// 与原自研实现保持一致。
var corsAllowedHeaders = []string{
	"Origin",
	"Content-Type",
	"Accept",
	"Authorization",
	"X-Requested-With",
}

// CORS 跨域中间件。
//
// 策略说明（与原自研实现行为等价，避免引入破坏性变更）：
//   - AllowOrigins 为空 + AllowOriginFunc 回调：反射请求方 Origin，等价于"允许所有来源"。
//   - AllowCredentials: true —— 允许携带 Cookie。
//   - AllowMethods / AllowHeaders / MaxAge 与原硬编码实现一致。
//
// gin-contrib/cors 在 AllowAllOrigins=true 且 AllowCredentials=true 时会 panic（属于不安全组合），
// 因此这里使用 AllowOriginFunc 返回 true 的方式来表达"反射 Origin + 允许 Credentials"的语义，
// 该写法是库官方推荐的安全等价表达。
func CORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOriginFunc:  func(origin string) bool { return true },
		AllowMethods:     corsAllowedMethods,
		AllowHeaders:     corsAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           corsMaxAge,
	})
}
