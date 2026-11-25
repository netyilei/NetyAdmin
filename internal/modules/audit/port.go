// port.go：api.Recorder 的模块侧实现（api DTO → domain 转换）。
package audit

import (
	"context"

	"github.com/netyilei/netyadmin/internal/modules/audit/api"
	"github.com/netyilei/netyadmin/internal/modules/audit/application"
	"github.com/netyilei/netyadmin/internal/modules/audit/domain"
)

// recorderPort 实现 api.Recorder（注册名 audit.Recorder）。
type recorderPort struct{ svc *application.Service }

var _ api.Recorder = recorderPort{}

// Write 适配 Service.Write（ID/CreatedAt 由服务端补齐）。
func (p recorderPort) Write(ctx context.Context, r api.Record) error {
	return p.svc.Write(ctx, domain.Record{
		Type:      domain.RecordType(r.Type),
		ActorType: r.ActorType,
		ActorID:   r.ActorID,
		Action:    r.Action,
		Resource:  r.Resource,
		Result:    r.Result,
		BizCode:   r.BizCode,
		IP:        r.IP,
		UserAgent: r.UserAgent,
		RequestID: r.RequestID,
		TraceID:   r.TraceID,
		NodeID:    r.NodeID,
		Detail:    r.Detail,
	})
}
