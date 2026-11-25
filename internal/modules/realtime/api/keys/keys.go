// Package keys 是 realtime 的 key 唯一封装。
//
// 铁律（docs/rules/namespaces.md）：业务代码禁止裸 key 字符串，一律 import 本包。
// realtime 的 key 面：WS ticket（Redis GETDEL）；builder 随首个 key 需求。
package keys

// TicketNS 是 WS ticket 键段（完整键=na:{env}:realtime:ticket:{id}——
// namespaces.md §4；realtime 装配注入 RedisTicketStore）。
const TicketNS = "realtime:ticket"

// 广播/presence 键命名空间（ws-protocol §9——na:{env}:{本段}:{...}）。
const (
	// BroadcastNS 是频道广播 Pub/Sub 键段（完整键=na:{env}:realtime:ch:{channel}）。
	BroadcastNS = "realtime:ch"
	// PresenceNS 是在线状态键段（完整键=na:{env}:realtime:presence:{surface}:{id}）。
	PresenceNS = "realtime:presence"
	// SeqNS 是频道序号计数器键段（完整键=na:{env}:realtime:seq:{channel}；
	// coordination.IncrSequence 滑动 TTL 续期）。
	SeqNS = "realtime:seq"
)

// Seq 返回频道序号计数键（nextSeq→coordination.IncrSequence 唯一入口）。
func Seq(channel string) string { return SeqNS + ":" + channel }
