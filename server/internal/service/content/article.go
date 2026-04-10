package content

import (
	"context"
	"errors"
	"time"

	contentDto "silentorder/internal/interface/admin/dto/content"
	contentEntity "silentorder/internal/domain/entity/content"
	contentRepo "silentorder/internal/repository/content"
)

type ArticleService interface {
	Create(ctx context.Context, adminID uint, req *contentDto.CreateContentArticleDTO) (*contentEntity.ContentArticle, error)
	Update(ctx context.Context, adminID uint, id uint, req *contentDto.UpdateContentArticleDTO) (*contentEntity.ContentArticle, error)
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*contentEntity.ContentArticle, error)
	List(ctx context.Context, query *contentDto.ContentArticleListQueryDTO) ([]*contentEntity.ContentArticle, int64, error)
	Publish(ctx context.Context, id uint) error
	Unpublish(ctx context.Context, id uint) error
	SetTop(ctx context.Context, id uint, req *contentDto.SetArticleTopDTO) error
}

type articleService struct {
	repo         contentRepo.ContentArticleRepository
	categoryRepo contentRepo.ContentCategoryRepository
}

func NewArticleService(repo contentRepo.ContentArticleRepository, categoryRepo contentRepo.ContentCategoryRepository) ArticleService {
	return &articleService{repo: repo, categoryRepo: categoryRepo}
}

func (s *articleService) Create(ctx context.Context, adminID uint, req *contentDto.CreateContentArticleDTO) (*contentEntity.ContentArticle, error) {
	_, err := s.categoryRepo.GetByID(ctx, req.CategoryID)
	if err != nil {
		return nil, errors.New("分类不存在")
	}

	contentType := contentEntity.ContentTypeRichText
	if req.ContentType == "plaintext" {
		contentType = contentEntity.ContentTypePlainText
	}

	publishStatus := contentEntity.PublishStatusDraft
	if req.PublishStatus == "published" {
		publishStatus = contentEntity.PublishStatusPublished
	} else if req.PublishStatus == "scheduled" {
		publishStatus = contentEntity.PublishStatusScheduled
	}

	var scheduledAt *time.Time
	if req.ScheduledAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ScheduledAt)
		if err == nil {
			scheduledAt = &t
		}
	}

	article := &contentEntity.ContentArticle{
		CategoryID:    req.CategoryID,
		Title:         req.Title,
		TitleColor:    req.TitleColor,
		CoverImage:    req.CoverImage,
		Summary:       req.Summary,
		Content:       req.Content,
		ContentType:   contentType,
		Author:        req.Author,
		Source:        req.Source,
		Keywords:      req.Keywords,
		Tags:          req.Tags,
		IsTop:         req.IsTop,
		TopSort:       req.TopSort,
		IsHot:         req.IsHot,
		IsRecommend:   req.IsRecommend,
		AllowComment:  req.AllowComment,
		PublishStatus: publishStatus,
		ScheduledAt:   scheduledAt,
	}
	article.CreatedBy = adminID
	article.UpdatedBy = adminID

	if publishStatus == contentEntity.PublishStatusPublished {
		now := time.Now()
		article.PublishedAt = &now
	}

	if err := s.repo.Create(ctx, article); err != nil {
		return nil, err
	}

	return article, nil
}

func (s *articleService) Update(ctx context.Context, adminID uint, id uint, req *contentDto.UpdateContentArticleDTO) (*contentEntity.ContentArticle, error) {
	article, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.CategoryID != nil {
		_, err := s.categoryRepo.GetByID(ctx, *req.CategoryID)
		if err != nil {
			return nil, errors.New("分类不存在")
		}
		article.CategoryID = *req.CategoryID
	}

	if req.Title != "" {
		article.Title = req.Title
	}
	if req.TitleColor != "" {
		article.TitleColor = req.TitleColor
	}
	if req.CoverImage != "" {
		article.CoverImage = req.CoverImage
	}
	article.Summary = req.Summary
	article.Content = req.Content
	if req.ContentType != "" {
		if req.ContentType == "plaintext" {
			article.ContentType = contentEntity.ContentTypePlainText
		} else {
			article.ContentType = contentEntity.ContentTypeRichText
		}
	}
	if req.Author != "" {
		article.Author = req.Author
	}
	if req.Source != "" {
		article.Source = req.Source
	}
	article.Keywords = req.Keywords
	article.Tags = req.Tags
	if req.IsTop != nil {
		article.IsTop = *req.IsTop
	}
	if req.TopSort != nil {
		article.TopSort = *req.TopSort
	}
	if req.IsHot != nil {
		article.IsHot = *req.IsHot
	}
	if req.IsRecommend != nil {
		article.IsRecommend = *req.IsRecommend
	}
	if req.AllowComment != nil {
		article.AllowComment = *req.AllowComment
	}
	if req.PublishStatus != "" {
		article.PublishStatus = contentEntity.PublishStatus(req.PublishStatus)
	}
	if req.ScheduledAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ScheduledAt)
		if err == nil {
			article.ScheduledAt = &t
		}
	}
	article.UpdatedBy = adminID

	if err := s.repo.Update(ctx, article); err != nil {
		return nil, err
	}

	return article, nil
}

func (s *articleService) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *articleService) GetByID(ctx context.Context, id uint) (*contentEntity.ContentArticle, error) {
	return s.repo.GetByIDWithCategory(ctx, id)
}

func (s *articleService) List(ctx context.Context, query *contentDto.ContentArticleListQueryDTO) ([]*contentEntity.ContentArticle, int64, error) {
	repoQuery := &contentRepo.ContentArticleQuery{
		Current:       query.Current,
		Size:          query.Size,
		CategoryID:    query.CategoryID,
		Title:         query.Title,
		PublishStatus: query.PublishStatus,
		IsTop:         query.IsTop,
		IsHot:         query.IsHot,
		IsRecommend:   query.IsRecommend,
		Author:        query.Author,
	}

	if query.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, query.StartTime); err == nil {
			repoQuery.StartTime = &t
		}
	}
	if query.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, query.EndTime); err == nil {
			repoQuery.EndTime = &t
		}
	}

	return s.repo.List(ctx, repoQuery)
}

func (s *articleService) Publish(ctx context.Context, id uint) error {
	_, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.repo.Publish(ctx, id, time.Now())
}

func (s *articleService) Unpublish(ctx context.Context, id uint) error {
	_, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.repo.Unpublish(ctx, id)
}

func (s *articleService) SetTop(ctx context.Context, id uint, req *contentDto.SetArticleTopDTO) error {
	_, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.repo.SetTop(ctx, id, req.IsTop, req.TopSort)
}
