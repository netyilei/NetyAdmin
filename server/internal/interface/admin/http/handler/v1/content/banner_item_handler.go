package content

import (
	"strconv"

	"github.com/gin-gonic/gin"

	contentDto "NetyAdmin/internal/interface/admin/dto/content"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	contentService "NetyAdmin/internal/service/content"
)

type ContentBannerItemHandler struct {
	service contentService.BannerItemService
}

func NewContentBannerItemHandler(service contentService.BannerItemService) *ContentBannerItemHandler {
	return &ContentBannerItemHandler{service: service}
}

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

func (h *ContentBannerItemHandler) Create(c *gin.Context) {
	var req contentDto.CreateContentBannerItemDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	adminID, _ := c.Get("adminID")
	operatorID := adminID.(uint)

	item, err := h.service.Create(c.Request.Context(), operatorID, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, item)
}

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

	adminID, _ := c.Get("adminID")
	operatorID := adminID.(uint)

	item, err := h.service.Update(c.Request.Context(), operatorID, uint(id), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, item)
}

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
