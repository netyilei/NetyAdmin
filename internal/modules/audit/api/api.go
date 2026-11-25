// Package api 是 audit 模块的跨模块端口（下游模块 import 本包即可写操作审计；
// 禁止 import application/domain）。
package api

import "context"

// Recorder 是审计写入端口（注册名 `audit.Recorder`）：
//
//	rec, err := module.ResolveAs[auditapi.Recorder](registry.Services, "audit.Recorder")
//	err = rec.Write(ctx, auditapi.Record{
//	    Type: auditapi.TypeOperation, Action: "silentorder.verify", Result: "success",
//	    ActorType: "user", ActorID: uid, RequestID: rid,
//	})
//
// 写入异步入队（队列满同步降级，审计不丢）；ID/时间由服务端补齐。
type Recorder interface {
	Write(ctx context.Context, r Record) error
}

// 记录类型（与 audit 域内取值一致；下游按语义选择）。
const (
	TypeOperation int8 = 1 // 管理端/业务变更（写操作审计）
	TypeSecurity  int8 = 3 // 登录失败/越权/限流（安全记录）
)

// Record 是审计记录（产生方填结构化上下文；Detail 需自行脱敏 PII/密钥）。
type Record struct {
	Type      int8
	ActorType string // admin | user | service | app | system | anonymous
	ActorID   string
	Action    string // 如 silentorder.verify.email
	Resource  string // 路由/资源，如 /client/v1/orders
	Result    string // success | deny | error
	BizCode   string // 5 位业务码（error/deny 时必带）
	IP        string
	UserAgent string
	RequestID string // 必带（链路追溯）
	TraceID   string
	NodeID    string
	Detail    map[string]any
}
