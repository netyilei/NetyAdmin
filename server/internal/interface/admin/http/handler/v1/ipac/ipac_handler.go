package ipac

import (
	"strconv"

	"github.com/gin-gonic/gin"

	ipacDto "NetyAdmin/internal/interface/admin/dto/ipac"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/pagination"
	"NetyAdmin/internal/pkg/response"
	ipacSvc "NetyAdmin/internal/service/ipac"
)

type IPACHandler struct {
	svc ipacSvc.IPACService
}

func NewIPACHandler(svc ipacSvc.IPACService) *IPACHandler {
	return &IPACHandler{svc: svc}
}

// List 获取 IP 规则列表
// @Summary      获取IP访问规则列表
// @Description  分页获取IP访问控制规则列表，支持按应用、IP、类型、状态筛选
// @Tags         IP访问控制
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        appId query string false "应用ID"
// @Param        ipAddr query string false "IP地址"
// @Param        type query int false "类型(1黑名单/2白名单)"
// @Param        status query int false "状态(0/1)"
// @Success      200 {object} response.Response "IP访问规则列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open-platform/ip-access [get]
func (h *IPACHandler) List(c *gin.Context) {
	var req ipacDto.IPACQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}
	req.Current, req.Size = pagination.NormalizePagination(req.Current, req.Size)

	// 收敛 Handler 跨层调用（spec B10）：service 接收 admin DTO，不再依赖 handler 构造 repo query
	list, total, err := h.svc.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, list)
}

// Create 新增 IP 规则
// @Summary      新增IP访问规则
// @Description  新增一条IP访问控制规则
// @Tags         IP访问控制
// @Accept       json
// @Produce      json
// @Param        req body ipac.CreateIPACReq true "新增IP规则参数"
// @Success      200 {object} response.Response "新增成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open-platform/ip-access [post]
func (h *IPACHandler) Create(c *gin.Context) {
	var req ipacDto.CreateIPACReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	operatorID := c.GetUint("adminID")

	if err := h.svc.Create(c.Request.Context(), &req, operatorID); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// Update 修改 IP 规则
// @Summary      修改IP访问规则
// @Description  修改一条IP访问控制规则
// @Tags         IP访问控制
// @Accept       json
// @Produce      json
// @Param        req body ipac.UpdateIPACReq true "修改IP规则参数"
// @Success      200 {object} response.Response "修改成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open-platform/ip-access [put]
func (h *IPACHandler) Update(c *gin.Context) {
	var req ipacDto.UpdateIPACReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	operatorID := c.GetUint("adminID")

	if err := h.svc.Update(c.Request.Context(), &req, operatorID); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// Delete 删除单个 IP 规则
// @Summary      删除IP访问规则
// @Description  根据ID删除单个IP访问控制规则
// @Tags         IP访问控制
// @Accept       json
// @Produce      json
// @Param        id path int true "IP规则ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open-platform/ip-access/{id} [delete]
func (h *IPACHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// DeleteBatch 批量删除 IP 规则
// @Summary      批量删除IP访问规则
// @Description  根据ID数组批量删除IP访问控制规则
// @Tags         IP访问控制
// @Accept       json
// @Produce      json
// @Param        req body ipac.BatchDeleteIPACReq true "批量删除参数"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/open-platform/ip-access/batch [delete]
func (h *IPACHandler) DeleteBatch(c *gin.Context) {
	var req ipacDto.BatchDeleteIPACReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.svc.DeleteBatch(c.Request.Context(), req.IDs); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
