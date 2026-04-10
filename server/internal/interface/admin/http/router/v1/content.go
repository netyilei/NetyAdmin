package v1

import (
	"github.com/gin-gonic/gin"

	"netyadmin/internal/interface/admin/http/handler/v1/content"
)

type ContentRouter struct {
	category    *content.ContentCategoryHandler
	article     *content.ContentArticleHandler
	bannerGroup *content.ContentBannerGroupHandler
	bannerItem  *content.ContentBannerItemHandler
}

func NewContentRouter(
	category *content.ContentCategoryHandler,
	article *content.ContentArticleHandler,
	bannerGroup *content.ContentBannerGroupHandler,
	bannerItem *content.ContentBannerItemHandler,
) *ContentRouter {
	return &ContentRouter{
		category:    category,
		article:     article,
		bannerGroup: bannerGroup,
		bannerItem:  bannerItem,
	}
}

func (r *ContentRouter) RegisterPublic(group *gin.RouterGroup) {}

func (r *ContentRouter) RegisterAuth(group *gin.RouterGroup) {}

func (r *ContentRouter) RegisterPermission(group *gin.RouterGroup) {
	r.registerCategory(group)
	r.registerArticle(group)
	r.registerBannerGroup(group)
	r.registerBannerItem(group)
}

func (r *ContentRouter) registerCategory(group *gin.RouterGroup) {
	categoryGroup := group.Group("/content/categories")
	{
		categoryGroup.GET("", r.category.List)
		categoryGroup.GET("/tree", r.category.Tree)
		categoryGroup.GET("/:id", r.category.Get)
		categoryGroup.POST("", r.category.Create)
		categoryGroup.PUT("/:id", r.category.Update)
		categoryGroup.DELETE("/:id", r.category.Delete)
	}
}

func (r *ContentRouter) registerArticle(group *gin.RouterGroup) {
	articleGroup := group.Group("/content/articles")
	{
		articleGroup.GET("", r.article.List)
		articleGroup.GET("/:id", r.article.Get)
		articleGroup.POST("", r.article.Create)
		articleGroup.PUT("/:id", r.article.Update)
		articleGroup.DELETE("/:id", r.article.Delete)
		articleGroup.PUT("/:id/publish", r.article.Publish)
		articleGroup.PUT("/:id/unpublish", r.article.Unpublish)
		articleGroup.PUT("/:id/top", r.article.SetTop)
	}
}

func (r *ContentRouter) registerBannerGroup(group *gin.RouterGroup) {
	bannerGroupGroup := group.Group("/content/banner-groups")
	{
		bannerGroupGroup.GET("", r.bannerGroup.List)
		bannerGroupGroup.GET("/:id", r.bannerGroup.Get)
		bannerGroupGroup.POST("", r.bannerGroup.Create)
		bannerGroupGroup.PUT("/:id", r.bannerGroup.Update)
		bannerGroupGroup.DELETE("/:id", r.bannerGroup.Delete)
	}
}

func (r *ContentRouter) registerBannerItem(group *gin.RouterGroup) {
	bannerItemGroup := group.Group("/content/banner-items")
	{
		bannerItemGroup.GET("", r.bannerItem.List)
		bannerItemGroup.GET("/:id", r.bannerItem.Get)
		bannerItemGroup.POST("", r.bannerItem.Create)
		bannerItemGroup.PUT("/:id", r.bannerItem.Update)
		bannerItemGroup.DELETE("/:id", r.bannerItem.Delete)
	}
}
