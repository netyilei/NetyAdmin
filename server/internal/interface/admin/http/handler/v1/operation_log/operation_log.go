package operation_log

import (
	"strconv"

	"github.com/gin-gonic/gin"

	logDto "NetyAdmin/internal/interface/admin/dto/log"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	logRepo "NetyAdmin/internal/repository/log"
	logService "NetyAdmin/internal/service/log"
)

type OperationLogHandler struct {
	svc logService.OperationService
}

func NewOperationLogHandler(svc logService.OperationService) *OperationLogHandler {
	return &OperationLogHandler{svc: svc}
}

// @Summary      获取操作日志列表
// @Description  分页获取操作日志，支持按管理员、操作、时间范围筛选
// @Tags         操作日志
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        adminId query int false "管理员ID"
// @Param        action query string false "操作类型"
// @Param        startDate query string false "开始日期"
// @Param        endDate query string false "结束日期"
// @Success      200 {object} response.Response "操作日志列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/operation-logs [get]
func (h *OperationLogHandler) List(c *gin.Context) {
	var req logDto.OperationQueryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	req.Normalize()

	query := &logRepo.OperationQuery{
		AdminID:   req.AdminID,
		Action:    req.Action,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Page:      req.Current,
		PageSize:  req.Size,
	}

	result, err := h.svc.List(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}

// @Summary      删除操作日志
// @Description  根据ID删除操作日志
// @Tags         操作日志
// @Accept       json
// @Produce      json
// @Param        id path int true "日志ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/operation-logs/{id} [delete]
func (h *OperationLogHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.svc.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      批量删除操作日志
// @Description  根据ID列表批量删除操作日志
// @Tags         操作日志
// @Accept       json
// @Produce      json
// @Param        req body object true "批量删除请求，包含ids字段"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/operation-logs/batch-delete [post]
func (h *OperationLogHandler) DeleteBatch(c *gin.Context) {
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
