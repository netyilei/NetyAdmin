package content

import (
	contentService "silentorder/internal/service/content"
)

type ContentHandler struct {
	Category    *ContentCategoryHandler
	Article     *ContentArticleHandler
	BannerGroup *ContentBannerGroupHandler
	BannerItem  *ContentBannerItemHandler
}

func NewContentHandler(
	categoryService contentService.CategoryService,
	articleService contentService.ArticleService,
	bannerGroupService contentService.BannerGroupService,
	bannerItemService contentService.BannerItemService,
) *ContentHandler {
	return &ContentHandler{
		Category:    NewContentCategoryHandler(categoryService),
		Article:     NewContentArticleHandler(articleService),
		BannerGroup: NewContentBannerGroupHandler(bannerGroupService),
		BannerItem:  NewContentBannerItemHandler(bannerItemService),
	}
}
