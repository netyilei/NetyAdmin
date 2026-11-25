// authorizer.go：频道 ACL 契约（ws-protocol §2——订阅与消息两级校验；实现由
// 各 surface 注入：gateway=Scope 模式匹配 / admin=权限码 / client=私有+白名单）。
// 契约在 session（消费方），实现在各 surface 装配侧（cmd 组合根桥接）。
package session

import (
	"context"
	"errors"

	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
)

// Action 是频道操作（ACL 分派面）。
type Action string

const (
	ActionSubscribe Action = "subscribe" // 订阅校验（§2 第一级）
	ActionDeliver   Action = "deliver"   // 下行发送前再校验（§2 第二级——权限变更后仍可发=缺陷）
	ActionPublish   Action = "publish"   // 上行 message 到频道
)

// ErrChannelDenied 是频道 ACL 拒绝（→ error 帧 + close 4002）。
var ErrChannelDenied = errors.New("realtime: channel denied")

// ChannelAuthorizer 是频道鉴权契约（两级校验共用同一实现）。
type ChannelAuthorizer interface {
	// Authorize 判定 principal 对 channel 的 action 许可；拒绝返回非 nil 错误
	// （ErrChannelDenied 或携带原因的包装）。
	Authorize(ctx context.Context, p authn.Principal, channel string, action Action) error
}

// AuthorizerFunc 是函数适配器（cmd 组合根轻量实现用）。
type AuthorizerFunc func(ctx context.Context, p authn.Principal, channel string, action Action) error

// Authorize 实现 ChannelAuthorizer。
func (f AuthorizerFunc) Authorize(ctx context.Context, p authn.Principal, channel string, action Action) error {
	return f(ctx, p, channel, action)
}
