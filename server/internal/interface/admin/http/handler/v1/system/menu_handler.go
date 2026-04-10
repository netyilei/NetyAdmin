package system

import (
	"strconv"

	"github.com/gin-gonic/gin"

	systemDto "netyadmin/internal/interface/admin/dto/system"
	"netyadmin/internal/pkg/errorx"
	"netyadmin/internal/pkg/response"
)

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

func (h *SystemHandler) GetAdminMenuTree(c *gin.Context) {
	tree, err := h.menuService.GetTree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, tree)
}

func (h *SystemHandler) GetAdminButtonTree(c *gin.Context) {
	tree, err := h.menuService.GetMenuButtonTree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, tree)
}

func (h *SystemHandler) GetAdminApiTree(c *gin.Context) {
	tree, err := h.menuService.GetMenuApiTree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, tree)
}

func (h *SystemHandler) AddAdminMenu(c *gin.Context) {
	operatorID, _ := c.Get("userID")

	var req systemDto.CreateMenuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	id, err := h.menuService.Create(c.Request.Context(), &req, operatorID.(uint))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "菜单创建成功", gin.H{"id": id})
}

func (h *SystemHandler) UpdateAdminMenu(c *gin.Context) {
	operatorID, _ := c.Get("userID")

	var req systemDto.UpdateMenuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.menuService.Update(c.Request.Context(), &req, operatorID.(uint)); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "菜单更新成功", nil)
}

func (h *SystemHandler) DeleteAdminMenu(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		idStr = c.Param("id")
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的菜单ID")
		return
	}

	if err := h.menuService.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "菜单删除成功", nil)
}

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

func (h *SystemHandler) GetAllPages(c *gin.Context) {
	pages, err := h.menuService.GetAllPages(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, pages)
}

func (h *SystemHandler) DeleteAdminMenus(c *gin.Context) {
	var req struct {
		Ids []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	for _, id := range req.Ids {
		_ = h.menuService.Delete(c.Request.Context(), id)
	}

	response.SuccessWithMsg(c, "菜单批量删除成功", nil)
}
