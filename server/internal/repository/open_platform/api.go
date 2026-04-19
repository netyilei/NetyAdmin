package open_platform

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity/open_platform"
)

type OpenApiRepository interface {
	Create(ctx context.Context, api *open_platform.OpenApi) error
	Update(ctx context.Context, api *open_platform.OpenApi) error
	Delete(ctx context.Context, id uint64) error
	GetByID(ctx context.Context, id uint64) (*open_platform.OpenApi, error)
	List(ctx context.Context, query *OpenApiRepoQuery) ([]*open_platform.OpenApi, int64, error)
	ListAll(ctx context.Context) ([]*open_platform.OpenApi, error)

	GetScopeApis(ctx context.Context, scopeID uint64) ([]*open_platform.OpenApi, error)
	UpdateScopeApis(ctx context.Context, scopeID uint64, apiIDs []uint64) error
	GetApisByScopeIDs(ctx context.Context, scopeIDs []uint64) ([]*open_platform.OpenApi, error)
}

type OpenApiRepoQuery struct {
	Page     int
	PageSize int
	Method   string
	Path     string
	Name     string
	Group    string
	Status   *int
}

type openApiRepository struct {
	db *gorm.DB
}

func NewOpenApiRepository(db *gorm.DB) OpenApiRepository {
	return &openApiRepository{db: db}
}

func (r *openApiRepository) Create(ctx context.Context, api *open_platform.OpenApi) error {
	return r.db.WithContext(ctx).Create(api).Error
}

func (r *openApiRepository) Update(ctx context.Context, api *open_platform.OpenApi) error {
	return r.db.WithContext(ctx).
		Model(&open_platform.OpenApi{}).
		Where("id = ?", api.ID).
		Updates(map[string]any{
			"method":      api.Method,
			"path":        api.Path,
			"name":        api.Name,
			"group_name":  api.Group,
			"description": api.Description,
			"status":      api.Status,
		}).Error
}

func (r *openApiRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&open_platform.OpenApi{}, id).Error
}

func (r *openApiRepository) GetByID(ctx context.Context, id uint64) (*open_platform.OpenApi, error) {
	var api open_platform.OpenApi
	err := r.db.WithContext(ctx).First(&api, id).Error
	return &api, err
}

func (r *openApiRepository) List(ctx context.Context, query *OpenApiRepoQuery) ([]*open_platform.OpenApi, int64, error) {
	var list []*open_platform.OpenApi
	var total int64
	db := r.db.WithContext(ctx).Model(&open_platform.OpenApi{})

	if query.Method != "" {
		db = db.Where("method = ?", query.Method)
	}
	if query.Path != "" {
		db = db.Where("path LIKE ?", "%"+query.Path+"%")
	}
	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Group != "" {
		db = db.Where("group_name = ?", query.Group)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if query.Page > 0 && query.PageSize > 0 {
		db = db.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize)
	}

	err := db.Order("id ASC").Find(&list).Error
	return list, total, err
}

func (r *openApiRepository) ListAll(ctx context.Context) ([]*open_platform.OpenApi, error) {
	var list []*open_platform.OpenApi
	err := r.db.WithContext(ctx).Where("status = ?", 1).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *openApiRepository) GetScopeApis(ctx context.Context, scopeID uint64) ([]*open_platform.OpenApi, error) {
	var list []*open_platform.OpenApi
	err := r.db.WithContext(ctx).
		Where("id IN (?)", r.db.Table("sys_scope_apis").
			Select("api_id").
			Where("scope_id = ? AND deleted_at = 0", scopeID)).
		Order("id ASC").
		Find(&list).Error
	return list, err
}

func (r *openApiRepository) UpdateScopeApis(ctx context.Context, scopeID uint64, apiIDs []uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("scope_id = ?", scopeID).Delete(&open_platform.ScopeApi{}).Error; err != nil {
			return err
		}
		if len(apiIDs) > 0 {
			var scopeApis []open_platform.ScopeApi
			for _, apiID := range apiIDs {
				scopeApis = append(scopeApis, open_platform.ScopeApi{
					ScopeID: scopeID,
					ApiID:   apiID,
				})
			}
			if err := tx.Create(&scopeApis).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *openApiRepository) GetApisByScopeIDs(ctx context.Context, scopeIDs []uint64) ([]*open_platform.OpenApi, error) {
	if len(scopeIDs) == 0 {
		return nil, nil
	}
	var list []*open_platform.OpenApi
	err := r.db.WithContext(ctx).
		Where("id IN (?)", r.db.Table("sys_scope_apis").
			Select("api_id").
			Where("scope_id IN ? AND deleted_at = 0", scopeIDs)).
		Order("id ASC").
		Find(&list).Error
	return list, err
}
