package system

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/pagination"
)

type APIRepository interface {
	Create(ctx context.Context, api *systemEntity.API) error
	Update(ctx context.Context, api *systemEntity.API) error
	Delete(ctx context.Context, id uint) error
	// ClearRoleApis 清理 admin_role_apis 关联表中指定 api 的全部角色关联。
	// 用于 api Delete 的 TM 事务编排，支持 context 传播事务。
	ClearRoleApis(ctx context.Context, apiID uint) error
	// ClearRoleApisByMenuID 清理 admin_role_apis 关联表中指定 menu 下全部 api 的角色关联。
	// 用于 menu Delete 的级联清理，通过子查询按 menu_id 定位 api 再删除其角色关联，支持 context 传播事务。
	ClearRoleApisByMenuID(ctx context.Context, menuID uint) error
	// DeleteByMenuID 按 menu_id 硬删除所有归属该 menu 的 api（Unscoped 绕过软删除）。
	// 用于 menu Delete 的级联清理，支持 context 传播事务。
	DeleteByMenuID(ctx context.Context, menuID uint) error
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

// getDB 取当前 context 下的 *gorm.DB：若 ctx 中存在事务句柄则复用事务，否则回退到 r.db。
func (r *apiRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *apiRepository) Create(ctx context.Context, api *systemEntity.API) error {
	return r.getDB(ctx).Create(api).Error
}

func (r *apiRepository) Update(ctx context.Context, api *systemEntity.API) error {
	return r.getDB(ctx).Save(api).Error
}

func (r *apiRepository) Delete(ctx context.Context, id uint) error {
	return r.getDB(ctx).Unscoped().Delete(&systemEntity.API{}, id).Error
}

// ClearRoleApis 清理 admin_role_apis 关联表中指定 api 的全部角色关联。
// 用于 api Delete 的 TM 事务编排，支持 context 传播事务。
// 使用原生 SQL 直接删除关联表行，避免加载实体再 Association().Clear() 的额外查询开销。
func (r *apiRepository) ClearRoleApis(ctx context.Context, apiID uint) error {
	return r.getDB(ctx).
		Table("admin_role_apis").
		Where("admin_api_id = ?", apiID).
		Delete(nil).Error
}

// ClearRoleApisByMenuID 清理 admin_role_apis 关联表中指定 menu 下全部 api 的角色关联。
// 用于 menu Delete 的级联清理：通过子查询按 menu_id 定位 admin_api.id，再删除其角色关联行，
// 避免「api 行已硬删除但 admin_role_apis 仍残留指向已删 api 的孤儿引用」。
// 支持 context 传播事务。
func (r *apiRepository) ClearRoleApisByMenuID(ctx context.Context, menuID uint) error {
	return r.getDB(ctx).
		Table("admin_role_apis").
		Where("admin_api_id IN (SELECT id FROM admin_api WHERE menu_id = ?)", menuID).
		Delete(nil).Error
}

// DeleteByMenuID 按 menu_id 硬删除所有归属该 menu 的 api（Unscoped 绕过软删除）。
// 用于 menu Delete 的级联清理：避免 api 残留为孤儿行（menu_id 指向已删 menu）。
// 支持 context 传播事务。调用方应在调用本方法前先调用 ClearRoleApisByMenuID 清理角色关联。
func (r *apiRepository) DeleteByMenuID(ctx context.Context, menuID uint) error {
	return r.getDB(ctx).Where("menu_id = ?", menuID).Unscoped().Delete(&systemEntity.API{}).Error
}

func (r *apiRepository) GetByID(ctx context.Context, id uint) (*systemEntity.API, error) {
	var api systemEntity.API
	err := r.getDB(ctx).
		Preload("Menu").
		First(&api, id).Error
	if err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *apiRepository) GetByMethodAndPath(ctx context.Context, method, path string) (*systemEntity.API, error) {
	var api systemEntity.API
	err := r.getDB(ctx).
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

	db := r.getDB(ctx).Model(&systemEntity.API{}).Preload("Menu")

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
		query.Size = entity.DefaultPageSize
	}

	if err := db.Order("id DESC").Scopes(pagination.Paginate(query.Current, query.Size)).Find(&apis).Error; err != nil {
		return nil, 0, err
	}

	return apis, total, nil
}

func (r *apiRepository) GetByMenuID(ctx context.Context, menuID uint) ([]*systemEntity.API, error) {
	var apis []*systemEntity.API
	err := r.getDB(ctx).Where("menu_id = ?", menuID).Find(&apis).Error
	return apis, err
}

func (r *apiRepository) GetByRoleID(ctx context.Context, roleID uint) ([]*systemEntity.API, error) {
	var apis []*systemEntity.API
	err := r.getDB(ctx).
		Joins("JOIN admin_role_apis ON admin_api.id = admin_role_apis.admin_api_id").
		Where("admin_role_apis.admin_role_id = ?", roleID).
		Find(&apis).Error
	return apis, err
}

func (r *apiRepository) GetAll(ctx context.Context) ([]*systemEntity.API, error) {
	var apis []*systemEntity.API
	err := r.getDB(ctx).Find(&apis).Error
	return apis, err
}

func (r *apiRepository) ExistsByMethodAndPath(ctx context.Context, method, path string, excludeID ...uint) (bool, error) {
	var count int64
	db := r.getDB(ctx).Model(&systemEntity.API{}).Where("method = ? AND path = ?", method, path)
	if len(excludeID) > 0 {
		db = db.Where("id != ?", excludeID[0])
	}
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
