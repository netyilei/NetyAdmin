package content

import (
	"strconv"

	contentDto "NetyAdmin/internal/interface/admin/dto/content"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	contentService "NetyAdmin/internal/service/content/admin"

	"github.com/gin-gonic/gin"
)

type ContentCategoryHandler struct {
	service contentService.CategoryService
}

func NewContentCategoryHandler(service contentService.CategoryService) *ContentCategoryHandler {
	return &ContentCategoryHandler{service: service}
}

// @Summary      获取内容分类列表
// @Description  分页获取内容分类列表，支持按名称、编码、内容类型、状态筛选
// @Tags         内容分类管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        name query string false "分类名称"
// @Param        code query string false "分类编码"
// @Param        contentType query string false "内容类型(plaintext/richtext)"
// @Param        status query string false "状态(0/1)"
// @Success      200 {object} response.Response "分类列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/categories [get]
func (h *ContentCategoryHandler) List(c *gin.Context) {
	var req contentDto.ContentCategoryListQueryDTO
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if req.Current <= 0 {
		req.Current = 1
	}
	if req.Size <= 0 {
		req.Size = 10
	}

	categories, total, err := h.service.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, categories)
}

// @Summary      获取内容分类详情
// @Description  根据ID获取单个内容分类详情
// @Tags         内容分类管理
// @Accept       json
// @Produce      json
// @Param        id path int true "分类ID"
// @Success      200 {object} response.Response "分类详情"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/categories/{id} [get]
func (h *ContentCategoryHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	category, err := h.service.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, category)
}

// @Summary      创建内容分类
// @Description  新建一个内容分类
// @Tags         内容分类管理
// @Accept       json
// @Produce      json
// @Param        req body content.CreateContentCategoryDTO true "创建分类参数"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/categories [post]
func (h *ContentCategoryHandler) Create(c *gin.Context) {
	var req contentDto.CreateContentCategoryDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	category, err := h.service.Create(c.Request.Context(), operatorID, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, category)
}

// @Summary      更新内容分类
// @Description  根据ID更新内容分类信息
// @Tags         内容分类管理
// @Accept       json
// @Produce      json
// @Param        id path int true "分类ID"
// @Param        req body content.UpdateContentCategoryDTO true "更新分类参数"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/categories/{id} [put]
func (h *ContentCategoryHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	var req contentDto.UpdateContentCategoryDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	category, err := h.service.Update(c.Request.Context(), operatorID, uint(id), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, category)
}

// @Summary      删除内容分类
// @Description  根据ID删除内容分类
// @Tags         内容分类管理
// @Accept       json
// @Produce      json
// @Param        id path int true "分类ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/categories/{id} [delete]
func (h *ContentCategoryHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.service.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      获取内容分类树
// @Description  获取内容分类的树形结构，可选强制刷新缓存
// @Tags         内容分类管理
// @Accept       json
// @Produce      json
// @Param        refresh query string false "是否强制刷新缓存(true)"
// @Success      200 {object} response.Response "分类树"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/categories/tree [get]
func (h *ContentCategoryHandler) Tree(c *gin.Context) {
	forceRefresh := c.Query("refresh") == "true"
	tree, err := h.service.GetTree(c.Request.Context(), forceRefresh)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, tree)
}
