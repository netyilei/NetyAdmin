package operation_log

import (
	"fmt"

	"github.com/gin-gonic/gin"

	logDto "netyadmin/internal/interface/admin/dto/log"
	"netyadmin/internal/pkg/errorx"
	"netyadmin/internal/pkg/response"
	logService "netyadmin/internal/service/log"
)

type OperationLogHandler struct {
	svc logService.OperationService
}

func NewOperationLogHandler(svc logService.OperationService) *OperationLogHandler {
	return &OperationLogHandler{svc: svc}
}

func (h *OperationLogHandler) List(c *gin.Context) {
	var req logDto.OperationQueryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	result, err := h.svc.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, result)
}

func (h *OperationLogHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	var idUint uint
	if _, err := fmt.Sscanf(id, "%d", &idUint); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), idUint); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

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
