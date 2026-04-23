package content

import (
	"context"
	"time"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	content "NetyAdmin/internal/domain/entity/content"
	"NetyAdmin/internal/pkg/pagination"
)

type ContentBannerItemRepository interface {
	Create(ctx context.Context, item *content.ContentBannerItem) error
	Update(ctx context.Context, item *content.ContentBannerItem) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*content.ContentBannerItem, error)
	GetByIDWithGroup(ctx context.Context, id uint) (*content.ContentBannerItem, error)
	List(ctx context.Context, query *ContentBannerItemQuery) ([]*content.ContentBannerItem, int64, error)
	GetByGroupID(ctx context.Context, groupID uint) ([]*content.ContentBannerItem, error)
	CountByGroupID(ctx context.Context, groupID uint) (int64, error)
	IncrementViewCount(ctx context.Context, id uint) error
	IncrementClickCount(ctx context.Context, id uint) error
}

type ContentBannerItemQuery struct {
	GroupID   uint   `form:"groupId"`
	Title     string `form:"title"`
	Status    string `form:"status"`
	StartTime string `form:"startTime"`
	EndTime   string `form:"endTime"`
	Current   int    `form:"current"`
	Size      int    `form:"size"`
}

type contentBannerItemRepository struct {
	db *gorm.DB
}

func NewContentBannerItemRepository(db *gorm.DB) ContentBannerItemRepository {
	return &contentBannerItemRepository{db: db}
}

func (r *contentBannerItemRepository) Create(ctx context.Context, item *content.ContentBannerItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *contentBannerItemRepository) Update(ctx context.Context, item *content.ContentBannerItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *contentBannerItemRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&content.ContentBannerItem{}, id).Error
}

func (r *contentBannerItemRepository) GetByID(ctx context.Context, id uint) (*content.ContentBannerItem, error) {
	var item content.ContentBannerItem
	err := r.db.WithContext(ctx).First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *contentBannerItemRepository) GetByIDWithGroup(ctx context.Context, id uint) (*content.ContentBannerItem, error) {
	var item content.ContentBannerItem
	err := r.db.WithContext(ctx).
		Preload("Group").
		Preload("Article").
		First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *contentBannerItemRepository) List(ctx context.Context, query *ContentBannerItemQuery) ([]*content.ContentBannerItem, int64, error) {
	db := r.db.WithContext(ctx).Model(&content.ContentBannerItem{})

	if query.GroupID > 0 {
		db = db.Where("group_id = ?", query.GroupID)
	}
	if query.Title != "" {
		db = db.Where("title LIKE ?", "%"+query.Title+"%")
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.StartTime != "" {
		db = db.Where("start_time >= ?", query.StartTime)
	}
	if query.EndTime != "" {
		db = db.Where("end_time <= ?", query.EndTime)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*content.ContentBannerItem
	if query.Size <= 0 {
		query.Size = entity.DefaultPageSize
	}

	err := db.Preload("Group").
		Preload("Article").
		Order("sort ASC, created_at DESC").
		Scopes(pagination.Paginate(query.Current, query.Size)).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *contentBannerItemRepository) GetByGroupID(ctx context.Context, groupID uint) ([]*content.ContentBannerItem, error) {
	var items []*content.ContentBannerItem
	now := time.Now()
	err := r.db.WithContext(ctx).
		Where("group_id = ? AND status = ?", groupID, entity.StatusEnabled).
		Where("(start_time IS NULL OR start_time <= ?) AND (end_time IS NULL OR end_time >= ?)", now, now).
		Order("sort ASC, created_at DESC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *contentBannerItemRepository) CountByGroupID(ctx context.Context, groupID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&content.ContentBannerItem{}).
		Where("group_id = ? AND status = ?", groupID, entity.StatusEnabled).
		Count(&count).Error
	return count, err
}

func (r *contentBannerItemRepository) IncrementViewCount(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&content.ContentBannerItem{}).
		Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

func (r *contentBannerItemRepository) IncrementClickCount(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&content.ContentBannerItem{}).
		Where("id = ?", id).
		UpdateColumn("click_count", gorm.Expr("click_count + 1")).Error
}
