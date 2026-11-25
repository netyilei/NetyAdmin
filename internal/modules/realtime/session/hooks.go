// hooks.go：连接生命周期回调契约（ws-protocol §10 连接级审计/风控挂点）。
// 消费方=审计/风控（cmd 组合根注入实现——直调 audit Service 同 auditmw 惯例；
// 模块间零 import）。Noop 默认零开销（机制面精神——无消费者不建实现）。
package session

import (
	"context"

	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
)

// Hooks 是连接生命周期回调（全部回调不得阻塞——实现方应异步落库）。
type Hooks interface {
	// OnConnect 握手完成并注册后调用（hello 帧前）。
	OnConnect(ctx context.Context, surface string, p authn.Principal, connID, ip string)
	// OnDisconnect 连接终结时调用（code=关闭码——ws-protocol §5；reason 面向
	// 日志不外泄敏感细节；channels=终结时仍订阅的频道快照）。
	OnDisconnect(ctx context.Context, surface string, p authn.Principal, connID, ip string,
		code int, reason string, channels []string)
}

// NoopHooks 是零开销空实现（无审计消费者装配时的默认）。
type NoopHooks struct{}

// OnConnect 实现 Hooks（空）。
func (NoopHooks) OnConnect(context.Context, string, authn.Principal, string, string) {}

// OnDisconnect 实现 Hooks（空）。
func (NoopHooks) OnDisconnect(context.Context, string, authn.Principal, string, string, int, string, []string) {
}
