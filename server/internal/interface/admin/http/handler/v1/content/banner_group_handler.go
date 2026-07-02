package content

import (
	"strconv"

	contentDto "NetyAdmin/internal/interface/admin/dto/content"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	contentService "NetyAdmin/internal/service/content"

	"github.com/gin-gonic/gin"
)

type ContentBannerGroupHandler struct {
	service contentService.BannerGroupService
}

func NewContentBannerGroupHandler(service contentService.BannerGroupService) *ContentBannerGroupHandler {
	return &ContentBannerGroupHandler{service: service}
}

// @Summary      获取Banner组列表
// @Description  分页获取Banner组列表，支持按名称、编码、描述、位置、状态筛选
// @Tags         Banner组管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        name query string false "组名称"
// @Param        code query string false "组编码"
// @Param        description query string false "描述"
// @Param        position query string false "位置"
// @Param        status query string false "状态(0/1)"
// @Success      200 {object} response.Response "Banner组列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/banner-groups [get]
func (h *ContentBannerGroupHandler) List(c *gin.Context) {
	var req contentDto.ContentBannerGroupListQueryDTO
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if req.Current <= 0 {
		req.Current = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}

	groups, total, err := h.service.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, groups)
}

// @Summary      获取Banner组详情
// @Description  根据ID获取单个Banner组详情
// @Tags         Banner组管理
// @Accept       json
// @Produce      json
// @Param        id path int true "Banner组ID"
// @Success      200 {object} response.Response "Banner组详情"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/banner-groups/{id} [get]
func (h *ContentBannerGroupHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	group, err := h.service.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, group)
}

func (h *ContentBannerGroupHandler) GetWithBanners(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	group, err := h.service.GetByIDWithBanners(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, group)
}

// @Summary      创建Banner组
// @Description  新建一个Banner组
// @Tags         Banner组管理
// @Accept       json
// @Produce      json
// @Param        req body content.CreateContentBannerGroupDTO true "创建Banner组参数"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/banner-groups [post]
func (h *ContentBannerGroupHandler) Create(c *gin.Context) {
	var req contentDto.CreateContentBannerGroupDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	group, err := h.service.Create(c.Request.Context(), operatorID, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, group)
}

// @Summary      更新Banner组
// @Description  根据ID更新Banner组信息
// @Tags         Banner组管理
// @Accept       json
// @Produce      json
// @Param        id path int true "Banner组ID"
// @Param        req body content.UpdateContentBannerGroupDTO true "更新Banner组参数"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/banner-groups/{id} [put]
func (h *ContentBannerGroupHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	var req contentDto.UpdateContentBannerGroupDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	group, err := h.service.Update(c.Request.Context(), operatorID, uint(id), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, group)
}

// @Summary      删除Banner组
// @Description  根据ID删除Banner组
// @Tags         Banner组管理
// @Accept       json
// @Produce      json
// @Param        id path int true "Banner组ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/banner-groups/{id} [delete]
func (h *ContentBannerGroupHandler) Delete(c *gin.Context) {
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

func (h *ContentBannerGroupHandler) GetAll(c *gin.Context) {
	groups, err := h.service.GetAll(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, groups)
}
