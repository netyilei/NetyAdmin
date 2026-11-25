// adminapi.go：消息管理端点（——模板 CRUD/send/records/resend）。
package adapters

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"

	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/message/api/keys"
	msgapp "github.com/netyilei/netyadmin/internal/modules/message/application"
	msgdomain "github.com/netyilei/netyadmin/internal/modules/message/domain"
)

// Deps 是端点依赖（cmd 组合根注入）。
type Deps struct {
	Service    *msgapp.Service
	Authorizer authz.Authorizer
	OnDeny     func(*gin.Context, authn.Principal, string, string)
}

// RegisterAdminRoutes 注册消息管理端点（module.go 调用）。
func RegisterAdminRoutes(authed gin.IRouter, deps Deps) {
	authzMW := transport.AuthzMiddleware(deps.Authorizer, transport.PrincipalFrom, deps.OnDeny)
	secured := func(perm string, h gin.HandlerFunc) []gin.HandlerFunc {
		return transport.Secured(authzMW, perm, h)
	}

	tpl := authed.Group("/message/templates")
	tpl.GET("", secured(keys.PermTemplatesList, listTemplatesHandler(deps))...)
	tpl.POST("", secured(keys.PermTemplatesWrite, createTemplateHandler(deps))...)
	tpl.PUT("/:id", secured(keys.PermTemplatesWrite, updateTemplateHandler(deps))...)
	tpl.DELETE("/:id", secured(keys.PermTemplatesWrite, deleteTemplateHandler(deps))...)
	authed.POST("/message/send", secured(keys.PermSend, sendHandler(deps))...)
	authed.POST("/message/test-email", secured(keys.PermSend, testEmailHandler(deps))...)
	authed.POST("/message/inbox", secured(keys.PermInboxSend, sendInboxHandler(deps.Service))...)
	rec := authed.Group("/message/records")
	rec.GET("", secured(keys.PermRecordsList, listRecordsHandler(deps))...)
	rec.POST("/:id/resend", secured(keys.PermSend, resendHandler(deps))...)
}

// mapMessageError 哨兵 → 业务码（AGENTS §6 单一源——禁裸哨兵归一 10009）。
func mapMessageError(err error) error {
	switch {
	case errors.Is(err, msgdomain.ErrTemplateNotFound), errors.Is(err, msgdomain.ErrRecordNotFound):
		return errorx.New(errorx.CodeNotFound)
	case errors.Is(err, msgdomain.ErrTemplateConflict), errors.Is(err, msgdomain.ErrTemplateNameImmutable):
		return errorx.New(errorx.CodeConflict)
	case errors.Is(err, msgdomain.ErrChannelInvalid), errors.Is(err, msgdomain.ErrTemplateDisabled),
		errors.Is(err, msgdomain.ErrTemplateCodeInvalid), errors.Is(err, msgdomain.ErrTemplateSpecInvalid):
		return errorx.New(errorx.CodeValidationFailed)
	case errors.Is(err, msgdomain.ErrNoActiveDriver), errors.Is(err, msgdomain.ErrNotReady):
		return errorx.New(errorx.CodeUnavailable)
	}
	return err
}

// pageOf 取分页偏移（transport.BindPage 一致口径）。
func pageOf(c *gin.Context) (int, int, int) {
	page, size := transport.BindPage(c)
	return page, transport.Offset(page, size), size
}

// @Summary 消息模板列表
// @Tags message
// @ID admin.message.templates.list
// @Param name query string false "名称包含过滤"
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Success 200 {object} errorx.Envelope{data=transport.Page}
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/message/templates [get]
func listTemplatesHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, offset, size := pageOf(c)
		list, total, err := deps.Service.ListTemplates(c.Request.Context(), c.Query("name"), offset, size)
		if err != nil {
			transport.FailOf(c, mapMessageError(err))
			return
		}
		transport.OK(c, transport.NewPage(list, int64(total), page, size))
	}
}

// templateRequest 是模板写请求（status 必填枚举——update 缺省落 0=静默禁用
// 防呆，联测审计 D4；create 缺省由 handler 兜底启用）。
type templateRequest struct {
	Name    string `json:"name" validate:"required"`
	Channel string `json:"channel" validate:"required"`
	Subject string `json:"subject"`
	Body    string `json:"body" validate:"required"`
	Status  int16  `json:"status" validate:"required,oneof=1 2"`
}

// @Summary 创建消息模板
// @Tags message
// @ID admin.message.templates.create
// @Accept json
// @Param body body templateRequest true "模板"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /admin/v1/message/templates [post]
func createTemplateHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req templateRequest
		if !transport.Bind(c, &req) {
			return
		}
		t := &msgdomain.Template{Name: req.Name, Channel: msgdomain.Channel(req.Channel),
			Subject: req.Subject, Body: req.Body, Status: msgdomain.TemplateEnabled}
		if req.Status != 0 {
			t.Status = req.Status
		}
		if err := deps.Service.CreateTemplate(c.Request.Context(), t); err != nil {
			transport.FailOf(c, mapMessageError(err))
			return
		}
		transport.OK(c, t)
	}
}

// @Summary 更新消息模板
// @Tags message
// @ID admin.message.templates.update
// @Accept json
// @Param id path string true "模板 ID"
// @Param body body templateRequest true "模板"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/message/templates/{id} [put]
func updateTemplateHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req templateRequest
		if !transport.Bind(c, &req) {
			return
		}
		t := &msgdomain.Template{ID: c.Param("id"), Name: req.Name, Channel: msgdomain.Channel(req.Channel),
			Subject: req.Subject, Body: req.Body, Status: req.Status}
		if err := deps.Service.UpdateTemplate(c.Request.Context(), t); err != nil {
			transport.FailOf(c, mapMessageError(err))
			return
		}
		transport.OK(c, t)
	}
}

// @Summary 删除消息模板
// @Tags message
// @ID admin.message.templates.delete
// @Param id path string true "模板 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/message/templates/{id} [delete]
func deleteTemplateHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.Service.DeleteTemplate(c.Request.Context(), c.Param("id")); err != nil {
			transport.FailOf(c, mapMessageError(err))
			return
		}
		transport.OK(c, gin.H{})
	}
}

// sendRequest 是发送请求（模板+目标+变量）。
type sendRequest struct {
	TemplateID string            `json:"template_id" validate:"required"`
	Target     string            `json:"target" validate:"required"`
	Vars       map[string]string `json:"vars"`
}

// @Summary 按模板发送消息
// @Tags message
// @ID admin.message.send
// @Accept json
// @Param body body sendRequest true "发送请求"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/message/send [post]
func sendHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req sendRequest
		if !transport.Bind(c, &req) {
			return
		}
		rec, err := deps.Service.Send(c.Request.Context(), req.TemplateID, req.Target, req.Vars)
		if err != nil {
			transport.FailOf(c, mapMessageError(err))
			return
		}
		transport.OK(c, rec)
	}
}

// testEmailRequest 测试邮件请求。
type testEmailRequest struct {
	To string `json:"to" validate:"required,email"`
}

// @Summary 发送测试邮件（SMTP 配置联调）
// @Tags message
// @ID admin.message.test-email
// @Accept json
// @Param body body testEmailRequest true "收件人"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /admin/v1/message/test-email [post]
func testEmailHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req testEmailRequest
		if !transport.Bind(c, &req) {
			return
		}
		if err := deps.Service.SendTest(c.Request.Context(), req.To); err != nil {
			transport.FailOf(c, mapMessageError(err))
			return
		}
		transport.OK(c, gin.H{"sent": true})
	}
}

// @Summary 发送记录列表
// @Tags message
// @ID admin.message.records.list
// @Param channel query string false "通道过滤（email|sms）"
// @Param status query int false "状态过滤（1=pending 2=sent 3=failed）"
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Success 200 {object} errorx.Envelope{data=transport.Page}
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/message/records [get]
func listRecordsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, offset, size := pageOf(c)
		status, _ := strconv.Atoi(c.Query("status"))
		list, total, err := deps.Service.Records(c.Request.Context(), c.Query("channel"), int16(status), offset, size)
		if err != nil {
			transport.FailOf(c, mapMessageError(err))
			return
		}
		transport.OK(c, transport.NewPage(list, int64(total), page, size))
	}
}

// @Summary 重发消息
// @Tags message
// @ID admin.message.records.resend
// @Param id path string true "记录 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/message/records/{id}/resend [post]
func resendHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		rec, err := deps.Service.Resend(c.Request.Context(), c.Param("id"))
		if err != nil {
			transport.FailOf(c, mapMessageError(err))
			return
		}
		transport.OK(c, rec)
	}
}
