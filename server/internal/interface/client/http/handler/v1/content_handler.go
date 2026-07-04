package v1

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"

	clientDto "NetyAdmin/internal/interface/client/dto/v1"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	contentClientService "NetyAdmin/internal/service/content/client"
)

type ContentHandler struct {
	articleSvc     contentClientService.ArticleService
	categorySvc    contentClientService.CategoryService
	bannerGroupSvc contentClientService.BannerGroupService
	bannerItemSvc  contentClientService.BannerItemService
}

func NewContentHandler(
	articleSvc contentClientService.ArticleService,
	categorySvc contentClientService.CategoryService,
	bannerGroupSvc contentClientService.BannerGroupService,
	bannerItemSvc contentClientService.BannerItemService,
) *ContentHandler {
	return &ContentHandler{
		articleSvc:     articleSvc,
		categorySvc:    categorySvc,
		bannerGroupSvc: bannerGroupSvc,
		bannerItemSvc:  bannerItemSvc,
	}
}

// @Summary      文章列表
// @Description  分页获取已发布文章列表，支持按分类与关键词筛选
// @Tags         客户端-内容
// @Accept       json
// @Produce      json
// @Param        page query int false "页码"
// @Param        pageSize query int false "每页数量"
// @Param        categoryId query int false "分类ID"
// @Param        keyword query string false "搜索关键词"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/content/articles [get]
func (h *ContentHandler) ListArticles(c *gin.Context) {
	var req clientDto.ClientArticleListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	ids, err := h.categorySvc.GetDescendantIDs(c.Request.Context(), req.CategoryID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	items, total, err := h.articleSvc.ListPublishedVO(c.Request.Context(), req.Page, req.PageSize, ids, req.Keyword)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.SuccessWithPage(c, req.Page, req.PageSize, total, items)
}

// @Summary      文章详情
// @Description  根据文章ID获取已发布文章详情并增加浏览数
// @Tags         客户端-内容
// @Accept       json
// @Produce      json
// @Param        id path int true "文章ID"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/content/article/{id} [get]
func (h *ContentHandler) GetArticle(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	article, err := h.articleSvc.GetPublishedVO(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	// IncrementViewCount 是 best-effort 浏览数自增：失败仅 Warn 不阻断响应
	// （响应已写入 article 数据，浏览数少计一次不影响正确性，下次访问会补上）。
	if err := h.articleSvc.IncrementViewCount(c.Request.Context(), uint(id)); err != nil {
		slog.Warn("IncrementViewCount failed (best-effort, response not affected)",
			"articleID", id, "error", err)
	}

	response.Success(c, article)
}

// @Summary      点赞文章
// @Description  根据文章ID点赞并增加点赞数
// @Tags         客户端-内容
// @Accept       json
// @Produce      json
// @Param        id path int true "文章ID"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/content/article/{id}/like [post]
func (h *ContentHandler) LikeArticle(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.articleSvc.IncrementLikeCount(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}

// @Summary      获取Banner组
// @Description  根据编码获取Banner组及其Banner项列表
// @Tags         客户端-内容
// @Accept       json
// @Produce      json
// @Param        code path string true "Banner组编码"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/content/banners/{code} [get]
func (h *ContentHandler) GetBannerGroupByCode(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	group, err := h.bannerGroupSvc.GetByCodeVO(c.Request.Context(), code)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, group)
}

// @Summary      点击Banner
// @Description  根据Banner项ID记录点击并增加点击数
// @Tags         客户端-内容
// @Accept       json
// @Produce      json
// @Param        id path int true "Banner项ID"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/content/banners/{id}/click [post]
func (h *ContentHandler) ClickBanner(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams)
		return
	}

	if err := h.bannerItemSvc.IncrementClickCount(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, nil)
}
