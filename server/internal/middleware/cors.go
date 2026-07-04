// Package middleware 提供 HTTP 中间件。
//
// cors.go 基于 github.com/gin-contrib/cors v1.7.7 实现跨域中间件。
//
// 安全策略：
//   - Origin 白名单精确匹配（来自 config.toml 的 [cors].allowed_origins 或环境变量
//     NETYADMIN_CORS_ALLOWED_ORIGINS，逗号分隔）。
//   - 空白名单 = 拒绝所有跨域（fail-closed），生产环境必须显式配置可信来源。
//   - AllowCredentials: true —— 仅对白名单内的 Origin 生效，允许携带 Cookie。
//   - 不再使用 AllowOriginFunc: func(origin string) bool { return true } 反射行为。
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
// allowedOrigins 为允许的来源白名单（精确匹配）。空列表时拒绝所有跨域请求。
//
// gin-contrib/cors 在 AllowAllOrigins=true 且 AllowCredentials=true 时会 panic（属于不安全组合），
// 因此这里使用 AllowOriginFunc 对请求 Origin 做白名单校验，仅对白名单内 Origin 返回 true。
// 该写法是库官方推荐的安全等价表达，且支持 AllowCredentials: true。
func CORS(allowedOrigins []string) gin.HandlerFunc {
	// 预构建 O(1) 查询表，避免每次请求线性扫描
	allowSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowSet[o] = struct{}{}
	}

	return cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			_, ok := allowSet[origin]
			return ok
		},
		AllowMethods:     corsAllowedMethods,
		AllowHeaders:     corsAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           corsMaxAge,
	})
}
