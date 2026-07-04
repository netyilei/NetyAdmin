package content

import (
	"strconv"

	contentDto "NetyAdmin/internal/interface/admin/dto/content"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	contentService "NetyAdmin/internal/service/content/admin"

	"github.com/gin-gonic/gin"
)

type ContentArticleHandler struct {
	service contentService.ArticleService
}

func NewContentArticleHandler(service contentService.ArticleService) *ContentArticleHandler {
	return &ContentArticleHandler{service: service}
}

// @Summary      获取文章列表
// @Description  分页获取文章列表，支持按分类、标题、发布状态、置顶、热门、推荐、作者、时间范围筛选
// @Tags         文章管理
// @Accept       json
// @Produce      json
// @Param        current query int false "页码"
// @Param        size query int false "每页数量"
// @Param        categoryId query int false "分类ID"
// @Param        title query string false "文章标题"
// @Param        publishStatus query string false "发布状态(draft/published/scheduled)"
// @Param        isTop query bool false "是否置顶"
// @Param        isHot query bool false "是否热门"
// @Param        isRecommend query bool false "是否推荐"
// @Param        author query string false "作者"
// @Param        startTime query string false "开始时间"
// @Param        endTime query string false "结束时间"
// @Success      200 {object} response.Response "文章列表"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/articles [get]
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

// @Summary      获取文章详情
// @Description  根据ID获取单个文章详情
// @Tags         文章管理
// @Accept       json
// @Produce      json
// @Param        id path int true "文章ID"
// @Success      200 {object} response.Response "文章详情"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/articles/{id} [get]
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

// @Summary      创建文章
// @Description  新建一篇文章
// @Tags         文章管理
// @Accept       json
// @Produce      json
// @Param        req body content.CreateContentArticleDTO true "创建文章参数"
// @Success      200 {object} response.Response "创建成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/articles [post]
func (h *ContentArticleHandler) Create(c *gin.Context) {
	var req contentDto.CreateContentArticleDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	article, err := h.service.Create(c.Request.Context(), operatorID, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, article)
}

// @Summary      更新文章
// @Description  根据ID更新文章信息
// @Tags         文章管理
// @Accept       json
// @Produce      json
// @Param        id path int true "文章ID"
// @Param        req body content.UpdateContentArticleDTO true "更新文章参数"
// @Success      200 {object} response.Response "更新成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/articles/{id} [put]
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

	operatorID := c.GetUint("adminID")
	if operatorID == 0 {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}

	article, err := h.service.Update(c.Request.Context(), operatorID, uint(id), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, article)
}

// @Summary      删除文章
// @Description  根据ID删除文章
// @Tags         文章管理
// @Accept       json
// @Produce      json
// @Param        id path int true "文章ID"
// @Success      200 {object} response.Response "删除成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/articles/{id} [delete]
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

// @Summary      发布文章
// @Description  根据ID发布文章
// @Tags         文章管理
// @Accept       json
// @Produce      json
// @Param        id path int true "文章ID"
// @Success      200 {object} response.Response "发布成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/articles/{id}/publish [put]
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

// @Summary      取消发布文章
// @Description  根据ID取消发布文章
// @Tags         文章管理
// @Accept       json
// @Produce      json
// @Param        id path int true "文章ID"
// @Success      200 {object} response.Response "取消发布成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/articles/{id}/unpublish [put]
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

// @Summary      设置文章置顶
// @Description  根据ID设置文章置顶状态及排序
// @Tags         文章管理
// @Accept       json
// @Produce      json
// @Param        id path int true "文章ID"
// @Param        req body content.SetArticleTopDTO true "置顶参数"
// @Success      200 {object} response.Response "设置成功"
// @Security    ApiKeyAuth
// @Router       /admin/v1/content/articles/{id}/top [put]
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
