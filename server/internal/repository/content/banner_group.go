package content

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	content "NetyAdmin/internal/domain/entity/content"
	"NetyAdmin/internal/pkg/pagination"
)

type ContentBannerGroupRepository interface {
	Create(ctx context.Context, group *content.ContentBannerGroup) error
	Update(ctx context.Context, group *content.ContentBannerGroup) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*content.ContentBannerGroup, error)
	GetByIDWithBanners(ctx context.Context, id uint) (*content.ContentBannerGroup, error)
	List(ctx context.Context, query *ContentBannerGroupQuery) ([]*content.ContentBannerGroup, int64, error)
	GetAll(ctx context.Context) ([]*content.ContentBannerGroup, error)
	GetByCode(ctx context.Context, code string) (*content.ContentBannerGroup, error)
	ExistsByCode(ctx context.Context, code string, excludeID ...uint) (bool, error)
	HasBanners(ctx context.Context, id uint) (bool, error)
}

type ContentBannerGroupQuery struct {
	Name        string
	Code        string
	Description string
	Position    string
	Status      string
	Current     int
	Size        int
}

type contentBannerGroupRepository struct {
	db *gorm.DB
}

func NewContentBannerGroupRepository(db *gorm.DB) ContentBannerGroupRepository {
	return &contentBannerGroupRepository{db: db}
}

func (r *contentBannerGroupRepository) Create(ctx context.Context, group *content.ContentBannerGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *contentBannerGroupRepository) Update(ctx context.Context, group *content.ContentBannerGroup) error {
	return r.db.WithContext(ctx).Save(group).Error
}

func (r *contentBannerGroupRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&content.ContentBannerGroup{}, id).Error
}

func (r *contentBannerGroupRepository) GetByID(ctx context.Context, id uint) (*content.ContentBannerGroup, error) {
	var group content.ContentBannerGroup
	err := r.db.WithContext(ctx).First(&group, id).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *contentBannerGroupRepository) GetByIDWithBanners(ctx context.Context, id uint) (*content.ContentBannerGroup, error) {
	var group content.ContentBannerGroup
	err := r.db.WithContext(ctx).
		Preload("Banners", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort ASC, created_at DESC")
		}).
		First(&group, id).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *contentBannerGroupRepository) List(ctx context.Context, query *ContentBannerGroupQuery) ([]*content.ContentBannerGroup, int64, error) {
	db := r.db.WithContext(ctx).Model(&content.ContentBannerGroup{})

	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Code != "" {
		db = db.Where("code = ?", query.Code)
	}
	if query.Description != "" {
		db = db.Where("description LIKE ?", "%"+query.Description+"%")
	}
	if query.Position != "" {
		db = db.Where("position = ?", query.Position)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var groups []*content.ContentBannerGroup
	if query.Size <= 0 {
		query.Size = entity.DefaultPageSize
	}

	err := db.Order("sort ASC, created_at DESC").
		Scopes(pagination.Paginate(query.Current, query.Size)).
		Find(&groups).Error
	if err != nil {
		return nil, 0, err
	}

	return groups, total, nil
}

func (r *contentBannerGroupRepository) GetAll(ctx context.Context) ([]*content.ContentBannerGroup, error) {
	var groups []*content.ContentBannerGroup
	err := r.db.WithContext(ctx).
		Where("status = ?", entity.StatusEnabled).
		Order("sort ASC, created_at DESC").
		Find(&groups).Error
	if err != nil {
		return nil, err
	}
	return groups, nil
}

func (r *contentBannerGroupRepository) GetByCode(ctx context.Context, code string) (*content.ContentBannerGroup, error) {
	var group content.ContentBannerGroup
	err := r.db.WithContext(ctx).
		Where("code = ? AND status = ?", code, entity.StatusEnabled).
		Preload("Banners", func(db *gorm.DB) *gorm.DB {
			return db.Where("status = ?", entity.StatusEnabled).Order("sort ASC")
		}).
		First(&group).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *contentBannerGroupRepository) ExistsByCode(ctx context.Context, code string, excludeID ...uint) (bool, error) {
	db := r.db.WithContext(ctx).Model(&content.ContentBannerGroup{}).Where("code = ?", code)
	if len(excludeID) > 0 {
		db = db.Where("id != ?", excludeID[0])
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *contentBannerGroupRepository) HasBanners(ctx context.Context, id uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&content.ContentBannerItem{}).
		Where("group_id = ?", id).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
