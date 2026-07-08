package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	contentEntity "NetyAdmin/internal/domain/entity/content"
	clientDto "NetyAdmin/internal/interface/client/dto/v1"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/pagination"
	contentRepo "NetyAdmin/internal/repository/content"
)

// ArticleService 客户端文章服务
// 仅暴露客户端需要的查询方法，返回 client DTO VO；entity 转换在 service 内部完成。
type ArticleService interface {
	ListPublishedVO(ctx context.Context, page, pageSize int, categoryIDs []uint, keyword string) ([]*clientDto.ClientArticleItemVO, int64, error)
	GetPublishedVO(ctx context.Context, id uint) (*clientDto.ClientArticleDetailVO, error)
	IncrementViewCount(ctx context.Context, id uint) error
	IncrementLikeCount(ctx context.Context, id uint) error
}

type articleService struct {
	repo         contentRepo.ContentArticleRepository
	categoryRepo contentRepo.ContentCategoryRepository
	cache        cache.ConfigCache
	watcher      configsync.ConfigWatcher
}

func NewArticleService(repo contentRepo.ContentArticleRepository, categoryRepo contentRepo.ContentCategoryRepository, cache cache.ConfigCache, watcher configsync.ConfigWatcher) ArticleService {
	return &articleService{repo: repo, categoryRepo: categoryRepo, cache: cache, watcher: watcher}
}

func (s *articleService) getArticleCacheTTL() time.Duration {
	val, ok := s.watcher.GetConfig(cache.ConfigGroupContentCache, cache.ConfigKeyArticleCacheTTL)
	if ok {
		if mins, err := time.ParseDuration(val + "m"); err == nil {
			return mins
		}
	}
	return 30 * time.Minute
}

func (s *articleService) ListPublishedVO(ctx context.Context, page, pageSize int, categoryIDs []uint, keyword string) ([]*clientDto.ClientArticleItemVO, int64, error) {
	page, pageSize = pagination.NormalizePagination(page, pageSize)

	var primaryCategoryID uint
	if len(categoryIDs) > 0 {
		primaryCategoryID = categoryIDs[0]
	}

	cacheKey := cache.KeyContentArticleList(primaryCategoryID, page, pageSize, keyword)
	cacheTags := []string{cache.TagContentArticle}

	type cachedResult struct {
		Articles []*contentEntity.ContentArticle
		Total    int64
	}

	var result cachedResult
	loader := func() (interface{}, error) {
		articles, total, err := s.repo.ListPublished(ctx, &contentRepo.ContentArticlePublishedQuery{
			CategoryIDs: categoryIDs,
			Keyword:     keyword,
			Current:     page,
			Size:        pageSize,
		})
		if err != nil {
			return nil, err
		}
		return cachedResult{Articles: articles, Total: total}, nil
	}

	err := s.cache.FetchFast(ctx, cacheKey, "content_article_list", cacheTags, s.getArticleCacheTTL(), &result, loader)
	if err != nil {
		return nil, 0, err
	}

	items := make([]*clientDto.ClientArticleItemVO, 0, len(result.Articles))
	for _, a := range result.Articles {
		items = append(items, articleToItemVO(a))
	}
	return items, result.Total, nil
}

func (s *articleService) GetPublishedVO(ctx context.Context, id uint) (*clientDto.ClientArticleDetailVO, error) {
	cacheKey := cache.KeyContentArticleDetail(id)
	cacheTags := []string{cache.TagContentArticle}

	var article *contentEntity.ContentArticle
	loader := func() (interface{}, error) {
		a, err := s.repo.GetByIDWithCategory(ctx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.New(errorx.CodeNotFound, "文章不存在")
			}
			slog.Error("repo.GetByIDWithCategory failed", "articleID", id, "err", err)
			return nil, fmt.Errorf("repo.GetByIDWithCategory: %w", err)
		}
		if !a.IsPublished() || !a.IsEnabled() {
			return nil, errorx.New(errorx.CodeNotFound, "文章不存在")
		}
		return a, nil
	}

	err := s.cache.FetchFast(ctx, cacheKey, "content_article_detail", cacheTags, s.getArticleCacheTTL(), &article, loader)
	if err != nil {
		return nil, err
	}

	return articleToDetailVO(article), nil
}

func (s *articleService) IncrementViewCount(ctx context.Context, id uint) error {
	return s.repo.IncrementViewCount(ctx, id)
}

func (s *articleService) IncrementLikeCount(ctx context.Context, id uint) error {
	return s.repo.IncrementLikeCount(ctx, id)
}

// articleToItemVO entity 转换为 client DTO VO（service 内部使用，不暴露给 handler）
// nil-safe：缓存反序列化可能返回 nil 指针 + nil error（如 JSON null），需显式检查避免 panic。
func articleToItemVO(a *contentEntity.ContentArticle) *clientDto.ClientArticleItemVO {
	if a == nil {
		return nil
	}
	categoryName := ""
	if a.Category != nil {
		categoryName = a.Category.Name
	}
	return &clientDto.ClientArticleItemVO{
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

// articleToDetailVO entity 转换为 client DTO VO
// nil-safe：与 articleToItemVO 一致，缓存反序列化可能返回 nil。
func articleToDetailVO(a *contentEntity.ContentArticle) *clientDto.ClientArticleDetailVO {
	if a == nil {
		return nil
	}
	categoryName := ""
	if a.Category != nil {
		categoryName = a.Category.Name
	}
	return &clientDto.ClientArticleDetailVO{
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
