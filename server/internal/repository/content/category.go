package content

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	content "NetyAdmin/internal/domain/entity/content"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/pagination"
)

type ContentCategoryRepository interface {
	Create(ctx context.Context, category *content.ContentCategory) error
	Update(ctx context.Context, category *content.ContentCategory) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*content.ContentCategory, error)
	List(ctx context.Context, query *ContentCategoryQuery) ([]*content.ContentCategory, int64, error)
	GetTree(ctx context.Context) ([]*content.ContentCategory, error)
	GetByParentID(ctx context.Context, parentID uint) ([]*content.ContentCategory, error)
	ExistsByCode(ctx context.Context, code string, excludeID ...uint) (bool, error)
	HasChildren(ctx context.Context, id uint) (bool, error)
	HasArticles(ctx context.Context, id uint) (bool, error)
	// CountChildren 统计指定父分类下的子分类数量（用于删除分类前的前置校验）
	CountChildren(ctx context.Context, parentID uint) (int64, error)
}

type ContentCategoryQuery struct {
	Name    string
	Status  string
	Current int
	Size    int
}

type contentCategoryRepository struct {
	db *gorm.DB
}

func NewContentCategoryRepository(db *gorm.DB) ContentCategoryRepository {
	return &contentCategoryRepository{db: db}
}

// getDB 从 context 中获取事务 DB，若不存在则回退到默认 db。
func (r *contentCategoryRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *contentCategoryRepository) Create(ctx context.Context, category *content.ContentCategory) error {
	return r.getDB(ctx).Create(category).Error
}

func (r *contentCategoryRepository) Update(ctx context.Context, category *content.ContentCategory) error {
	return r.getDB(ctx).Save(category).Error
}

func (r *contentCategoryRepository) Delete(ctx context.Context, id uint) error {
	// ContentCategory 为硬删除实体，使用 Unscoped 跳过软删除过滤
	return r.getDB(ctx).Unscoped().Delete(&content.ContentCategory{}, id).Error
}

func (r *contentCategoryRepository) GetByID(ctx context.Context, id uint) (*content.ContentCategory, error) {
	var category content.ContentCategory
	err := r.getDB(ctx).First(&category, id).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *contentCategoryRepository) List(ctx context.Context, query *ContentCategoryQuery) ([]*content.ContentCategory, int64, error) {
	if query.Current <= 0 {
		query.Current = 1
	}
	if query.Size <= 0 {
		query.Size = entity.DefaultPageSize
	}

	db := r.getDB(ctx).Model(&content.ContentCategory{})

	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var categories []*content.ContentCategory

	err := db.Order("sort ASC, created_at DESC").
		Scopes(pagination.Paginate(query.Current, query.Size)).
		Find(&categories).Error
	if err != nil {
		return nil, 0, err
	}

	return categories, total, nil
}

func (r *contentCategoryRepository) GetTree(ctx context.Context) ([]*content.ContentCategory, error) {
	var categories []*content.ContentCategory
	err := r.getDB(ctx).
		Where("status = ?", entity.StatusEnabled).
		Order("sort ASC, created_at DESC").
		Find(&categories).Error
	if err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *contentCategoryRepository) GetByParentID(ctx context.Context, parentID uint) ([]*content.ContentCategory, error) {
	var categories []*content.ContentCategory
	err := r.getDB(ctx).
		Where("parent_id = ?", parentID).
		Order("sort ASC, created_at DESC").
		Find(&categories).Error
	if err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *contentCategoryRepository) ExistsByCode(ctx context.Context, code string, excludeID ...uint) (bool, error) {
	if code == "" {
		return false, nil
	}
	db := r.getDB(ctx).Model(&content.ContentCategory{}).Where("code = ?", code)
	if len(excludeID) > 0 {
		db = db.Where("id != ?", excludeID[0])
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *contentCategoryRepository) HasChildren(ctx context.Context, id uint) (bool, error) {
	var count int64
	if err := r.getDB(ctx).Model(&content.ContentCategory{}).
		Where("parent_id = ?", id).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *contentCategoryRepository) HasArticles(ctx context.Context, id uint) (bool, error) {
	var count int64
	if err := r.getDB(ctx).Model(&content.ContentArticle{}).
		Where("category_id = ?", id).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// CountChildren 统计指定父分类下的子分类数量（用于删除分类前的前置校验，包含所有状态的子分类）
func (r *contentCategoryRepository) CountChildren(ctx context.Context, parentID uint) (int64, error) {
	var count int64
	if err := r.getDB(ctx).Model(&content.ContentCategory{}).
		Where("parent_id = ?", parentID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
