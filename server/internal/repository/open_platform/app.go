package open_platform

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity/open_platform"
)

type AppRepository interface {
	Create(ctx context.Context, app *open_platform.App) error
	Update(ctx context.Context, app *open_platform.App) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*open_platform.App, error)
	GetByKey(ctx context.Context, appKey string) (*open_platform.App, error)
	List(ctx context.Context, query *AppRepoQuery) ([]*open_platform.App, int64, error)

	// Scopes
	GetAppScopes(ctx context.Context, appID string) ([]string, error)
	UpdateAppScopes(ctx context.Context, appID string, scopes []string) error

	// Scope Groups
	ListScopeGroups(ctx context.Context) ([]*open_platform.AppScopeGroup, error)
	CreateScopeGroup(ctx context.Context, group *open_platform.AppScopeGroup) error
	UpdateScopeGroup(ctx context.Context, group *open_platform.AppScopeGroup) error
	DeleteScopeGroup(ctx context.Context, id uint64) error
	GetScopeGroupByID(ctx context.Context, id uint64) (*open_platform.AppScopeGroup, error)
}

type AppRepoQuery struct {
	Page     int
	PageSize int
	Name     string
	AppKey   string
	Type     *int
	Status   *int
}

type appRepository struct {
	db *gorm.DB
}

func NewAppRepository(db *gorm.DB) AppRepository {
	return &appRepository{db: db}
}

func (r *appRepository) Create(ctx context.Context, app *open_platform.App) error {
	return r.db.WithContext(ctx).Create(app).Error
}

func (r *appRepository) Update(ctx context.Context, app *open_platform.App) error {
	return r.db.WithContext(ctx).Save(app).Error
}

func (r *appRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&open_platform.App{}, "id = ?", id).Error
}

func (r *appRepository) GetByID(ctx context.Context, id string) (*open_platform.App, error) {
	var app open_platform.App
	err := r.db.WithContext(ctx).First(&app, "id = ?", id).Error
	return &app, err
}

func (r *appRepository) GetByKey(ctx context.Context, appKey string) (*open_platform.App, error) {
	var app open_platform.App
	err := r.db.WithContext(ctx).Where("app_key = ? AND status = ?", appKey, open_platform.AppStatusEnabled).First(&app).Error
	return &app, err
}

func (r *appRepository) List(ctx context.Context, query *AppRepoQuery) ([]*open_platform.App, int64, error) {
	var list []*open_platform.App
	var total int64
	db := r.db.WithContext(ctx).Model(&open_platform.App{})

	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.AppKey != "" {
		db = db.Where("app_key = ?", query.AppKey)
	}
	if query.Type != nil {
		db = db.Where("type = ?", *query.Type)
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

	err := db.Order("created_at DESC").Find(&list).Error
	return list, total, err
}

func (r *appRepository) GetAppScopes(ctx context.Context, appID string) ([]string, error) {
	var scopes []string
	err := r.db.WithContext(ctx).Model(&open_platform.AppScope{}).
		Where("app_id = ?", appID).
		Pluck("scope", &scopes).Error
	return scopes, err
}

func (r *appRepository) UpdateAppScopes(ctx context.Context, appID string, scopes []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete old
		if err := tx.Where("app_id = ?", appID).Delete(&open_platform.AppScope{}).Error; err != nil {
			return err
		}
		// Insert new
		if len(scopes) > 0 {
			var appScopes []open_platform.AppScope
			for _, s := range scopes {
				appScopes = append(appScopes, open_platform.AppScope{
					AppID: appID,
					Scope: s,
				})
			}
			if err := tx.Create(&appScopes).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *appRepository) ListScopeGroups(ctx context.Context) ([]*open_platform.AppScopeGroup, error) {
	var list []*open_platform.AppScopeGroup
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *appRepository) CreateScopeGroup(ctx context.Context, group *open_platform.AppScopeGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *appRepository) UpdateScopeGroup(ctx context.Context, group *open_platform.AppScopeGroup) error {
	return r.db.WithContext(ctx).Save(group).Error
}

func (r *appRepository) DeleteScopeGroup(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&open_platform.AppScopeGroup{}, id).Error
}

func (r *appRepository) GetScopeGroupByID(ctx context.Context, id uint64) (*open_platform.AppScopeGroup, error) {
	var group open_platform.AppScopeGroup
	err := r.db.WithContext(ctx).First(&group, id).Error
	return &group, err
}
