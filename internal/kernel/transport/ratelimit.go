// ratelimit.go：限流中间件（接入面按策略装配—— adminapi/clientapi
// 与 gateway 各自组合）。维度键提取在此层（gin 面），判定委托
// ratelimit.Engine（contracts 失败模式由统一决策）。
package transport

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/security/ratelimit"
)

// RateLimitMiddleware 按命名策略限流（keyFn 提取维度值；nil 默认 ClientIP→ip 维度）。
// 超限 → 10007（429 retryable，api-style §5 重试语义由码面表达）；不可用且
// fail-closed → 10010；fail-open 策略放行（决策）。onDeny 为审计回调
// （可 nil；security.md §2 限流触发必记——装配方接 SecurityRecord，kernel 不依赖
// 审计模块；系统错误 err!=nil 时 dimKey 原样透传供诊断）。
func RateLimitMiddleware(engine *ratelimit.Engine, policyName string,
	keyFn func(*gin.Context) string,
	onDeny func(c *gin.Context, policyName, dimKey string, denied bool)) gin.HandlerFunc {
	if keyFn == nil {
		keyFn = func(c *gin.Context) string { return c.ClientIP() } // ip 维度默认提取
	}
	return func(c *gin.Context) {
		if engine == nil {
			c.Next()
			return
		}
		dimKey := keyFn(c)
		if err := engine.Allow(c.Request.Context(), policyName, dimKey); err != nil {
			switch {
			case errors.Is(err, ratelimit.ErrDenied):
				if onDeny != nil {
					onDeny(c, policyName, dimKey, true)
				}
				Fail(c, errorx.New(errorx.CodeTooManyRequests))
			case errors.Is(err, ratelimit.ErrUnavailable):
				if onDeny != nil {
					onDeny(c, policyName, dimKey, false)
				}
				Fail(c, errorx.New(errorx.CodeUnavailable))
			default:
				FailOf(c, err) // 装配错误（未注册策略等）→ 10009 归一
			}
			return
		}
		c.Next()
	}
}
