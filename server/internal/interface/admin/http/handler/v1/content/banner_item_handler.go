package content

import (
	"strconv"

	"github.com/gin-gonic/gin"

	contentDto "NetyAdmin/internal/interface/admin/dto/content"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	contentService "NetyAdmin/internal/service/content/admin"
)

type ContentBannerItemHandler struct {
	service contentService.BannerItemService
}

func NewContentBannerItemHandler(service contentService.BannerItemService) *ContentBannerItemHandler {
	return &ContentBannerItemHandler{service: service}
}

// @Summary      获取Banner项列表
// @Description  分页获取Banner项列表，支持按组、标题、状态、时间范围筛选
// @Tags         Banner项管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        groupId query int false "Banner组ID"
// @Param        title query string false "标题"
// @Param        status query string false "状态(0/1)"
// @Param        startTime query string false "开始时间"
// @Param        endTime query string false "结束时间"
// @Success      200 {object} response.Response "Banner项列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/banner-items [get]
func (h *ContentBannerItemHandler) List(c *gin.Context) {
	var req contentDto.ContentBannerItemListQueryDTO
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

	items, total, err := h.service.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, items)
}

// @Summary      获取Banner项详情
// @Description  根据ID获取单个Banner项详情
// @Tags         Banner项管理
// @Accept       json
// @Produce      json
// @Param        id path int true "Banner项ID"
// @Success      200 {object} response.Response "Banner项详情"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/banner-items/{id} [get]
func (h *ContentBannerItemHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	item, err := h.service.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, item)
}

// @Summary      创建Banner项
// @Description  新建一个Banner项
// @Tags         Banner项管理
// @Accept       json
// @Produce      json
// @Param        req body content.CreateContentBannerItemDTO true "创建Banner项参数"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/banner-items [post]
func (h *ContentBannerItemHandler) Create(c *gin.Context) {
	var req contentDto.CreateContentBannerItemDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	item, err := h.service.Create(c.Request.Context(), operatorID, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, item)
}

// @Summary      更新Banner项
// @Description  根据ID更新Banner项信息
// @Tags         Banner项管理
// @Accept       json
// @Produce      json
// @Param        id path int true "Banner项ID"
// @Param        req body content.UpdateContentBannerItemDTO true "更新Banner项参数"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/banner-items/{id} [put]
func (h *ContentBannerItemHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	var req contentDto.UpdateContentBannerItemDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	item, err := h.service.Update(c.Request.Context(), operatorID, uint(id), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, item)
}

// @Summary      删除Banner项
// @Description  根据ID删除Banner项
// @Tags         Banner项管理
// @Accept       json
// @Produce      json
// @Param        id path int true "Banner项ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/banner-items/{id} [delete]
func (h *ContentBannerItemHandler) Delete(c *gin.Context) {
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

func (h *ContentBannerItemHandler) GetByGroupID(c *gin.Context) {
	groupIDStr := c.Param("groupId")
	groupID, err := strconv.ParseUint(groupIDStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	items, err := h.service.GetByGroupID(c.Request.Context(), uint(groupID))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, items)
}
