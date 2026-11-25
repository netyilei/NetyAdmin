// broadcast.go：跨节点广播/在线状态契约（ws-protocol §9）。契约在
// session（消费方），multi 拓扑实现=realtime RedisBroadcaster/RedisPresence；
// single 拓扑 nil=本地直投零开销（连接归属本节点，无跨节点语义）。
package session

import "context"

// Broadcaster 是跨节点频道广播面（fire-and-forget——§8 默认 at-most-once；
// 发布失败由调用方本地兜底+计数，不阻塞会话）。
type Broadcaster interface {
	// Publish 向广播面发布帧。本节点经订阅回环收取（统一投递路径防双投）。
	Publish(ctx context.Context, channel string, frame []byte) error
	// RefChannel 本地出现该频道订阅者（首次 → 订阅广播键）。
	RefChannel(channel string)
	// UnrefChannel 本地该频道订阅者归零（→ 退订广播键）。
	UnrefChannel(channel string)
}

// Presence 是跨节点在线状态写入面（§9 presence 键+心跳 TTL；查询面随首个
// 消费方——G1-15）。
type Presence interface {
	// Touch 注册/心跳刷新（EXPIRE 续期——idleTimeout 对齐）。
	Touch(ctx context.Context, surface, principalID, connID string) error
	// Remove 注销清理（SREM；键空则短 TTL 自清）。
	Remove(ctx context.Context, surface, principalID, connID string) error
}
