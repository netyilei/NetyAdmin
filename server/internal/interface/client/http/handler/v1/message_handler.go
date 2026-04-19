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
