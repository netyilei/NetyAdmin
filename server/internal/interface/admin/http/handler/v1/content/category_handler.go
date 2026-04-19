package content

import (
	"strconv"

	contentDto "NetyAdmin/internal/interface/admin/dto/content"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	contentService "NetyAdmin/internal/service/content"

	"github.com/gin-gonic/gin"
)

type ContentCategoryHandler struct {
	service contentService.CategoryService
}

func NewContentCategoryHandler(service contentService.CategoryService) *ContentCategoryHandler {
	return &ContentCategoryHandler{service: service}
}

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

func (h *ContentCategoryHandler) Create(c *gin.Context) {
	var req contentDto.CreateContentCategoryDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	adminID, _ := c.Get("adminID")
	operatorID := adminID.(uint)

	category, err := h.service.Create(c.Request.Context(), operatorID, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, category)
}

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

	adminID, _ := c.Get("adminID")
	operatorID := adminID.(uint)

	category, err := h.service.Update(c.Request.Context(), operatorID, uint(id), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, category)
}

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

func (h *ContentCategoryHandler) Tree(c *gin.Context) {
	forceRefresh := c.Query("refresh") == "true"
	tree, err := h.service.GetTree(c.Request.Context(), forceRefresh)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, tree)
}
