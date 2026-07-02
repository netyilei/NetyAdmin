package system

import (
	"strconv"

	"github.com/gin-gonic/gin"

	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
)

// @Summary      获取按钮列表
// @Description  分页查询按钮列表，支持按标签、编码、菜单ID筛选
// @Tags         按钮管理
// @Accept       json
// @Produce      json
// @Param        label query string false "按钮标签"
// @Param        code query string false "按钮编码"
// @Param        menuId query int false "菜单ID"
// @Param        current query int false "当前页"
// @Param        size query int false "每页数量"
// @Success      200 {object} response.Response "按钮分页列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getButtonList [get]
func (h *SystemHandler) GetAdminButtonList(c *gin.Context) {
	var req systemDto.ButtonQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if req.Current <= 0 {
		req.Current = 1
	}
	if req.Size <= 0 {
		req.Size = 10
	}

	buttons, total, err := h.buttonService.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, buttons)
}

// @Summary      创建按钮
// @Description  创建新的按钮，绑定到指定菜单，设置名称与编码
// @Tags         按钮管理
// @Accept       json
// @Produce      json
// @Param        req body system.CreateButtonReq true "按钮创建参数"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/createButton [post]
func (h *SystemHandler) AddAdminButton(c *gin.Context) {
	var req systemDto.CreateButtonReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	id, err := h.buttonService.Create(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "按钮创建成功", gin.H{"id": id})
}

// @Summary      更新按钮
// @Description  更新按钮信息，包括所属菜单、名称与编码
// @Tags         按钮管理
// @Accept       json
// @Produce      json
// @Param        req body system.UpdateButtonReq true "按钮更新参数"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/updateButton [put]
func (h *SystemHandler) UpdateAdminButton(c *gin.Context) {
	var req systemDto.UpdateButtonReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.buttonService.Update(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "按钮更新成功", nil)
}

// @Summary      删除按钮
// @Description  根据按钮ID删除单个按钮
// @Tags         按钮管理
// @Accept       json
// @Produce      json
// @Param        id query int true "按钮ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/deleteButton [delete]
func (h *SystemHandler) DeleteAdminButton(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的按钮ID")
		return
	}

	if err := h.buttonService.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "按钮删除成功", nil)
}

// @Summary      获取全部按钮
// @Description  获取所有按钮列表，不分页
// @Tags         按钮管理
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "按钮列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getAllButton [get]
func (h *SystemHandler) GetAllAdminButton(c *gin.Context) {
	buttons, err := h.buttonService.GetAll(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, buttons)
}

// @Summary      获取按钮详情
// @Description  根据按钮ID获取按钮详细信息
// @Tags         按钮管理
// @Accept       json
// @Produce      json
// @Param        id path int true "按钮ID"
// @Success      200 {object} response.Response "按钮详情"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getButton/{id} [get]
func (h *SystemHandler) GetAdminButtonByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的按钮ID")
		return
	}

	button, err := h.buttonService.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, button)
}
