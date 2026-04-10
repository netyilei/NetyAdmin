package content

import (
	"strconv"

	contentDto "netyadmin/internal/interface/admin/dto/content"
	"netyadmin/internal/pkg/errorx"
	"netyadmin/internal/pkg/response"
	contentService "netyadmin/internal/service/content"

	"github.com/gin-gonic/gin"
)

type ContentArticleHandler struct {
	service contentService.ArticleService
}

func NewContentArticleHandler(service contentService.ArticleService) *ContentArticleHandler {
	return &ContentArticleHandler{service: service}
}

func (h *ContentArticleHandler) List(c *gin.Context) {
	var req contentDto.ContentArticleListQueryDTO
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

	articles, total, err := h.service.List(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Current, req.Size, total, articles)
}

func (h *ContentArticleHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	article, err := h.service.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, article)
}

func (h *ContentArticleHandler) Create(c *gin.Context) {
	var req contentDto.CreateContentArticleDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	userID, _ := c.Get("userID")
	adminID := userID.(uint)

	article, err := h.service.Create(c.Request.Context(), adminID, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, article)
}

func (h *ContentArticleHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	var req contentDto.UpdateContentArticleDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	userID, _ := c.Get("userID")
	adminID := userID.(uint)

	article, err := h.service.Update(c.Request.Context(), adminID, uint(id), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, article)
}

func (h *ContentArticleHandler) Delete(c *gin.Context) {
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

func (h *ContentArticleHandler) Publish(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.service.Publish(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

func (h *ContentArticleHandler) Unpublish(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.service.Unpublish(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

func (h *ContentArticleHandler) SetTop(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	var req contentDto.SetArticleTopDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.service.SetTop(c.Request.Context(), uint(id), &req); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
