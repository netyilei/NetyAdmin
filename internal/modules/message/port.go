// port.go：api.Sender / api.Templates 的模块侧实现（把 application.Service
// 适配为跨模块端口；出参只回中性投影，不暴露 domain 类型）。
package message

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/netyilei/netyadmin/internal/modules/message/api"
	"github.com/netyilei/netyadmin/internal/modules/message/application"
	"github.com/netyilei/netyadmin/internal/modules/message/domain"
)

// portError 把 domain 哨兵映射为 api 哨兵（端口边界单点）：下游只 import
// message/api，用 errors.Is(err, messageapi.Err*) 判定；detail 说明保留
// （domain 哨兵后的附加文本）；未识别错误原样透传。
func portError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrTemplateNotFound):
		return api.ErrTemplateNotFound
	case errors.Is(err, domain.ErrTemplateDisabled):
		return api.ErrTemplateDisabled
	case errors.Is(err, domain.ErrTemplateConflict):
		return api.ErrTemplateConflict
	case errors.Is(err, domain.ErrNoActiveDriver):
		return api.ErrNoActiveDriver
	case errors.Is(err, domain.ErrNotReady):
		return api.ErrNotReady
	case errors.Is(err, domain.ErrTemplateCodeInvalid):
		return fmt.Errorf("%w: %s", api.ErrTemplateCodeInvalid, detailOf(err, domain.ErrTemplateCodeInvalid))
	case errors.Is(err, domain.ErrTemplateSpecInvalid):
		return fmt.Errorf("%w: %s", api.ErrTemplateSpecInvalid, detailOf(err, domain.ErrTemplateSpecInvalid))
	}
	return err
}

// detailOf 取 domain 哨兵后的附加说明（"sentinel: detail" → "detail"；无=空）。
func detailOf(err, sentinel error) string {
	return strings.TrimPrefix(strings.TrimPrefix(err.Error(), sentinel.Error()), ": ")
}

// senderPort 实现 api.Sender（注册名 message.Sender）。
type senderPort struct{ svc *application.Service }

var _ api.Sender = senderPort{}

// Send 适配 Service.Send：返回发送记录 ID。
func (p senderPort) Send(ctx context.Context, templateID, target string, vars map[string]string) (string, error) {
	rec, err := p.svc.Send(ctx, templateID, target, vars)
	if err != nil {
		return "", portError(err)
	}
	return rec.ID, nil
}

// SendInbox 适配 Service.SendInbox：返回站内信 ID。
func (p senderPort) SendInbox(ctx context.Context, userID, title, body string) (string, error) {
	m, err := p.svc.SendInbox(ctx, userID, title, body)
	if err != nil {
		return "", portError(err)
	}
	return m.ID, nil
}

// templatesPort 实现 api.Templates（注册名 message.Templates）。
type templatesPort struct{ svc *application.Service }

var _ api.Templates = templatesPort{}

// Ensure 适配 Service.EnsureTemplate：spec → domain.Template → 中性投影。
// repo 未 Attach 时返回 api.ErrNotReady（调用方按可重试处理）。
func (p templatesPort) Ensure(ctx context.Context, spec api.TemplateSpec) (api.Template, error) {
	tpl, err := p.svc.EnsureTemplate(ctx, spec.Code, domain.Channel(spec.Channel), spec.Subject, spec.Body)
	if err != nil {
		return api.Template{}, portError(err)
	}
	return api.Template{
		ID: tpl.ID, Code: tpl.Name, Channel: string(tpl.Channel),
		Subject: tpl.Subject, Body: tpl.Body, Status: tpl.Status,
	}, nil
}
