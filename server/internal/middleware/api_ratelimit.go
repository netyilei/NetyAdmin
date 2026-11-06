package middleware

import (
	"fmt"
	"hash/fnv"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/ratelimit"
	"NetyAdmin/internal/pkg/response"
)

// 通用业务 API 限流中间件（api_ratelimit）。
//
// 与 login_ratelimit（IP 维度、仅登录端点）和开放平台 AppKey 限流互补，
// 覆盖 admin/client 常规业务路由组。配置见 [api_ratelimit].rules：
// 规则为空时 wire 不注册本中间件；Redis 故障 fail-open（SlidingWindow 内置）。
// 维度：ip / user_id / claim（取不到时回退 ip，见 APIRateLimitRule 注释）。

// NewAPIRateLimit 构造通用限流中间件。limiter 为 nil 或 rules 为空时返回 nil（调用方跳过注册）。
func NewAPIRateLimit(limiter *ratelimit.SlidingWindow, rules []config.APIRateLimitRule) gin.HandlerFunc {
	if limiter == nil || len(rules) == 0 {
		return nil
	}
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		for _, rule := range rules {
			if !rulePathsMatch(rule.Paths, path) {
				continue
			}
			// 规则作用域 + 维度键：同维度不同规则（window/max 相同的不同 paths）也隔离计数。
			// SlidingWindow 内部还会拼入 window/max，双重保证不同参数的规则不共享 ZSET。
			if limiter.Allow(c.Request.Context(), ruleScope(rule)+":"+ruleDimensionKey(c, rule), rule.Window.Duration(), rule.Max) {
				continue
			}
			slog.Warn("api_ratelimit: request rejected",
				"rule", rule.Name, "path", path, "ip", c.ClientIP())
			response.FailWithCode(c, errorx.CodeRateLimited, "请求过于频繁，请稍后重试")
			c.Abort()
			return
		}
		c.Next()
	}
}

// ruleScope 为每条规则生成稳定且唯一的计数作用域。
//
// 用规则语义字段的指纹而不是配置序号：配置顺序调整或增删规则时，已有计数
// 不会静默转移给另一条规则（序号会）。名称保留在 key 中便于排查；
// 完全相同（含名称）的两条规则指纹相同、共享计数，语义等价无影响。
func ruleScope(rule config.APIRateLimitRule) string {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%q|%q|%q|%q|%s|%d",
		rule.Name, rule.Dimension, rule.Claim, rule.Paths,
		rule.Window.Duration(), rule.Max)
	return rule.Name + ":" + strconv.FormatUint(h.Sum64(), 16)
}

// rulePathsMatch 前缀匹配；空 Paths 匹配全部（规则作用于所挂路由组）。
func rulePathsMatch(paths []string, reqPath string) bool {
	if len(paths) == 0 {
		return true
	}
	for _, p := range paths {
		if strings.HasPrefix(reqPath, p) {
			return true
		}
	}
	return false
}

// ruleDimensionKey 提取限流维度键；user_id/claim 在未认证阶段或字段缺失时回退 ip。
func ruleDimensionKey(c *gin.Context, rule config.APIRateLimitRule) string {
	switch rule.Dimension {
	case "user_id":
		// admin 端 adminID 为 uint（auth.go:106），client 端 userID 为 string（auth.go:169）
		if id := c.GetUint("adminID"); id != 0 {
			return "uid:" + strconv.FormatUint(uint64(id), 10)
		}
		if id := c.GetString("userID"); id != "" {
			return "uid:" + id
		}
	case "claim":
		if rule.Claim != "" {
			if v := c.GetString(rule.Claim); v != "" {
				return "claim:" + rule.Claim + ":" + v
			}
		}
		slog.Debug("api_ratelimit: claim dimension miss, fallback to ip", "rule", rule.Name, "claim", rule.Claim)
	}
	// ip 维度与回退路径共用（c.ClientIP 受 SetTrustedProxies([]) 保护，不可伪造）
	return "ip:" + c.ClientIP()
}
