package system

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/pagination"
)

type RoleRepository interface {
	Create(ctx context.Context, role *systemEntity.Role) error
	Update(ctx context.Context, role *systemEntity.Role) error
	// Delete 硬删除 role 主数据（需 Unscoped 绕过软删除）。
	// 注意：本方法仅删除主数据，不再清理关联表（清理职责已上移到 Service 层 TM 事务）。
	// Service 层 Delete 应在 TM 事务内先调 ClearUserRoles + ClearPermissions 再调 Delete，保证原子性。
	Delete(ctx context.Context, id uint) error
	// ClearUserRoles 清理 admin_user_roles 关联表中指定 role 的全部管理员关联。
	// 用于 role Delete 的 TM 事务编排，支持 context 传播事务。
	ClearUserRoles(ctx context.Context, roleID uint) error
	// ClearPermissions 清理 role 的全部权限关联表（admin_role_menus/admin_role_buttons/admin_role_apis）。
	// 用于 role Delete 的 TM 事务编排，支持 context 传播事务。
	ClearPermissions(ctx context.Context, roleID uint) error
	// ClearHomeMenuRef 清理 admin_role.home_menu_id 中指向指定 menuID 的引用（置 0）。
	// 用于 menu Delete 的级联清理：menu 删除后将所有以该 menu 为首页的 role 的 home_menu_id 重置为 0，
	// 避免角色首页路由悬空指向已删 menu。role.home_menu_id 仅为数值引用，无 FK CASCADE，需手动维护。
	// 支持 context 传播事务。
	ClearHomeMenuRef(ctx context.Context, menuID uint) error
	GetByID(ctx context.Context, id uint) (*systemEntity.Role, error)
	GetByCode(ctx context.Context, code string) (*systemEntity.Role, error)
	List(ctx context.Context, query *RoleRepoQuery) ([]*systemEntity.Role, int64, error)
	ExistsByCode(ctx context.Context, code string, excludeID ...uint) (bool, error)
	GetAll(ctx context.Context) ([]*systemEntity.Role, error)
	GetByCodes(ctx context.Context, codes []string) ([]*systemEntity.Role, error)
}

type RoleRepoQuery struct {
	Name    string
	Code    string
	Status  *string
	Current int
	Size    int
}

type roleRepository struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) RoleRepository {
	return &roleRepository{db: db}
}

// getDB 取当前 context 下的 *gorm.DB：若 ctx 中存在事务句柄则复用事务，否则回退到 r.db。
func (r *roleRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *roleRepository) Create(ctx context.Context, role *systemEntity.Role) error {
	return r.getDB(ctx).Create(role).Error
}

func (r *roleRepository) Update(ctx context.Context, role *systemEntity.Role) error {
	return r.getDB(ctx).Save(role).Error
}

// Delete 硬删除 role 主数据（需 Unscoped 绕过软删除）。
// 注意：本方法仅删除主数据，不再清理关联表（清理职责已上移到 Service 层 TM 事务）。
// Service 层 Delete 应在 TM 事务内先调 ClearUserRoles + ClearPermissions 再调 Delete，保证原子性。
func (r *roleRepository) Delete(ctx context.Context, id uint) error {
	return r.getDB(ctx).Unscoped().Delete(&systemEntity.Role{}, id).Error
}

// ClearUserRoles 清理 admin_user_roles 关联表中指定 role 的全部管理员关联。
// 用于 role Delete 的 TM 事务编排，支持 context 传播事务。
// 使用原生 SQL 直接删除关联表行，避免加载实体再 Association().Clear() 的额外查询开销。
func (r *roleRepository) ClearUserRoles(ctx context.Context, roleID uint) error {
	return r.getDB(ctx).
		Table("admin_user_roles").
		Where("admin_role_id = ?", roleID).
		Delete(nil).Error
}

// ClearPermissions 清理 role 的全部权限关联表：
//   - admin_role_menus（role ↔ menu）
//   - admin_role_buttons（role ↔ button）
//   - admin_role_apis（role ↔ api）
//
// 用于 role Delete 的 TM 事务编排，支持 context 传播事务。
// 使用原生 SQL 批量删除三张关联表，避免加载实体再 Association().Clear() 的额外查询开销。
func (r *roleRepository) ClearPermissions(ctx context.Context, roleID uint) error {
	db := r.getDB(ctx)
	tables := []string{"admin_role_menus", "admin_role_buttons", "admin_role_apis"}
	for _, table := range tables {
		if err := db.Table(table).Where("admin_role_id = ?", roleID).Delete(nil).Error; err != nil {
			return err
		}
	}
	return nil
}

// ClearHomeMenuRef 清理 admin_role.home_menu_id 中指向指定 menuID 的引用（置 0）。
// 用于 menu Delete 的级联清理：menu 删除后将所有以该 menu 为首页的 role 的 home_menu_id 重置为 0，
// 避免角色首页路由悬空指向已删 menu。role.home_menu_id 仅为数值引用（无 FK CASCADE），需手动维护。
// 使用 UpdateColumn 跳过 GORM 钩子，直接更新字段值。支持 context 传播事务。
func (r *roleRepository) ClearHomeMenuRef(ctx context.Context, menuID uint) error {
	return r.getDB(ctx).Model(&systemEntity.Role{}).
		Where("home_menu_id = ?", menuID).
		UpdateColumn("home_menu_id", 0).Error
}

func (r *roleRepository) GetByID(ctx context.Context, id uint) (*systemEntity.Role, error) {
	var role systemEntity.Role
	err := r.getDB(ctx).
		Preload("Menus").
		Preload("Buttons").
		Preload("Apis").
		Preload("HomeMenu").
		First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *roleRepository) GetByCode(ctx context.Context, code string) (*systemEntity.Role, error) {
	var role systemEntity.Role
	err := r.getDB(ctx).
		Preload("Menus").
		Preload("Buttons").
		Preload("Apis").
		Preload("HomeMenu").
		Where("code = ?", code).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *roleRepository) List(ctx context.Context, query *RoleRepoQuery) ([]*systemEntity.Role, int64, error) {
	var roles []*systemEntity.Role
	var total int64

	db := r.getDB(ctx).Model(&systemEntity.Role{})

	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Code != "" {
		db = db.Where("code LIKE ?", "%"+query.Code+"%")
	}
	if query.Status != nil && *query.Status != "" {
		db = db.Where("status = ?", *query.Status)
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

	if err := db.Order("id DESC").Scopes(pagination.Paginate(query.Current, query.Size)).Find(&roles).Error; err != nil {
		return nil, 0, err
	}

	return roles, total, nil
}

func (r *roleRepository) ExistsByCode(ctx context.Context, code string, excludeID ...uint) (bool, error) {
	var count int64
	db := r.getDB(ctx).Model(&systemEntity.Role{}).Where("code = ?", code)
	if len(excludeID) > 0 {
		db = db.Where("id != ?", excludeID[0])
	}
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *roleRepository) GetAll(ctx context.Context) ([]*systemEntity.Role, error) {
	var roles []*systemEntity.Role
	err := r.getDB(ctx).Where("status = ?", entity.StatusEnabled).Find(&roles).Error
	return roles, err
}

func (r *roleRepository) GetByCodes(ctx context.Context, codes []string) ([]*systemEntity.Role, error) {
	var roles []*systemEntity.Role
	if len(codes) == 0 {
		return roles, nil
	}
	err := r.getDB(ctx).Where("code IN ?", codes).Find(&roles).Error
	return roles, err
}
