// access_middleware.go：管理端/客户端面访问控制中间件（——admin/client
// 作为开放平台应用接入，接受应用级 IP 控制（组集判定）与访问频率限制）。
// 挂载惯例：adminapi/clientapi 路由组（authn 之前——IP 层最先判定）。
// 默认零强制：应用组集空=不做 IP 判定（零配置不拒）；rate_limit_enabled=false
// 时跳过频率判定——部署者在「IP 访问控制」绑组、在「应用平台」开限流后生效。
//
// 应用行数据源=gw 装载型快照（销项：原 30s 单机内存缓存与 edge
// 快照/DB 构成三源，停用生效延迟 30s 且多节点不一致；快照化后写路径 commit
// 即 Reload+广播，本面与 edge 同源同失效链）。
package application

import (
	"context"
	"net"
	"net/netip"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/gw/domain"
)

// EdgePassedHeader/EdgePassedValue 是网关自环转发的内部标记（P1-A：边缘已完成
// 应用级判定——内层 accessCtl 不得二次判定 IP 组集/重复扣应用 quota）。
// 防伪：跳过要求传输层对端为环回（见 edgeForwardedBySelf）——外部请求源地址
// 非环回，伪造同名头不生效；transport 默认不信任 X-Forwarded-For，环回对端仅
// 本机进程可达=部署者信任域（与本机可直连 DB 同级），不做额外 HMAC。
const (
	EdgePassedHeader = "X-NA-Edge-Passed"
	EdgePassedValue  = "1"
)

// edgeForwardedBySelf 判定请求是否为本进程网关自环反代的内层请求。
// 条件=传输层对端环回 && 标记头值=EdgePassedValue（两者缺一不可）。
func edgeForwardedBySelf(c *gin.Context) bool {
	if c.GetHeader(EdgePassedHeader) != EdgePassedValue {
		return false
	}
	host := c.Request.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.IsLoopback()
}

// AccessControlDeps 是面级访问控制中间件依赖。
type AccessControlDeps struct {
	// AppByKey 按 appKey 取应用行（未命中/禁用 → 拒——面未自举/停用即封闭）。
	// 生产=应用快照读（Service.AppByKey）；无缓存层——失效由写路径广播承担。
	AppByKey func(ctx context.Context, appKey string) (*domain.App, error)
	Engine   *IPEngine
	// Quota 应用级访问频率滑窗（app.RateLimitEnabled=true 时判定；allowed=false
	// reason 给出超限/不可用区分——中间件映射 4293）。
	Quota func(ctx context.Context, app *domain.App, ip string) (allowed bool, reason string)
}

// AccessControlMiddleware 构造面级访问控制中间件（appKey=面绑定应用行）。
func AccessControlMiddleware(deps AccessControlDeps, appKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 网关自环内层跳过（P1-A）：边缘管线已按调用方应用完成 IP/配额判定，
		// 内层面级判定只对直连流量成立（127.0.0.1 非真实 C 端 IP、quota 双扣）。
		// 防伪造前提见 EdgePassedHeader 注释（环回对端=部署者信任域）。
		if edgeForwardedBySelf(c) {
			c.Next()
			return
		}
		app, err := deps.AppByKey(c.Request.Context(), appKey)
		if err != nil {
			transport.Fail(c, errorx.New(errorx.CodeUnavailable))
			c.Abort()
			return
		}
		if app.Status != domain.StatusEnabled {
			transport.Fail(c, errorx.New(errorx.CodeForbidden))
			c.Abort()
			return
		}
		ip, err := netip.ParseAddr(c.ClientIP())
		if err != nil {
			transport.Fail(c, errorx.New(errorx.CodeForbidden))
			c.Abort()
			return
		}
		// IP 组集判定（应用启用组集——严格合成 deny 优先；组集空=不限）
		d := deps.Engine.MatchGroups(ip, app.IPGroupIDs)
		if !d.Allowed {
			transport.Fail(c, errorx.New(errorx.CodeForbidden))
			c.Abort()
			return
		}
		// 应用级访问频率（quota 滑窗——rateLimitEnabled 才判定）
		if app.RateLimitEnabled && deps.Quota != nil {
			allowed, reason := deps.Quota(c.Request.Context(), app, c.ClientIP())
			if !allowed {
				code := errorx.CodeTooManyRequests
				if reason == "unavailable" {
					code = errorx.CodeUnavailable
				}
				transport.Fail(c, errorx.New(code))
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
