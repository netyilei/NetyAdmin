// Package middleware 提供 HTTP 中间件。
//
// login_ratelimit.go 实现 login 端点 IP 维度限流中间件（fix-fundamental-design-flaws Task 3）。
//
// 与全局中间件的关系：
//   - 本中间件不通过 engine.Use 全局注册，仅在 admin /auth/login + /auth/refreshToken
//     与 client /user/login + /user/refresh-token 路由上单独应用。
//   - 由 router/v1/auth.go (admin) 与 router/v1/user_router.go (client) 在路由注册时挂载。
//
// IP 来源：
//   - 使用 c.ClientIP()，该方法会根据 gin.SetTrustedProxies 配置决定是否信任
//     X-Forwarded-For / X-Real-IP 头。空 trusted_proxies（默认）时回退到 RemoteAddr，
//     防止攻击者伪造 IP 头绕过限流（与 IPAC 安全策略一致，参见 RULES.md §四）。
//
// 计数语义：
//   - Check 在 handler 执行前读取滑动窗口内当前计数，未达阈值放行。
//   - Record 在 handler 返回后追加一条记录（不论 handler 成功或失败），
//     用于统计所有登录尝试（含密码错误、用户不存在等），而非仅成功登录。
//   - 失败开放（fail-open）：Check 错误时放行，Record 错误时仅 Warn 不阻断。
package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	authPkg "NetyAdmin/internal/pkg/auth"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
)

// LoginRateLimit 返回登录限流中间件。
//
// limiter 为 nil 时中间件退化为透传（不挂 Redis 的开发场景兜底）。
// 调用方应在 wire.go 中根据 Redis 是否可用选择具体实现（NewLoginLimiter 内部已处理 nil）。
func LoginRateLimit(limiter authPkg.LoginLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// limiter 为 nil（理论上不会发生：NewLoginLimiter(nil,...) 返回 noopLoginLimiter）
		// 兜底直接放行，避免 nil panic 阻断登录关键路径。
		if limiter == nil {
			c.Next()
			return
		}

		ip := c.ClientIP()
		ctx := c.Request.Context()

		// Check：滑动窗口内尝试次数是否已达上限
		ok, err := limiter.Check(ctx, ip)
		if err != nil {
			// Check 错误（如 Redis 短暂故障）→ fail-open 放行。
			// 不调 Record：Redis 故障时 Record 大概率也会失败，且响应已由 handler 写入，
			// 不需要在响应后做额外 Redis 调用拖慢链路。
			c.Next()
			return
		}
		if !ok {
			// 达到限流阈值：返回 100006 CodeTooManyRequest 并终止中间件链。
			// 注：errorx 中常量名为 CodeTooManyRequest（单数），与 HTTP 429 TooManyRequests 同义。
			// c.Abort() 是关键：response.FailWithCode 仅调用 c.JSON + c.Error，不终止中间件链。
			// 若不调用 c.Abort()，Gin 的 Context.Next() 会继续执行下一个 handler（Login/RefreshToken），
			// 导致限流保护完全失效——攻击者可绕过限流进行暴力破解。
			// 与 IPACAuth（middleware/ipac.go）和 JWTAuth（pkg/auth/middleware.go）行为一致。
			response.FailWithCode(c, errorx.CodeTooManyRequest)
			c.Abort()
			return
		}

		// 放行：执行 handler
		c.Next()

		// 调用顺序：Check（只读判定，不写 ZSET）→ c.Next()（handler 执行）→ Record（ZADD 写入本次尝试）。
		// 该顺序不可调换，原因：
		//   1. 计数语义：Record 记录的是「被 Check 放行并实际进入 handler 的尝试」，
		//      不论 handler 返回成功 / 密码错误 / 用户不存在等任何结果都记一次。
		//      这是限流器期望的语义——攻击者用大量错误密码轮询正是要被统计的流量；
		//      若只记成功登录，暴力破解在窗口内会“隐形”，导致 Max 永远不会被触发。
		//   2. 限流请求不记账：被 Check 拒绝的请求会提前 return，不会执行到 Record，
		//      因此不消耗 ZSET 槽位。Check 已是窗口上限判定步骤，避免被限流的请求被二次记账
		//      导致窗口提前填满、误伤正常用户。
		//   3. 严禁将 Record 移到 c.Next() 之前：handler 若中途 return / panic，
		//      仍会留一条“未完成尝试”在 ZSET，污染计数（数量级小但语义不正确）。
		//   4. 严禁将 Record 移到 Check 之前：被限流的请求也会被 Record 写入 ZSET，
		//      窗口配额会被已经限流的请求二次消耗，正常用户会被提前限流。
		//
		// Record 失败仅 Warn 不影响响应（响应已由 handler 写入；fail-open 语义见 login_limiter.go）。
		if err := limiter.Record(ctx, ip); err != nil {
			slog.Warn("login_ratelimit: Record failed (fail-open, request not blocked)",
				"ip", ip, "error", err)
		}
	}
}
