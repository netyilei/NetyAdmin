// payload.go：request Hook 六段共享 payload 契约。
// transport 管线按段构造：
// Received 段独立快照（鉴权前——Principal 必缺席、无 Status）；Authenticated 起
// 与 handler 包裹段共享同一实例（AuthN 已注入 ctx——含 Principal；Status 在
// handler 相位填充）；同一段内各钩子共享同一实例——消费方只读，不得修改
// （异步段的读发生在同步段结束后，故只读）。
package transport

import (
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
)

// RequestContext 是六段 request hook 的共享 payload。
type RequestContext struct {
	Method    string
	Path      string
	RequestID string
	TraceID   string
	// Principal 是已认证身份（ctx 注入后携带）；nil = 匿名——匿名是合法状态，
	// 是否要求认证由路由元数据/接入面决定（AuthN 中间件接线后非 nil）。
	Principal *authn.Principal
	// Status 在 AfterHandler/ResponseSent/RequestError 段填充响应码。
	Status int
	// Error 仅 RequestError 段携带 panic 摘要；业务错误路径为 nil（状态码已表达）。
	Error error
}
