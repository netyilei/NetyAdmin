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

func (h *ErrorLogHandler) Resolve(c *gin.Context) {
	var req struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	userID, _ := c.Get("userID")

	if err := h.svc.Resolve(c.Request.Context(), req.ID, userID.(uint)); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

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
