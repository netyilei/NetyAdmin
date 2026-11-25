// Package protocol 是 WS 协议常量唯一源（ws-protocol.md §4/§5 逐字
// 镜像——帧类型/关闭码/应用段码；禁止手写字符串）。config 面默认值在 realtime
// 模块 Config（ws-protocol §6「所有限制必须通过配置」）。
package protocol

// 帧类型（ws-protocol §4；JSON 信封 {type, channel?, payload, ...}）。
const (
	// 客户端 → 服务端
	TypeSubscribe   = "subscribe"
	TypeUnsubscribe = "unsubscribe"
	TypeMessage     = "message"
	TypePing        = "ping"
	TypeAck         = "ack"
	TypeResume      = "resume"

	// 服务端 → 客户端
	TypeHello        = "hello"
	TypeSubscribed   = "subscribed"
	TypeUnsubscribed = "unsubscribed"
	TypePong         = "pong"
	TypeError        = "error"
	TypeClose        = "close"
)

// 关闭码（ws-protocol §5 逐字镜像；RFC 6455 + realtime 应用段）。
const (
	CloseNormal          = 1000 // Normal Closure（客户端主动）
	CloseGoingAway       = 1001 // 服务端优雅关闭/滚动升级
	ClosePolicyViolation = 1008 // 鉴权失败/频道越权
	CloseTryAgainLater   = 1013 // 过载/背压/限流（慢消费者断开=V 协议 §6）
	CloseAuthExpired     = 4001 // ticket/JWT 过期
	CloseChannelDenied   = 4002 // 频道 ACL 拒绝
	CloseRateLimited     = 4003 // 消息速率超限
	ClosePayloadTooLarge = 4004 // 单帧超限
	CloseDuplicateConn   = 4005 // 同身份连接数超限
	CloseProtocolErr     = 4006 // 非法帧/未知类型
	CloseSessionRevoked  = 4007 // 令牌吊销/账号禁用
	CloseServerErr       = 4008 // 服务端内部错误
)
