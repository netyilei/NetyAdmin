package open_platform

import (
	"strconv"

	"github.com/gin-gonic/gin"

	openDto "NetyAdmin/internal/interface/admin/dto/open_platform"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/pagination"
	"NetyAdmin/internal/pkg/response"
	openSvc "NetyAdmin/internal/service/open_platform"
)

type OpenLogHandler struct {
	svc openSvc.OpenLogService
}

func NewOpenLogHandler(svc openSvc.OpenLogService) *OpenLogHandler {
	return &OpenLogHandler{svc: svc}
}

// @Summary      获取开放平台调用日志列表
// @Description  分页获取开放平台调用日志，支持按应用、AppKey、API路径、状态码、时间范围筛选
// @Tags         开放平台日志
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        appId query string false "应用ID"
// @Param        appKey query string false "应用AppKey"
// @Param        apiPath query string false "API路径"
// @Param        statusCode query int false "HTTP状态码"
// @Param        startTime query string false "开始时间"
// @Param        endTime query string false "结束时间"
// @Success      200 {object} response.Response "日志列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/ops/open-platform-log [get]
func (h *OpenLogHandler) List(c *gin.Context) {
	var req openDto.OpenLogQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	req.Current, req.Size = pagination.NormalizePagination(req.Current, req.Size)

	// 收敛 Handler 跨层调用（spec B10）：service 接收 admin DTO，不再依赖 handler 构造 repo query
	list, total, err := h.svc.ListLogs(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, list)
}

// Get 获取日志详情
func (h *OpenLogHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	log, err := h.svc.GetLog(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, log)
}
