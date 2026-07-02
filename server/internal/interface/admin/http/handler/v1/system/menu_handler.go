package system

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
)

// @Summary      获取菜单列表
// @Description  分页查询菜单列表，支持按名称、状态、父菜单ID筛选
// @Tags         菜单管理
// @Accept       json
// @Produce      json
// @Param        name query string false "菜单名称"
// @Param        status query string false "状态"
// @Param        parentId query int false "父菜单ID"
// @Param        current query int false "当前页"
// @Param        size query int false "每页数量"
// @Success      200 {object} response.Response "菜单分页列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getMenuList [get]
func (h *SystemHandler) GetAdminMenuList(c *gin.Context) {
	var req systemDto.MenuQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	menus, total, err := h.menuService.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, menus)
}

// @Summary      获取菜单树
// @Description  获取完整的菜单树形结构
// @Tags         菜单管理
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "菜单树"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getMenuTree [get]
func (h *SystemHandler) GetAdminMenuTree(c *gin.Context) {
	tree, err := h.menuService.GetTree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, tree)
}

// @Summary      获取菜单按钮树
// @Description  获取菜单与按钮关联的树形结构
// @Tags         菜单管理
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "菜单按钮树"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getButtonTree [get]
func (h *SystemHandler) GetAdminButtonTree(c *gin.Context) {
	tree, err := h.menuService.GetMenuButtonTree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, tree)
}

// @Summary      获取菜单API树
// @Description  获取菜单与API关联的树形结构
// @Tags         菜单管理
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "菜单API树"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getApiTree [get]
func (h *SystemHandler) GetAdminApiTree(c *gin.Context) {
	tree, err := h.menuService.GetMenuApiTree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, tree)
}

// @Summary      创建菜单
// @Description  创建新的菜单，可设置路由、组件、图标、按钮等信息
// @Tags         菜单管理
// @Accept       json
// @Produce      json
// @Param        req body system.CreateMenuReq true "菜单创建参数"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/addMenu [post]
func (h *SystemHandler) AddAdminMenu(c *gin.Context) {
	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	var req systemDto.CreateMenuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	id, err := h.menuService.Create(c.Request.Context(), &req, operatorID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "菜单创建成功", gin.H{"id": id})
}

// @Summary      更新菜单
// @Description  更新菜单信息，可调整路由、组件、图标、按钮等
// @Tags         菜单管理
// @Accept       json
// @Produce      json
// @Param        req body system.UpdateMenuReq true "菜单更新参数"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/updateMenu [put]
func (h *SystemHandler) UpdateAdminMenu(c *gin.Context) {
	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	var req systemDto.UpdateMenuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.menuService.Update(c.Request.Context(), &req, operatorID); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "菜单更新成功", nil)
}

// @Summary      删除菜单
// @Description  根据菜单ID删除单个菜单
// @Tags         菜单管理
// @Accept       json
// @Produce      json
// @Param        id query int true "菜单ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/deleteMenu [delete]
func (h *SystemHandler) DeleteAdminMenu(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.menuService.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "菜单删除成功", nil)
}

// @Summary      获取菜单详情
// @Description  根据菜单ID获取菜单详细信息
// @Tags         菜单管理
// @Accept       json
// @Produce      json
// @Param        id path int true "菜单ID"
// @Success      200 {object} response.Response "菜单详情"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getMenu/{id} [get]
func (h *SystemHandler) GetAdminMenuByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的菜单ID")
		return
	}

	menu, err := h.menuService.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, menu)
}

// @Summary      获取全部页面
// @Description  获取所有可作为菜单的页面列表
// @Tags         菜单管理
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "页面列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getAllPages [get]
func (h *SystemHandler) GetAllPages(c *gin.Context) {
	pages, err := h.menuService.GetAllPages(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, pages)
}

// @Summary      批量删除菜单
// @Description  根据菜单ID列表批量删除菜单
// @Tags         菜单管理
// @Accept       json
// @Produce      json
// @Param        menuIds query []uint true "菜单ID列表"
// @Success      200 {object} response.Response "批量删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/deleteMenus [delete]
func (h *SystemHandler) DeleteAdminMenus(c *gin.Context) {
	var req struct {
		MenuIds []uint `form:"menuIds" json:"menuIds" binding:"required"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		if err = c.ShouldBindJSON(&req); err != nil {
			response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
			return
		}
	}

	var failedIDs []uint
	for _, id := range req.MenuIds {
		if err := h.menuService.Delete(c.Request.Context(), id); err != nil {
			failedIDs = append(failedIDs, id)
		}
	}
	if len(failedIDs) > 0 {
		response.FailWithCode(c, errorx.CodeInternalError, fmt.Sprintf("部分菜单删除失败，失败ID: %v", failedIDs))
		return
	}

	response.SuccessWithMsg(c, "菜单批量删除成功", nil)
}
