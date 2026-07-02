package system

import (
	"strconv"

	"github.com/gin-gonic/gin"

	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
)

// @Summary      获取角色列表
// @Description  分页查询角色列表，支持按角色名称、编码、状态筛选
// @Tags         角色管理
// @Accept       json
// @Produce      json
// @Param        current query int false "当前页"
// @Param        size query int false "每页数量"
// @Param        name query string false "角色名称"
// @Param        code query string false "角色编码"
// @Param        status query string false "状态"
// @Success      200 {object} response.Response "角色分页列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getRoleList [get]
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

// @Summary      获取角色详情
// @Description  根据角色ID获取角色详细信息
// @Tags         角色管理
// @Accept       json
// @Produce      json
// @Param        id path int true "角色ID"
// @Success      200 {object} response.Response "角色详情"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getRole/{id} [get]
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

// @Summary      创建角色
// @Description  创建新的角色，可同时绑定菜单、按钮及API权限
// @Tags         角色管理
// @Accept       json
// @Produce      json
// @Param        req body system.CreateRoleReq true "角色创建参数"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/addRole [post]
func (h *SystemHandler) AddAdminRole(c *gin.Context) {
	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	var req systemDto.CreateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	id, err := h.roleService.Create(c.Request.Context(), &req, operatorID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "角色创建成功", gin.H{"id": id})
}

// @Summary      更新角色
// @Description  更新角色信息，可同时调整菜单、按钮及API权限
// @Tags         角色管理
// @Accept       json
// @Produce      json
// @Param        req body system.UpdateRoleReq true "角色更新参数"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/updateRole [put]
func (h *SystemHandler) UpdateAdminRole(c *gin.Context) {
	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	var req systemDto.UpdateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.roleService.Update(c.Request.Context(), &req, operatorID); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "角色更新成功", nil)
}

// @Summary      删除角色
// @Description  根据角色ID删除单个角色
// @Tags         角色管理
// @Accept       json
// @Produce      json
// @Param        id query int true "角色ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/deleteRole [delete]
func (h *SystemHandler) DeleteAdminRole(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.roleService.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "角色删除成功", nil)
}

// @Summary      批量删除角色
// @Description  根据角色ID列表批量删除角色
// @Tags         角色管理
// @Accept       json
// @Produce      json
// @Param        roleIds query []uint true "角色ID列表"
// @Success      200 {object} response.Response "批量删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/deleteRoles [delete]
func (h *SystemHandler) DeleteAdminRoles(c *gin.Context) {
	var req struct {
		Ids []uint `form:"roleIds" json:"roleIds" binding:"required"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		if err = c.ShouldBindJSON(&req); err != nil {
			response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
			return
		}
	}

	if err := h.roleService.DeleteBatch(c.Request.Context(), req.Ids); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "角色批量删除成功", nil)
}

// @Summary      获取全部角色
// @Description  获取所有角色列表，不分页
// @Tags         角色管理
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "角色列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getAllRoles [get]
func (h *SystemHandler) GetAllAdminRoles(c *gin.Context) {
	roles, err := h.roleService.GetAll(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, roles)
}

// @Summary      获取角色菜单权限
// @Description  根据角色ID获取角色已分配的菜单权限及首页路由
// @Tags         角色权限管理
// @Accept       json
// @Produce      json
// @Param        id path int true "角色ID"
// @Success      200 {object} response.Response "角色菜单权限"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/role/{id}/menus [get]
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

// @Summary      更新角色菜单权限
// @Description  根据角色ID更新角色的菜单权限列表及首页路由
// @Tags         角色权限管理
// @Accept       json
// @Produce      json
// @Param        id path int true "角色ID"
// @Param        req body object true "菜单权限信息(menuIds:菜单ID列表,homeRouteName:首页路由名)"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/role/{id}/menus [put]
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

// @Summary      获取角色按钮权限
// @Description  根据角色ID获取角色已分配的按钮权限
// @Tags         角色权限管理
// @Accept       json
// @Produce      json
// @Param        id path int true "角色ID"
// @Success      200 {object} response.Response "角色按钮权限"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/role/{id}/buttons [get]
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

// @Summary      更新角色按钮权限
// @Description  根据角色ID更新角色的按钮权限列表
// @Tags         角色权限管理
// @Accept       json
// @Produce      json
// @Param        id path int true "角色ID"
// @Param        buttonIds body []uint true "按钮ID列表"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/role/{id}/buttons [put]
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

// @Summary      获取角色API权限
// @Description  根据角色ID获取角色已分配的API权限
// @Tags         角色权限管理
// @Accept       json
// @Produce      json
// @Param        id path int true "角色ID"
// @Success      200 {object} response.Response "角色API权限"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/role/{id}/apis [get]
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

// @Summary      更新角色API权限
// @Description  根据角色ID更新角色的API权限列表
// @Tags         角色权限管理
// @Accept       json
// @Produce      json
// @Param        id path int true "角色ID"
// @Param        apiIds body []uint true "API ID列表"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/role/{id}/apis [put]
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
