package open_platform

import (
	"strconv"

	"github.com/gin-gonic/gin"

	openDto "NetyAdmin/internal/interface/admin/dto/open_platform"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	openRepo "NetyAdmin/internal/repository/open_platform"
	openSvc "NetyAdmin/internal/service/open_platform"
)

type OpenLogHandler struct {
	svc openSvc.OpenLogService
}

func NewOpenLogHandler(svc openSvc.OpenLogService) *OpenLogHandler {
	return &OpenLogHandler{svc: svc}
}

// List 获取调用日志列表
func (h *OpenLogHandler) List(c *gin.Context) {
	var req openDto.OpenLogQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	query := &openRepo.OpenLogRepoQuery{
		Page:       req.Current,
		PageSize:   req.Size,
		AppID:      req.AppID,
		AppKey:     req.AppKey,
		ApiPath:    req.ApiPath,
		StatusCode: req.StatusCode,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
	}

	list, total, err := h.svc.ListLogs(c.Request.Context(), query)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
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
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, log)
}
