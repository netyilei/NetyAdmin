package error_log

import (
	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	logService "NetyAdmin/internal/service/log"
)

type ErrorLogHandler struct {
	svc logService.ErrorService
}

func NewErrorLogHandler(svc logService.ErrorService) *ErrorLogHandler {
	return &ErrorLogHandler{svc: svc}
}

type ErrorLogQueryReq struct {
	Current  int    `form:"current"`
	Size     int    `form:"size"`
	Level    string `form:"level"`
	Resolved string `form:"resolved"`
}

func (r *ErrorLogQueryReq) Normalize() {
	if r.Current < 1 {
		r.Current = 1
	}
	if r.Size < 1 {
		r.Size = 10
	}
	if r.Size > 100 {
		r.Size = 100
	}
}

// @Summary      获取错误日志列表
// @Description  分页获取错误日志，支持按日志级别、是否已解决筛选
// @Tags         错误日志
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        level query string false "日志级别"
// @Param        resolved query string false "是否已解决(true/false)"
// @Success      200 {object} response.Response "错误日志列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/error-logs [get]
func (h *ErrorLogHandler) List(c *gin.Context) {
	var req ErrorLogQueryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	req.Normalize()

	var resolved *bool
	if req.Resolved == "true" {
		val := true
		resolved = &val
	} else if req.Resolved == "false" {
		val := false
		resolved = &val
	}

	logs, total, err := h.svc.List(c.Request.Context(), req.Level, resolved, req.Current, req.Size)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, logs)
}

// @Summary      标记错误日志已解决
// @Description  根据ID将错误日志标记为已解决
// @Tags         错误日志
// @Accept       json
// @Produce      json
// @Param        id path int true "日志ID"
// @Success      200 {object} response.Response "标记成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/error-logs/{id}/resolve [put]
func (h *ErrorLogHandler) Resolve(c *gin.Context) {
	var req struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	adminID := c.GetUint("adminID")
	if adminID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	if err := h.svc.Resolve(c.Request.Context(), req.ID, adminID); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      删除错误日志
// @Description  根据ID删除错误日志
// @Tags         错误日志
// @Accept       json
// @Produce      json
// @Param        id path int true "日志ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/error-logs/{id} [delete]
func (h *ErrorLogHandler) Delete(c *gin.Context) {
	var req struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), req.ID); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      批量删除错误日志
// @Description  根据ID列表批量删除错误日志
// @Tags         错误日志
// @Accept       json
// @Produce      json
// @Param        req body object true "批量删除请求，包含ids字段"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/error-logs/batch-delete [post]
func (h *ErrorLogHandler) DeleteBatch(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if len(req.IDs) == 0 {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.DeleteBatch(c.Request.Context(), req.IDs); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
