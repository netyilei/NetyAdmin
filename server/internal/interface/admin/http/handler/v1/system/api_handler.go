package system

import (
	"strconv"

	"github.com/gin-gonic/gin"

	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
)

// @Summary      获取API列表
// @Description  分页查询API列表，支持按名称、路径、请求方法、分组筛选
// @Tags         API管理
// @Accept       json
// @Produce      json
// @Param        name query string false "API名称"
// @Param        path query string false "API路径"
// @Param        method query string false "请求方法"
// @Param        group query string false "API分组"
// @Param        current query int false "当前页"
// @Param        size query int false "每页数量"
// @Success      200 {object} response.Response "API分页列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getApiList [get]
func (h *SystemHandler) GetAdminAPIList(c *gin.Context) {
	var req systemDto.APIQuery
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

	apis, total, err := h.apiService.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, apis)
}

// @Summary      创建API
// @Description  创建新的API接口记录，包含名称、路径、方法、分组等信息
// @Tags         API管理
// @Accept       json
// @Produce      json
// @Param        req body system.CreateAPIReq true "API创建参数"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/createApi [post]
func (h *SystemHandler) AddAdminAPI(c *gin.Context) {
	var req systemDto.CreateAPIReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	id, err := h.apiService.Create(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "API创建成功", gin.H{"id": id})
}

// @Summary      更新API
// @Description  更新API接口记录，包含名称、路径、方法、分组等信息
// @Tags         API管理
// @Accept       json
// @Produce      json
// @Param        req body system.UpdateAPIReq true "API更新参数"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/updateApi [put]
func (h *SystemHandler) UpdateAdminAPI(c *gin.Context) {
	var req systemDto.UpdateAPIReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "参数错误")
		return
	}

	if err := h.apiService.Update(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "API更新成功", nil)
}

// @Summary      删除API
// @Description  根据API ID删除单个API接口记录
// @Tags         API管理
// @Accept       json
// @Produce      json
// @Param        id path int true "API ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/deleteApi/{id} [delete]
func (h *SystemHandler) DeleteAdminAPI(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的API ID")
		return
	}

	if err := h.apiService.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithMsg(c, "API删除成功", nil)
}

// @Summary      获取API详情
// @Description  根据API ID获取API接口详细信息
// @Tags         API管理
// @Accept       json
// @Produce      json
// @Param        id path int true "API ID"
// @Success      200 {object} response.Response "API详情"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getApi/{id} [get]
func (h *SystemHandler) GetAdminAPIByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的API ID")
		return
	}

	api, err := h.apiService.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, api)
}

// @Summary      获取全部API
// @Description  获取所有API接口列表，不分页
// @Tags         API管理
// @Accept       json
// @Produce      json
// @Success      200 {object} response.Response "API列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/systemManage/getAllApi [get]
func (h *SystemHandler) GetAllAdminAPI(c *gin.Context) {
	apis, err := h.apiService.GetAll(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, apis)
}
