package v1

import (
	"github.com/gin-gonic/gin"

	"NetyAdmin/internal/interface/client/http/handler/v1"
)

type contentRouter struct {
	handler *v1.ContentHandler
}

func NewContentRouter(handler *v1.ContentHandler) *contentRouter {
	return &contentRouter{handler: handler}
}

func (r *contentRouter) RegisterPublic(publicGroup *gin.RouterGroup) {
	content := publicGroup.Group("/content")
	{
		content.GET("/categories/tree", r.handler.GetCategoryTree)
		content.GET("/articles", r.handler.ListArticles)
		content.GET("/article/:id", r.handler.GetArticle)
		content.GET("/banners/:code", r.handler.GetBannerGroupByCode)
	}
}

func (r *contentRouter) RegisterAuth(authGroup *gin.RouterGroup) {
	content := authGroup.Group("/content")
	{
		content.POST("/article/:id/like", r.handler.LikeArticle)
		content.POST("/banners/:id/click", r.handler.ClickBanner)
	}
}

func (r *contentRouter) RegisterPermission(_ *gin.RouterGroup) {}
