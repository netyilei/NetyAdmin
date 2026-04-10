package content

import (
	"strconv"

	contentDto "silentorder/internal/interface/admin/dto/content"
	"silentorder/internal/pkg/errorx"
	"silentorder/internal/pkg/response"
	contentService "silentorder/internal/service/content"

	"github.com/gin-gonic/gin"
)

type ContentBannerGroupHandler struct {
	service contentService.BannerGroupService
}

func NewContentBannerGroupHandler(service contentService.BannerGroupService) *ContentBannerGroupHandler {
	return &ContentBannerGroupHandler{service: service}
}

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

func (h *ContentBannerGroupHandler) Create(c *gin.Context) {
	var req contentDto.CreateContentBannerGroupDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	userID, _ := c.Get("userID")
	adminID := userID.(uint)

	group, err := h.service.Create(c.Request.Context(), adminID, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, group)
}

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

	userID, _ := c.Get("userID")
	adminID := userID.(uint)

	group, err := h.service.Update(c.Request.Context(), adminID, uint(id), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, group)
}

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
