package v1

import (
	"strconv"

	"github.com/gin-gonic/gin"

	contentEntity "NetyAdmin/internal/domain/entity/content"
	clientDto "NetyAdmin/internal/interface/client/dto/v1"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	contentService "NetyAdmin/internal/service/content"
)

type ContentHandler struct {
	articleSvc     contentService.ArticleService
	categorySvc    contentService.CategoryService
	bannerGroupSvc contentService.BannerGroupService
	bannerItemSvc  contentService.BannerItemService
}

func NewContentHandler(
	articleSvc contentService.ArticleService,
	categorySvc contentService.CategoryService,
	bannerGroupSvc contentService.BannerGroupService,
	bannerItemSvc contentService.BannerItemService,
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

	articles, total, err := h.articleSvc.ListPublishedByCategoryIDs(c.Request.Context(), req.Page, req.PageSize, ids, req.Keyword)
	if err != nil {
		response.Fail(c, err)
		return
	}

	items := make([]clientDto.ClientArticleItemVO, 0, len(articles))
	for _, a := range articles {
		items = append(items, articleToItemVO(a))
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

	article, err := h.articleSvc.GetPublishedByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, err)
		return
	}

	_ = h.articleSvc.IncrementViewCount(c.Request.Context(), uint(id))

	response.Success(c, articleToDetailVO(article))
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

	group, err := h.bannerGroupSvc.GetByCode(c.Request.Context(), code)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, bannerGroupToClientVO(group))
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

func articleToItemVO(a *contentEntity.ContentArticle) clientDto.ClientArticleItemVO {
	categoryName := ""
	if a.Category != nil {
		categoryName = a.Category.Name
	}
	return clientDto.ClientArticleItemVO{
		ID:           a.ID,
		CategoryID:   a.CategoryID,
		CategoryName: categoryName,
		Title:        a.Title,
		TitleColor:   a.TitleColor,
		CoverImage:   a.CoverImage,
		Summary:      a.Summary,
		ContentType:  string(a.ContentType),
		Author:       a.Author,
		Source:       a.Source,
		IsTop:        a.IsTop,
		IsHot:        a.IsHot,
		IsRecommend:  a.IsRecommend,
		ViewCount:    a.ViewCount,
		LikeCount:    a.LikeCount,
		CommentCount: a.CommentCount,
		PublishedAt:  a.PublishedAt,
		CreatedAt:    a.CreatedAt,
	}
}

func articleToDetailVO(a *contentEntity.ContentArticle) clientDto.ClientArticleDetailVO {
	categoryName := ""
	if a.Category != nil {
		categoryName = a.Category.Name
	}
	return clientDto.ClientArticleDetailVO{
		ID:           a.ID,
		CategoryID:   a.CategoryID,
		CategoryName: categoryName,
		Title:        a.Title,
		TitleColor:   a.TitleColor,
		CoverImage:   a.CoverImage,
		Summary:      a.Summary,
		Content:      a.Content,
		ContentType:  string(a.ContentType),
		Author:       a.Author,
		Source:       a.Source,
		Keywords:     a.Keywords,
		Tags:         a.Tags,
		IsTop:        a.IsTop,
		IsHot:        a.IsHot,
		IsRecommend:  a.IsRecommend,
		AllowComment: a.AllowComment,
		ViewCount:    a.ViewCount,
		LikeCount:    a.LikeCount,
		CommentCount: a.CommentCount,
		PublishedAt:  a.PublishedAt,
		CreatedAt:    a.CreatedAt,
	}
}

func bannerGroupToClientVO(g *contentEntity.ContentBannerGroup) clientDto.ClientBannerGroupVO {
	banners := make([]clientDto.ClientBannerItemVO, 0, len(g.Banners))
	for _, b := range g.Banners {
		if !b.IsInTimeRange() {
			continue
		}
		banners = append(banners, clientDto.ClientBannerItemVO{
			ID:           b.ID,
			Title:        b.Title,
			Subtitle:     b.Subtitle,
			ImageURL:     b.ImageURL,
			ImageAlt:     b.ImageAlt,
			LinkType:     string(b.LinkType),
			LinkURL:      b.LinkURL,
			Content:      b.Content,
			CustomParams: b.CustomParams,
			Sort:         b.Sort,
		})
	}
	return clientDto.ClientBannerGroupVO{
		ID:          g.ID,
		Name:        g.Name,
		Code:        g.Code,
		Description: g.Description,
		Position:    g.Position,
		Width:       g.Width,
		Height:      g.Height,
		AutoPlay:    g.AutoPlay,
		Interval:    g.Interval,
		Banners:     banners,
	}
}
