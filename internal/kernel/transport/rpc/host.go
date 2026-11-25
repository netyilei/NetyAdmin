// host.go：RPC 宿主增强——接收方服务白名单（「接收方端口必须
// 校验调用方服务白名单」； TR-02 只交付 scope 校验，白名单触发点
// 兑现）+ 服务端 na_rpc_* 指标（calls/duration——RPC 行兑现）。
// 白名单语义 fail-closed：未配置 = 拒绝全部调用方（部署拓扑必须显式登记——
// 「不信任内网即可信」）；跨模块同进程绑定（local）不经此面。
package rpc

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
)

// AllowList 是调用方服务白名单（部署拓扑登记；nil/空=全拒 fail-closed）。
type AllowList map[string]bool

// NewAllowList 构造（ids 为合法调用方服务名清单；空清单 = 显式全拒）。
func NewAllowList(ids ...string) AllowList {
	m := make(AllowList, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// Allowed 判定调用方是否在白名单（fail-closed：空清单恒 false）。
func (a AllowList) Allowed(serviceID string) bool {
	return a != nil && a[serviceID]
}

// applyHostChecks 宿主级校验（JWT 后）：调用方白名单（接收方必须校验
// 调用方服务白名单——fail-closed）。act 二次 AuthZ 属消费（
// 保留面——无代行场景消费者）；module/port/method 形参为宿主校验扩展位
// （当前白名单是全局面——端口级差异化白名单随首个消费场景）。
func (s *Server) applyHostChecks(c *gin.Context, principal authn.Principal, module, port, method string) bool {
	if !s.allow.Allowed(principal.ID) {
		writeRPCError(c, http.StatusForbidden, errorx.New(errorx.CodePermissionDenied,
			"reason", "caller not allowed", "caller", principal.ID))
		return false
	}
	return true
}

// observeServer 服务端指标（handle 管线末段；code 组=2xx/4xx/5xx 低基数）。
func (s *Server) observeServer(module, port, method string, start time.Time, status int) {
	if s.metrics == nil {
		return
	}
	code := "2xx"
	switch {
	case status >= 500:
		code = "5xx"
	case status >= 400:
		code = "4xx"
	}
	s.metrics.RPCCallsTotal.WithLabelValues(module, port, method, "server", code).Inc()
	s.metrics.RPCDuration.WithLabelValues(module, port, method, "server").Observe(time.Since(start).Seconds())
}
