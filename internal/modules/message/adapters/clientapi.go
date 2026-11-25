// clientapi.go：message client 端点（站内信——authed 组挂载，principal
// 即 user ID；authn 由 users JWT 中间件承载——cmd 接线，模块零 import）。
package adapters

import (
	"github.com/gin-gonic/gin"

	// blank import 供 swag 解析 @Failure 注解引用（errorx.Envelope——rbac 同惯例）
	_ "github.com/netyilei/netyadmin/internal/kernel/errorx"

	"github.com/netyilei/netyadmin/internal/kernel/transport"
	msgapp "github.com/netyilei/netyadmin/internal/modules/message/application"
)

// RegisterInboxClientRoutes 注册站内信 client 端点（module.go 调用——
// authed 组由 users JWT 中间件承载）。
func RegisterInboxClientRoutes(authed gin.IRouter, svc *msgapp.Service) {
	ib := authed.Group("/messages/inbox")
	ib.GET("", listInboxHandler(svc))
	ib.GET("/unread-count", unreadCountHandler(svc))
	ib.POST("/read-all", markAllReadHandler(svc))
	ib.POST("/:id/read", markReadHandler(svc))
}

// @Summary 站内信列表
// @Tags messages-client
// @ID message.client.inbox.list
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Success 200 {object} errorx.Envelope{data=transport.Page}
// @Failure 401 {object} errorx.Envelope
// @Router /client/v1/messages/inbox [get]
func listInboxHandler(svc *msgapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		page, size := transport.BindPage(c)
		list, total, err := svc.ListInbox(c.Request.Context(), p.ID, transport.Offset(page, size), size)
		if err != nil {
			transport.FailOf(c, mapMessageError(err)) // 裸哨兵禁直传（D3——与 adminapi 同源）
			return
		}
		transport.OK(c, transport.NewPage(list, int64(total), page, size))
	}
}

// @Summary 站内信标记已读
// @Tags messages-client
// @ID message.client.inbox.read
// @Param id path string true "站内信 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /client/v1/messages/inbox/{id}/read [post]
func markReadHandler(svc *msgapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		if err := svc.MarkRead(c.Request.Context(), p.ID, c.Param("id")); err != nil {
			transport.FailOf(c, mapMessageError(err)) // 重复已读/越权探测=404 语义（D3）
			return
		}
		transport.OK(c, gin.H{})
	}
}

// @Summary 站内信全部已读
// @Tags messages-client
// @ID message.client.inbox.read-all
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Router /client/v1/messages/inbox/read-all [post]
func markAllReadHandler(svc *msgapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		n, err := svc.MarkAllRead(c.Request.Context(), p.ID)
		if err != nil {
			transport.FailOf(c, mapMessageError(err))
			return
		}
		transport.OK(c, gin.H{"marked": n})
	}
}

// @Summary 站内信未读计数
// @Tags messages-client
// @ID message.client.inbox.unread-count
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Router /client/v1/messages/inbox/unread-count [get]
func unreadCountHandler(svc *msgapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		n, err := svc.UnreadCount(c.Request.Context(), p.ID)
		if err != nil {
			transport.FailOf(c, mapMessageError(err))
			return
		}
		transport.OK(c, gin.H{"unread": n})
	}
}

// inboxSendRequest 是站内信管理端请求（admin 发送面）。
type inboxSendRequest struct {
	UserID string `json:"user_id" validate:"required"`
	Title  string `json:"title" validate:"required"`
	Body   string `json:"body" validate:"required"`
}

// @Summary 发送站内信（管理端）
// @Tags message
// @ID admin.message.inbox.send
// @Accept json
// @Param body body inboxSendRequest true "站内信"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/message/inbox [post]
func sendInboxHandler(svc *msgapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req inboxSendRequest
		if !transport.Bind(c, &req) {
			return
		}
		m, err := svc.SendInbox(c.Request.Context(), req.UserID, req.Title, req.Body)
		if err != nil {
			transport.FailOf(c, mapMessageError(err))
			return
		}
		transport.OK(c, m)
	}
}
