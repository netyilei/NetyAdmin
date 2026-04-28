package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/domain/entity"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	systemService "NetyAdmin/internal/service/system"
)

type AdminHandler struct {
	adminService systemService.AdminService
}

func NewAdminHandler(adminService systemService.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

func (h *AdminHandler) List(c *gin.Context) {
	var req systemDto.AdminQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if req.Current <= 0 {
		req.Current = 1
	}
	if req.Size <= 0 {
		req.Size = entity.DefaultPageSize
	}

	admins, total, err := h.adminService.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, admins)
}

func (h *AdminHandler) Create(c *gin.Context) {
	operatorID, _ := c.Get("adminID")

	var req systemDto.CreateAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	id, err := h.adminService.Create(c.Request.Context(), &req, operatorID.(uint))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "管理员创建成功", gin.H{"id": id})
}

func (h *AdminHandler) Update(c *gin.Context) {
	operatorID, _ := c.Get("adminID")

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的管理员ID")
		return
	}

	var req systemDto.UpdateAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}
	req.ID = uint(id)

	if err := h.adminService.Update(c.Request.Context(), &req, operatorID.(uint)); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "管理员更新成功", nil)
}

func (h *AdminHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的管理员ID")
		return
	}

	if err := h.adminService.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "管理员删除成功", nil)
}

func (h *AdminHandler) DeleteBatch(c *gin.Context) {
	var req struct {
		Ids []uint `form:"ids" json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.adminService.DeleteBatch(c.Request.Context(), req.Ids); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "管理员批量删除成功", nil)
}

func (h *AdminHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的管理员ID")
		return
	}

	admin, err := h.adminService.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, admin)
}
