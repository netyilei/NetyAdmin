// Package ws 实现 WS 传输原语：upgrade、超时豁免、连接生命周期骨架。
// 协议常量唯一来源 modules/realtime/api/protocol 与 api/channels（禁止裸频道/关闭码）；
// ticket 鉴权、Hub、ACL、背压在 realtime 平台服务落地。
// 实现：TR-03。规范：docs/rules/ws-protocol.md。
package ws
