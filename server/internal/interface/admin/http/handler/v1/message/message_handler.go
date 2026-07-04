package message

import (
	"strconv"

	"github.com/gin-gonic/gin"

	msgDto "NetyAdmin/internal/interface/admin/dto/message"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/pagination"
	"NetyAdmin/internal/pkg/response"
	msgSvc "NetyAdmin/internal/service/message"
)

type MessageHandler struct {
	svc msgSvc.MessageService
}

func NewMessageHandler(svc msgSvc.MessageService) *MessageHandler {
	return &MessageHandler{svc: svc}
}

// @Summary      获取消息模板列表
// @Description  分页获取消息模板列表，支持按渠道、编码、名称、状态筛选
// @Tags         消息管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        channel query string false "消息渠道"
// @Param        code query string false "模板编码"
// @Param        name query string false "模板名称"
// @Param        status query int false "状态(0:禁用 1:启用)"
// @Success      200 {object} response.Response "模板列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/message/templates [get]
func (h *MessageHandler) ListTemplates(c *gin.Context) {
	var req msgDto.MsgTemplateQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	req.Current, req.Size = pagination.NormalizePagination(req.Current, req.Size)

	// 收敛 Handler 跨层调用（spec B10）：service 接收 admin DTO，不再依赖 handler 构造 repo query
	list, total, err := h.svc.ListTemplates(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, list)
}

// @Summary      获取消息记录列表
// @Description  分页获取消息发送记录，支持按渠道、接收人、状态筛选
// @Tags         消息管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        channel query string false "消息渠道"
// @Param        receiver query string false "接收人"
// @Param        status query int false "发送状态"
// @Success      200 {object} response.Response "记录列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/message/records [get]
func (h *MessageHandler) ListRecords(c *gin.Context) {
	var req msgDto.MsgRecordQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	req.Current, req.Size = pagination.NormalizePagination(req.Current, req.Size)

	// 收敛 Handler 跨层调用（spec B10）：service 接收 admin DTO，不再依赖 handler 构造 repo query
	list, total, err := h.svc.ListRecords(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, list)
}

// @Summary      直接发送消息
// @Description  管理员直接发送消息到指定接收人
// @Tags         消息管理
// @Accept       json
// @Produce      json
// @Param        req body message.SendDirectReq true "发送消息请求"
// @Success      200 {object} response.Response "发送成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/message/send [post]
func (h *MessageHandler) SendDirect(c *gin.Context) {
	var req msgDto.SendDirectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.SendDirect(c.Request.Context(), req.Channel, req.Receiver, req.Title, req.Content); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      新增消息模板
// @Description  创建消息模板
// @Tags         消息管理
// @Accept       json
// @Produce      json
// @Param        req body message.CreateTemplateReq true "模板信息"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/message/templates [post]
func (h *MessageHandler) CreateTemplate(c *gin.Context) {
	var req msgDto.CreateTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.svc.CreateTemplate(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary      修改消息模板
// @Description  更新消息模板信息
// @Tags         消息管理
// @Accept       json
// @Produce      json
// @Param        id path int true "模板ID"
// @Param        req body message.UpdateTemplateReq true "模板信息"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/message/templates/{id} [put]
func (h *MessageHandler) UpdateTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的模板ID")
		return
	}

	var req msgDto.UpdateTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.svc.UpdateTemplate(c.Request.Context(), id, &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary      删除消息模板
// @Description  根据ID删除消息模板
// @Tags         消息管理
// @Accept       json
// @Produce      json
// @Param        id path int true "模板ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/message/templates/{id} [delete]
func (h *MessageHandler) DeleteTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.DeleteTemplate(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary      重发消息
// @Description  根据记录ID重发失败的消息
// @Tags         消息管理
// @Accept       json
// @Produce      json
// @Param        id path int true "记录ID"
// @Success      200 {object} response.Response "重发成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/message/records/{id}/retry [post]
func (h *MessageHandler) RetryRecord(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.RetryRecord(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
