package content

import (
	"context"
	"time"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	content "NetyAdmin/internal/domain/entity/content"
	"NetyAdmin/internal/pkg/database"
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

// getDB 从 context 中获取事务 DB，若不存在则回退到默认 db。
func (r *contentBannerItemRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *contentBannerItemRepository) Create(ctx context.Context, item *content.ContentBannerItem) error {
	return r.getDB(ctx).Create(item).Error
}

func (r *contentBannerItemRepository) Update(ctx context.Context, item *content.ContentBannerItem) error {
	return r.getDB(ctx).Save(item).Error
}

func (r *contentBannerItemRepository) Delete(ctx context.Context, id uint) error {
	// ContentBannerItem 为硬删除实体，使用 Unscoped 跳过软删除过滤
	return r.getDB(ctx).Unscoped().Delete(&content.ContentBannerItem{}, id).Error
}

func (r *contentBannerItemRepository) GetByID(ctx context.Context, id uint) (*content.ContentBannerItem, error) {
	var item content.ContentBannerItem
	err := r.getDB(ctx).First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *contentBannerItemRepository) GetByIDWithGroup(ctx context.Context, id uint) (*content.ContentBannerItem, error) {
	var item content.ContentBannerItem
	err := r.getDB(ctx).
		Preload("Group").
		Preload("Article").
		First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *contentBannerItemRepository) List(ctx context.Context, query *ContentBannerItemQuery) ([]*content.ContentBannerItem, int64, error) {
	if query.Current <= 0 {
		query.Current = 1
	}
	if query.Size <= 0 {
		query.Size = entity.DefaultPageSize
	}

	db := r.getDB(ctx).Model(&content.ContentBannerItem{})

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
	err := r.getDB(ctx).
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
	err := r.getDB(ctx).Model(&content.ContentBannerItem{}).
		Where("group_id = ?", groupID).
		Count(&count).Error
	return count, err
}

func (r *contentBannerItemRepository) IncrementViewCount(ctx context.Context, id uint) error {
	return r.getDB(ctx).Model(&content.ContentBannerItem{}).
		Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

func (r *contentBannerItemRepository) IncrementClickCount(ctx context.Context, id uint) error {
	return r.getDB(ctx).Model(&content.ContentBannerItem{}).
		Where("id = ?", id).
		UpdateColumn("click_count", gorm.Expr("click_count + 1")).Error
}
