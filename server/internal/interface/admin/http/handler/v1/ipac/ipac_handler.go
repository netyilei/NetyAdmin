package ipac

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/domain/entity"
	ipacEntity "NetyAdmin/internal/domain/entity/ipac"
	ipacDto "NetyAdmin/internal/interface/admin/dto/ipac"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	ipacRepo "NetyAdmin/internal/repository/ipac"
	ipacSvc "NetyAdmin/internal/service/ipac"
)

type IPACHandler struct {
	svc ipacSvc.IPACService
}

func NewIPACHandler(svc ipacSvc.IPACService) *IPACHandler {
	return &IPACHandler{svc: svc}
}

// List 获取 IP 规则列表
func (h *IPACHandler) List(c *gin.Context) {
	var req ipacDto.IPACQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	query := &ipacRepo.IPACQuery{
		AppID:    req.AppID,
		IPAddr:   req.IPAddr,
		Type:     req.Type,
		Status:   req.Status,
		Page:     req.Current,
		PageSize: req.Size,
	}

	list, total, err := h.svc.List(c.Request.Context(), query)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, list)
}

// Create 新增 IP 规则
func (h *IPACHandler) Create(c *gin.Context) {
	var req ipacDto.CreateIPACReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	operatorID := c.GetUint("adminID")

	item := &ipacEntity.IPAccessControl{
		AppID:  req.AppID,
		IPAddr: req.IPAddr,
		Type:   req.Type,
		Reason: req.Reason,
		Status: req.Status,
		Operator: entity.Operator{
			CreatedBy: operatorID,
		},
	}

	if req.ExpiredAt != nil && *req.ExpiredAt != "" {
		t, err := time.Parse(time.DateTime, *req.ExpiredAt)
		if err != nil {
			response.FailWithCode(c, errorx.CodeInvalidParams, "过期时间格式错误")
			return
		}
		item.ExpiredAt = &t
	}

	if err := h.svc.Create(c.Request.Context(), item); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, nil)
}

// Update 修改 IP 规则
func (h *IPACHandler) Update(c *gin.Context) {
	var req ipacDto.UpdateIPACReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	operatorID := c.GetUint("adminID")

	item := &ipacEntity.IPAccessControl{
		Model: entity.Model{
			ID: req.ID,
		},
		Type:   req.Type,
		Reason: req.Reason,
		Status: req.Status,
		Operator: entity.Operator{
			UpdatedBy: operatorID,
		},
	}

	if req.ExpiredAt != nil && *req.ExpiredAt != "" {
		t, err := time.Parse(time.DateTime, *req.ExpiredAt)
		if err != nil {
			response.FailWithCode(c, errorx.CodeInvalidParams, "过期时间格式错误")
			return
		}
		item.ExpiredAt = &t
	}

	if err := h.svc.Update(c.Request.Context(), item); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, nil)
}

// Delete 删除单个 IP 规则
func (h *IPACHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), uint(id)); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, nil)
}

// DeleteBatch 批量删除 IP 规则
func (h *IPACHandler) DeleteBatch(c *gin.Context) {
	var req ipacDto.BatchDeleteIPACReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.DeleteBatch(c.Request.Context(), req.IDs); err != nil {
		response.FailWithCode(c, errorx.CodeInternalError)
		return
	}

	response.Success(c, nil)
}
