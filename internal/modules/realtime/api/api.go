// Package api 是 realtime 模块的跨模块端口（WS 推送与连接面）。
// 类型定义在 kernel 单一源（transport/ws）与本模块 protocol 包，本包只做
// 模块级消费入口别名——下游 import 本包即可注入依赖，不必逐个引用内部包。
package api

import (
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/transport/ws"
	"github.com/netyilei/netyadmin/internal/modules/realtime/api/protocol"
)

// Hub 是 WS 连接注册与投递面（注册名 `realtime.Hub`）：
//
//	hub, err := module.ResolveAs[realtimeapi.Hub](registry.Services, "realtime.Hub")
//	// 向频道投递（连接生命周期由进程管理；下游只做广播/定向投递）
//
// 连接归属本节点；multi 拓扑跨节点投递经总线广播。
type Hub = ws.Hub

// Conn 是单条 WS 连接（Hub 投递面使用）。
type Conn = ws.Conn

// TicketStore 是一次性 ticket 存储（注册名 `realtime.TicketStore`；边缘层签发/消费）。
type TicketStore = ws.RedisTicketStore

// TicketGrant 是 ticket 绑定负载（签发面使用）。
type TicketGrant = ws.TicketGrant

// Principal 是 WS 会话身份（订阅 ACL 与连接 hook 面使用）。
type Principal = authn.Principal

// Envelope 是 WS 消息信封（频道广播/订阅帧；字段见 protocol 包）。
type Envelope = protocol.Envelope

// 帧类型常量（与 protocol 包同源——避免下游手写字符串）。
const (
	TypeMessage = protocol.TypeMessage
	TypePing    = protocol.TypePing
	TypePong    = protocol.TypePong
)
