// subscriber.go：登录审计事件订阅（配套）——identity 发布
// domain.identity.login.audit（topic 常量唯一源=identity/api/events，公开 api 包
// 允许跨模块 import——AGENTS §2「模块间只走 api/Port 或事件」），audit 模块
// （SecurityRecord 存储归属）订阅后转 Record 经既有 Sink 链落
// operation_logs（type=security）。语义：Bus Broadcast at-most-once——审计丢失
// 窗口与 Bus 可靠性对齐（不放大承诺；handler 失败计数不阻断）。
package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/netyilei/netyadmin/internal/kernel/bus"

	"github.com/netyilei/netyadmin/internal/modules/audit/application"
	"github.com/netyilei/netyadmin/internal/modules/audit/domain"
	idapievents "github.com/netyilei/netyadmin/internal/modules/identity/api/events"
)

// loginAuditEvent 是 identity 登录审计事件的 payload 契约镜像（字段名跨模块
// 对齐 identity 发布侧 JSON；跨模块只依赖字段名，不 import identity 实现）。
type loginAuditEvent struct {
	Action   string `json:"action"`
	ActorID  string `json:"actor_id"`
	Username string `json:"username"`
	Result   string `json:"result"`
	BizCode  string `json:"biz_code"`
}

// SubscribeLoginAudit 订阅登录审计事件（module.go 装配调用）。
func SubscribeLoginAudit(b *bus.Bus, svc *application.Service) error {
	if b == nil {
		return fmt.Errorf("audit: nil bus")
	}
	_, err := b.Subscribe(idapievents.TopicLoginAudit, func(ctx context.Context, msg bus.Message) error {
		var evt loginAuditEvent
		if err := json.Unmarshal(msg.Payload, &evt); err != nil {
			return fmt.Errorf("audit: decode login audit: %w", err)
		}
		rec := domain.Record{
			Type:      domain.TypeSecurity,
			ActorType: domain.ActorAdmin,
			ActorID:   evt.ActorID,
			Action:    evt.Action,
			Resource:  "/admin/v1/auth/login",
			Result:    evt.Result,
			BizCode:   evt.BizCode,
			Detail:    map[string]any{"username": evt.Username},
		}
		return svc.Write(ctx, rec)
	})
	if err != nil {
		return fmt.Errorf("audit: subscribe login audit: %w", err)
	}
	return nil
}
