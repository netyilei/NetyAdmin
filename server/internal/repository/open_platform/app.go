package open_platform

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	"NetyAdmin/internal/domain/entity/open_platform"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/pagination"
)

type AppRepository interface {
	Create(ctx context.Context, app *open_platform.App) error
	Update(ctx context.Context, app *open_platform.App) error
	UpdateSecret(ctx context.Context, id string, encryptedSecret string) error
	Delete(ctx context.Context, id string) error
	// DeleteWithCascade 级联删除 app 及其关联数据（sys_app_scopes + sys_open_platform_logs）。
	// 本方法不自管事务，依赖 service 层通过 TM 已开启的事务 DB（通过 ctx 注入）；
	// service 调用前必须 tm.Begin 拿到 txCtx 后再传入本方法，由 service 负责 Commit/Rollback。
	DeleteWithCascade(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*open_platform.App, error)
	GetByKey(ctx context.Context, appKey string) (*open_platform.App, error)
	List(ctx context.Context, query *AppRepoQuery) ([]*open_platform.App, int64, error)

	// Scopes
	GetAppScopes(ctx context.Context, appID string) ([]string, error)
	GetAppScopesByAppIDs(ctx context.Context, appIDs []string) (map[string][]string, error)
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
	Status   *int
}

type appRepository struct {
	db *gorm.DB
}

func NewAppRepository(db *gorm.DB) AppRepository {
	return &appRepository{db: db}
}

// getDB 根据 context 中是否携带事务，返回事务内的 *gorm.DB 或回退到 r.db
func (r *appRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *appRepository) Create(ctx context.Context, app *open_platform.App) error {
	return r.getDB(ctx).Create(app).Error
}

func (r *appRepository) Update(ctx context.Context, app *open_platform.App) error {
	return r.getDB(ctx).
		Model(&open_platform.App{}).
		Where("id = ?", app.ID).
		Updates(map[string]any{
			"name":               app.Name,
			"status":             app.Status,
			"ip_filter_enabled":  app.IPFilterEnabled,
			"rate_limit_enabled": app.RateLimitEnabled,
			"remark":             app.Remark,
			"quota_config":       app.QuotaConfig,
			"cache_ttl":          app.CacheTTL,
			"storage_id":         app.StorageID,
		}).Error
}

func (r *appRepository) UpdateSecret(ctx context.Context, id string, encryptedSecret string) error {
	return r.getDB(ctx).Model(&open_platform.App{}).Where("id = ?", id).Update("app_secret", encryptedSecret).Error
}

func (r *appRepository) Delete(ctx context.Context, id string) error {
	return r.getDB(ctx).Unscoped().Delete(&open_platform.App{}, "id = ?", id).Error
}

// DeleteWithCascade 级联删除 app 及其关联数据。
//
// 关联表：
//   - sys_app_scopes（AppScope）：app_id 字段
//   - sys_open_platform_logs（OpenPlatformLog）：app_id 字段
//
// 注：nonce 防重放使用 Redis 缓存（cache.KeyAppNonce），不落库，无需在此清理。
//
// 本方法遵循「Repository 不自管事务」红线（RULES.md §二）：不调用 db.Transaction()，
// 而是直接复用 service 层通过 ctx 注入的事务 DB（getDB(ctx) 返回值）。
// service 调用方负责通过 TM Begin/Commit/Rollback 控制事务边界，保证原子性。
//
// 删除顺序：先删关联表（避免孤儿行），最后删主表。
func (r *appRepository) DeleteWithCascade(ctx context.Context, id string) error {
	db := r.getDB(ctx)
	// 1. 删除 app 的 scope 关联（sys_app_scopes）
	if err := db.Where("app_id = ?", id).Delete(&open_platform.AppScope{}).Error; err != nil {
		return err
	}
	// 2. 删除 app 的开放平台调用日志（sys_open_platform_logs）
	if err := db.Where("app_id = ?", id).Delete(&open_platform.OpenPlatformLog{}).Error; err != nil {
		return err
	}
	// 3. 最后删除 app 主表（sys_apps，硬删除）
	return db.Unscoped().Delete(&open_platform.App{}, "id = ?", id).Error
}

func (r *appRepository) GetByID(ctx context.Context, id string) (*open_platform.App, error) {
	var app open_platform.App
	if err := r.getDB(ctx).First(&app, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *appRepository) GetByKey(ctx context.Context, appKey string) (*open_platform.App, error) {
	var app open_platform.App
	if err := r.getDB(ctx).Where("app_key = ?", appKey).First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *appRepository) List(ctx context.Context, query *AppRepoQuery) ([]*open_platform.App, int64, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = entity.DefaultPageSize
	}

	var list []*open_platform.App
	var total int64
	db := r.getDB(ctx).Model(&open_platform.App{})

	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.AppKey != "" {
		db = db.Where("app_key = ?", query.AppKey)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := db.Order("created_at DESC").Scopes(pagination.Paginate(query.Page, query.PageSize)).Find(&list).Error
	return list, total, err
}

func (r *appRepository) GetAppScopes(ctx context.Context, appID string) ([]string, error) {
	var scopes []string
	err := r.getDB(ctx).Model(&open_platform.AppScope{}).
		Where("app_id = ?", appID).
		Pluck("scope", &scopes).Error
	return scopes, err
}

func (r *appRepository) GetAppScopesByAppIDs(ctx context.Context, appIDs []string) (map[string][]string, error) {
	if len(appIDs) == 0 {
		return make(map[string][]string), nil
	}
	var results []struct {
		AppID string
		Scope string
	}
	err := r.getDB(ctx).Model(&open_platform.AppScope{}).
		Select("app_id, scope").
		Where("app_id IN ?", appIDs).
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	scopesMap := make(map[string][]string, len(appIDs))
	for _, r := range results {
		scopesMap[r.AppID] = append(scopesMap[r.AppID], r.Scope)
	}
	return scopesMap, nil
}

func (r *appRepository) UpdateAppScopes(ctx context.Context, appID string, scopes []string) error {
	if err := r.getDB(ctx).Where("app_id = ?", appID).Delete(&open_platform.AppScope{}).Error; err != nil {
		return err
	}
	if len(scopes) > 0 {
		var appScopes []open_platform.AppScope
		for _, s := range scopes {
			appScopes = append(appScopes, open_platform.AppScope{
				AppID: appID,
				Scope: s,
			})
		}
		if err := r.getDB(ctx).Create(&appScopes).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *appRepository) ListScopeGroups(ctx context.Context) ([]*open_platform.AppScopeGroup, error) {
	var list []*open_platform.AppScopeGroup
	err := r.getDB(ctx).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *appRepository) CreateScopeGroup(ctx context.Context, group *open_platform.AppScopeGroup) error {
	return r.getDB(ctx).Create(group).Error
}

func (r *appRepository) UpdateScopeGroup(ctx context.Context, group *open_platform.AppScopeGroup) error {
	return r.getDB(ctx).
		Model(&open_platform.AppScopeGroup{}).
		Where("id = ?", group.ID).
		Updates(map[string]any{
			"name":        group.Name,
			"description": group.Description,
			"status":      group.Status,
		}).Error
}

func (r *appRepository) DeleteScopeGroup(ctx context.Context, id uint64) error {
	return r.getDB(ctx).Delete(&open_platform.AppScopeGroup{}, id).Error
}

func (r *appRepository) GetScopeGroupByID(ctx context.Context, id uint64) (*open_platform.AppScopeGroup, error) {
	var group open_platform.AppScopeGroup
	if err := r.getDB(ctx).First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}
