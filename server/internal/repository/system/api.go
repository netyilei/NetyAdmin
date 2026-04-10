package system

import (
	"context"

	"gorm.io/gorm"

	systemEntity "NetyAdmin/internal/domain/entity/system"
)

type APIRepository interface {
	Create(ctx context.Context, api *systemEntity.API) error
	Update(ctx context.Context, api *systemEntity.API) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*systemEntity.API, error)
	GetByMethodAndPath(ctx context.Context, method, path string) (*systemEntity.API, error)
	List(ctx context.Context, query *APIRepoQuery) ([]*systemEntity.API, int64, error)
	GetByMenuID(ctx context.Context, menuID uint) ([]*systemEntity.API, error)
	GetByRoleID(ctx context.Context, roleID uint) ([]*systemEntity.API, error)
	GetAll(ctx context.Context) ([]*systemEntity.API, error)
	ExistsByMethodAndPath(ctx context.Context, method, path string, excludeID ...uint) (bool, error)
}

type APIRepoQuery struct {
	Name    string
	Method  string
	Path    string
	MenuID  *uint
	Auth    *string
	Current int
	Size    int
}

type apiRepository struct {
	db *gorm.DB
}

func NewAPIRepository(db *gorm.DB) APIRepository {
	return &apiRepository{db: db}
}

func (r *apiRepository) Create(ctx context.Context, api *systemEntity.API) error {
	return r.db.WithContext(ctx).Create(api).Error
}

func (r *apiRepository) Update(ctx context.Context, api *systemEntity.API) error {
	return r.db.WithContext(ctx).Save(api).Error
}

func (r *apiRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&systemEntity.API{}, id).Error
}

func (r *apiRepository) GetByID(ctx context.Context, id uint) (*systemEntity.API, error) {
	var api systemEntity.API
	err := r.db.WithContext(ctx).
		Preload("Menu").
		First(&api, id).Error
	if err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *apiRepository) GetByMethodAndPath(ctx context.Context, method, path string) (*systemEntity.API, error) {
	var api systemEntity.API
	err := r.db.WithContext(ctx).
		Where("method = ? AND path = ?", method, path).
		First(&api).Error
	if err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *apiRepository) List(ctx context.Context, query *APIRepoQuery) ([]*systemEntity.API, int64, error) {
	var apis []*systemEntity.API
	var total int64

	db := r.db.WithContext(ctx).Model(&systemEntity.API{}).Preload("Menu")

	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Method != "" {
		db = db.Where("method = ?", query.Method)
	}
	if query.Path != "" {
		db = db.Where("path LIKE ?", "%"+query.Path+"%")
	}
	if query.MenuID != nil {
		db = db.Where("menu_id = ?", *query.MenuID)
	}
	if query.Auth != nil && *query.Auth != "" {
		db = db.Where("auth = ?", *query.Auth)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if query.Current <= 0 {
		query.Current = 1
	}
	if query.Size <= 0 {
		query.Size = 10
	}

	offset := (query.Current - 1) * query.Size
	if err := db.Order("id DESC").Offset(offset).Limit(query.Size).Find(&apis).Error; err != nil {
		return nil, 0, err
	}

	return apis, total, nil
}

func (r *apiRepository) GetByMenuID(ctx context.Context, menuID uint) ([]*systemEntity.API, error) {
	var apis []*systemEntity.API
	err := r.db.WithContext(ctx).Where("menu_id = ?", menuID).Find(&apis).Error
	return apis, err
}

func (r *apiRepository) GetByRoleID(ctx context.Context, roleID uint) ([]*systemEntity.API, error) {
	var apis []*systemEntity.API
	err := r.db.WithContext(ctx).
		Joins("JOIN admin_role_apis ON admin_api.id = admin_role_apis.admin_api_id").
		Where("admin_role_apis.admin_role_id = ?", roleID).
		Find(&apis).Error
	return apis, err
}

func (r *apiRepository) GetAll(ctx context.Context) ([]*systemEntity.API, error) {
	var apis []*systemEntity.API
	err := r.db.WithContext(ctx).Find(&apis).Error
	return apis, err
}

func (r *apiRepository) ExistsByMethodAndPath(ctx context.Context, method, path string, excludeID ...uint) (bool, error) {
	var count int64
	db := r.db.WithContext(ctx).Model(&systemEntity.API{}).Where("method = ? AND path = ?", method, path)
	if len(excludeID) > 0 {
		db = db.Where("id != ?", excludeID[0])
	}
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
