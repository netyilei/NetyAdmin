package system

import (
	"strconv"

	"github.com/gin-gonic/gin"

	systemDto "silentorder/internal/interface/admin/dto/system"
	"silentorder/internal/pkg/errorx"
	"silentorder/internal/pkg/response"
)

func (h *SystemHandler) GetAdminRoleList(c *gin.Context) {
	var req systemDto.RoleQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	roles, total, err := h.roleService.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, roles)
}

func (h *SystemHandler) GetAdminRoleByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的角色ID")
		return
	}

	role, err := h.roleService.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, role)
}

func (h *SystemHandler) AddAdminRole(c *gin.Context) {
	operatorID, _ := c.Get("userID")

	var req systemDto.CreateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	id, err := h.roleService.Create(c.Request.Context(), &req, operatorID.(uint))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "角色创建成功", gin.H{"id": id})
}

func (h *SystemHandler) UpdateAdminRole(c *gin.Context) {
	operatorID, _ := c.Get("userID")

	var req systemDto.UpdateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.roleService.Update(c.Request.Context(), &req, operatorID.(uint)); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "角色更新成功", nil)
}

func (h *SystemHandler) DeleteAdminRole(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		idStr = c.Param("id")
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的角色ID")
		return
	}

	if err := h.roleService.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "角色删除成功", nil)
}

func (h *SystemHandler) DeleteAdminRoles(c *gin.Context) {
	var req struct {
		Ids []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.roleService.DeleteBatch(c.Request.Context(), req.Ids); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "角色批量删除成功", nil)
}

func (h *SystemHandler) GetAllAdminRoles(c *gin.Context) {
	roles, err := h.roleService.GetAll(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, roles)
}

func (h *SystemHandler) GetAdminRoleMenus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的角色ID")
		return
	}

	data, err := h.roleService.GetRoleMenusWithHome(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, data)
}

func (h *SystemHandler) UpdateAdminRoleMenus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的角色ID")
		return
	}

	var req struct {
		MenuIds       []uint `json:"menuIds"`
		HomeRouteName string `json:"homeRouteName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.roleService.UpdateMenus(c.Request.Context(), uint(id), req.MenuIds, req.HomeRouteName); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "权限更新成功", nil)
}

func (h *SystemHandler) GetAdminRoleButtons(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的角色ID")
		return
	}

	data, err := h.roleService.GetRoleButtons(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, data)
}

func (h *SystemHandler) UpdateAdminRoleButtons(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的角色ID")
		return
	}

	var buttonIDs []uint
	if err := c.ShouldBindJSON(&buttonIDs); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.roleService.UpdateButtons(c.Request.Context(), uint(id), buttonIDs); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "角色按钮权限更新成功", nil)
}

func (h *SystemHandler) GetAdminRoleAPIs(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的角色ID")
		return
	}

	data, err := h.roleService.GetRoleAPIs(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, data)
}

func (h *SystemHandler) UpdateAdminRoleAPIs(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的角色ID")
		return
	}

	var apiIDs []uint
	if err := c.ShouldBindJSON(&apiIDs); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.roleService.UpdateAPIs(c.Request.Context(), uint(id), apiIDs); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "角色API权限更新成功", nil)
}
