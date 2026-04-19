package message

import (
	"strconv"

	"github.com/gin-gonic/gin"

	msgEntity "NetyAdmin/internal/domain/entity/message"
	msgDto "NetyAdmin/internal/interface/admin/dto/message"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	msgRepo "NetyAdmin/internal/repository/message"
	msgSvc "NetyAdmin/internal/service/message"
)

type MessageHandler struct {
	svc msgSvc.MessageService
}

func NewMessageHandler(svc msgSvc.MessageService) *MessageHandler {
	return &MessageHandler{svc: svc}
}

// ListTemplates 获取模板列表
func (h *MessageHandler) ListTemplates(c *gin.Context) {
	var req msgDto.MsgTemplateQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	query := &msgRepo.MsgRepoQuery{
		Page:     req.Current,
		PageSize: req.Size,
		Channel:  req.Channel,
		Code:     req.Code,
		Name:     req.Name,
		Status:   req.Status,
	}

	list, total, err := h.svc.ListTemplates(c.Request.Context(), query)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, list)
}

// ListRecords 获取记录列表
func (h *MessageHandler) ListRecords(c *gin.Context) {
	var req msgDto.MsgRecordQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	query := &msgRepo.MsgRepoQuery{
		Page:     req.Current,
		PageSize: req.Size,
		Channel:  req.Channel,
		Receiver: req.Receiver,
	}

	list, total, err := h.svc.ListRecords(c.Request.Context(), query)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, list)
}

// SendDirect 管理员直接发送消息
func (h *MessageHandler) SendDirect(c *gin.Context) {
	var req msgDto.SendDirectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.SendDirect(c.Request.Context(), req.Channel, req.Receiver, req.Title, req.Content); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, nil)
}

// CreateTemplate 新增消息模板
func (h *MessageHandler) CreateTemplate(c *gin.Context) {
	var req msgEntity.MsgTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.CreateTemplate(c.Request.Context(), &req); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}
	response.Success(c, nil)
}

// UpdateTemplate 修改消息模板
func (h *MessageHandler) UpdateTemplate(c *gin.Context) {
	var req msgEntity.MsgTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.UpdateTemplate(c.Request.Context(), &req); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}
	response.Success(c, nil)
}

// DeleteTemplate 删除消息模板
func (h *MessageHandler) DeleteTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.DeleteTemplate(c.Request.Context(), id); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}
	response.Success(c, nil)
}

// RetryRecord 重发失败的消息
func (h *MessageHandler) RetryRecord(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.RetryRecord(c.Request.Context(), id); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError, err.Error())
		return
	}
	response.Success(c, nil)
}
