package content

import (
	"context"
	"time"

	"gorm.io/gorm"

	content "netyadmin/internal/domain/entity/content"
)

type ContentArticleRepository interface {
	Create(ctx context.Context, article *content.ContentArticle) error
	Update(ctx context.Context, article *content.ContentArticle) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*content.ContentArticle, error)
	GetByIDWithCategory(ctx context.Context, id uint) (*content.ContentArticle, error)
	List(ctx context.Context, query *ContentArticleQuery) ([]*content.ContentArticle, int64, error)
	Publish(ctx context.Context, id uint, publishedAt time.Time) error
	Unpublish(ctx context.Context, id uint) error
	SetTop(ctx context.Context, id uint, isTop bool, topSort int) error
	GetScheduledArticles(ctx context.Context) ([]*content.ContentArticle, error)
	PublishScheduled(ctx context.Context, now time.Time) (int64, error)
	IncrementViewCount(ctx context.Context, id uint) error
	IncrementLikeCount(ctx context.Context, id uint) error
}

type ContentArticleQuery struct {
	CategoryID    uint
	Title         string
	Author        string
	PublishStatus string
	IsTop         *bool
	IsHot         *bool
	IsRecommend   *bool
	StartTime     *time.Time
	EndTime       *time.Time
	Current       int
	Size          int
}

type contentArticleRepository struct {
	db *gorm.DB
}

func NewContentArticleRepository(db *gorm.DB) ContentArticleRepository {
	return &contentArticleRepository{db: db}
}

func (r *contentArticleRepository) Create(ctx context.Context, article *content.ContentArticle) error {
	return r.db.WithContext(ctx).Create(article).Error
}

func (r *contentArticleRepository) Update(ctx context.Context, article *content.ContentArticle) error {
	return r.db.WithContext(ctx).Save(article).Error
}

func (r *contentArticleRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&content.ContentArticle{}, id).Error
}

func (r *contentArticleRepository) GetByID(ctx context.Context, id uint) (*content.ContentArticle, error) {
	var article content.ContentArticle
	err := r.db.WithContext(ctx).First(&article, id).Error
	if err != nil {
		return nil, err
	}
	return &article, nil
}

func (r *contentArticleRepository) GetByIDWithCategory(ctx context.Context, id uint) (*content.ContentArticle, error) {
	var article content.ContentArticle
	err := r.db.WithContext(ctx).
		Preload("Category").
		First(&article, id).Error
	if err != nil {
		return nil, err
	}
	return &article, nil
}

func (r *contentArticleRepository) List(ctx context.Context, query *ContentArticleQuery) ([]*content.ContentArticle, int64, error) {
	db := r.db.WithContext(ctx).Model(&content.ContentArticle{})

	if query.CategoryID > 0 {
		db = db.Where("category_id = ?", query.CategoryID)
	}
	if query.Title != "" {
		db = db.Where("title LIKE ?", "%"+query.Title+"%")
	}
	if query.Author != "" {
		db = db.Where("author LIKE ?", "%"+query.Author+"%")
	}
	if query.PublishStatus != "" {
		db = db.Where("publish_status = ?", query.PublishStatus)
	}
	if query.IsTop != nil {
		db = db.Where("is_top = ?", *query.IsTop)
	}
	if query.IsHot != nil {
		db = db.Where("is_hot = ?", *query.IsHot)
	}
	if query.IsRecommend != nil {
		db = db.Where("is_recommend = ?", *query.IsRecommend)
	}
	if query.StartTime != nil {
		db = db.Where("created_at >= ?", query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("created_at <= ?", query.EndTime)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var articles []*content.ContentArticle
	offset := (query.Current - 1) * query.Size
	if offset < 0 {
		offset = 0
	}
	if query.Size <= 0 {
		query.Size = 10
	}

	err := db.Preload("Category").
		Order("is_top DESC, top_sort ASC, created_at DESC").
		Offset(offset).
		Limit(query.Size).
		Find(&articles).Error
	if err != nil {
		return nil, 0, err
	}

	return articles, total, nil
}

func (r *contentArticleRepository) Publish(ctx context.Context, id uint, publishedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&content.ContentArticle{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"publish_status": content.PublishStatusPublished,
			"published_at":   publishedAt,
		}).Error
}

func (r *contentArticleRepository) Unpublish(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&content.ContentArticle{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"publish_status": content.PublishStatusDraft,
			"published_at":   nil,
		}).Error
}

func (r *contentArticleRepository) SetTop(ctx context.Context, id uint, isTop bool, topSort int) error {
	return r.db.WithContext(ctx).Model(&content.ContentArticle{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_top":   isTop,
			"top_sort": topSort,
		}).Error
}

func (r *contentArticleRepository) GetScheduledArticles(ctx context.Context) ([]*content.ContentArticle, error) {
	var articles []*content.ContentArticle
	err := r.db.WithContext(ctx).
		Where("publish_status = ? AND scheduled_at <= ?", content.PublishStatusScheduled, time.Now()).
		Find(&articles).Error
	if err != nil {
		return nil, err
	}
	return articles, nil
}

func (r *contentArticleRepository) PublishScheduled(ctx context.Context, now time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&content.ContentArticle{}).
		Where("publish_status = ? AND scheduled_at <= ?", content.PublishStatusScheduled, now).
		Updates(map[string]interface{}{
			"publish_status": content.PublishStatusPublished,
			"published_at":   gorm.Expr("scheduled_at"),
		})
	return result.RowsAffected, result.Error
}

func (r *contentArticleRepository) IncrementViewCount(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&content.ContentArticle{}).
		Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

func (r *contentArticleRepository) IncrementLikeCount(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&content.ContentArticle{}).
		Where("id = ?", id).
		UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
}
