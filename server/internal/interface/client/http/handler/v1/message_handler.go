package v1

import (
	"strconv"

	"github.com/gin-gonic/gin"

	clientDto "NetyAdmin/internal/interface/client/dto/v1"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	msgServicePkg "NetyAdmin/internal/service/message"
)

type MessageHandler struct {
	msgSvc msgServicePkg.MessageService
}

func NewMessageHandler(msgSvc msgServicePkg.MessageService) *MessageHandler {
	return &MessageHandler{msgSvc: msgSvc}
}

// @Summary      站内消息列表
// @Description  分页获取当前用户的站内消息列表，支持已读过滤
// @Tags         客户端-消息
// @Accept       json
// @Produce      json
// @Param        page query int false "页码"
// @Param        pageSize query int false "每页数量"
// @Param        readFilter query string false "已读过滤"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/message/internal [get]
func (h *MessageHandler) ListInternalMsgs(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		response.FailWithCode(c, errorx.CodeUnauthorized)
		return
	}

	var req clientDto.InternalMsgListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	list, total, err := h.msgSvc.ListUserInternalMsgs(c.Request.Context(), userID, req.Page, req.PageSize, req.ReadFilter)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Page, req.PageSize, total, list)
}

// @Summary      站内消息详情
// @Description  根据消息ID获取站内消息详情
// @Tags         客户端-消息
// @Accept       json
// @Produce      json
// @Param        id path int true "消息ID"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/message/internal/{id} [get]
func (h *MessageHandler) GetInternalMsg(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		response.FailWithCode(c, errorx.CodeUnauthorized)
		return
	}

	msgIDStr := c.Param("id")
	msgID, err := strconv.ParseUint(msgIDStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	msg, err := h.msgSvc.GetInternalMsgDetail(c.Request.Context(), msgID, userID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, msg)
}

// @Summary      标记已读
// @Description  将指定站内消息标记为已读
// @Tags         客户端-消息
// @Accept       json
// @Produce      json
// @Param        req body clientDto.MarkReadReq true "标记已读请求"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/message/internal/read [put]
func (h *MessageHandler) MarkRead(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		response.FailWithCode(c, errorx.CodeUnauthorized)
		return
	}

	var req clientDto.MarkReadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.msgSvc.MarkInternalMsgRead(c.Request.Context(), req.MsgInternalID, userID); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      全部标记已读
// @Description  将当前用户所有站内消息标记为已读
// @Tags         客户端-消息
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/message/internal/read-all [put]
func (h *MessageHandler) MarkAllRead(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		response.FailWithCode(c, errorx.CodeUnauthorized)
		return
	}

	if err := h.msgSvc.MarkAllInternalMsgRead(c.Request.Context(), userID); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      未读消息数量
// @Description  获取当前用户的站内消息未读数量
// @Tags         客户端-消息
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/message/internal/unread-count [get]
func (h *MessageHandler) CountUnread(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		response.FailWithCode(c, errorx.CodeUnauthorized)
		return
	}

	count, err := h.msgSvc.CountUnreadInternalMsgs(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, gin.H{"unreadCount": count})
}
