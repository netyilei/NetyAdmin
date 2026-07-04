package system

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/pagination"
)

type MenuRepository interface {
	Create(ctx context.Context, menu *systemEntity.Menu) error
	Update(ctx context.Context, menu *systemEntity.Menu) error
	Delete(ctx context.Context, id uint) error
	// ClearRoleMenus 清理 admin_role_menus 关联表中指定 menu 的全部角色关联。
	// 用于 menu Delete 的 TM 事务编排，支持 context 传播事务。
	ClearRoleMenus(ctx context.Context, menuID uint) error
	GetByID(ctx context.Context, id uint) (*systemEntity.Menu, error)
	GetByRouteName(ctx context.Context, routeName string) (*systemEntity.Menu, error)
	List(ctx context.Context, query *MenuRepoQuery) ([]*systemEntity.Menu, int64, error)
	GetTree(ctx context.Context) ([]*systemEntity.Menu, error)
	GetAll(ctx context.Context) ([]systemEntity.Menu, error)
	GetByRoleID(ctx context.Context, roleID uint) ([]*systemEntity.Menu, error)
	GetAllPages(ctx context.Context) ([]*systemEntity.Menu, error)
	GetAllWithButtons(ctx context.Context) ([]systemEntity.Menu, error)
	GetAllWithApis(ctx context.Context) ([]systemEntity.Menu, error)
	ExistsByRouteName(ctx context.Context, routeName string, excludeID ...uint) (bool, error)
	HasChildren(ctx context.Context, id uint) (bool, error)
	GetByRoleCodes(ctx context.Context, roleCodes []string) ([]*systemEntity.Menu, error)
}

type MenuRepoQuery struct {
	Name     string
	Status   *string
	ParentID *uint
	Current  int
	Size     int
}

type menuRepository struct {
	db *gorm.DB
}

func NewMenuRepository(db *gorm.DB) MenuRepository {
	return &menuRepository{db: db}
}

// getDB 取当前 context 下的 *gorm.DB：若 ctx 中存在事务句柄则复用事务，否则回退到 r.db。
func (r *menuRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *menuRepository) Create(ctx context.Context, menu *systemEntity.Menu) error {
	return r.getDB(ctx).Create(menu).Error
}

func (r *menuRepository) Update(ctx context.Context, menu *systemEntity.Menu) error {
	return r.getDB(ctx).Save(menu).Error
}

// Delete 硬删除 menu 主数据（需 Unscoped 绕过软删除）。
// 注意：本方法仅删除主数据，不再清理 buttons/admin_role_menus 关联表（清理职责已上移到 Service 层 TM 事务）。
// Service 层 Delete 应在 TM 事务内先调 buttonRepo.DeleteByMenuID + menuRepo.ClearRoleMenus 再调 Delete，保证原子性。
func (r *menuRepository) Delete(ctx context.Context, id uint) error {
	return r.getDB(ctx).Unscoped().Delete(&systemEntity.Menu{}, id).Error
}

// ClearRoleMenus 清理 admin_role_menus 关联表中指定 menu 的全部角色关联。
// 用于 menu Delete 的 TM 事务编排，支持 context 传播事务。
// 使用原生 SQL 直接删除关联表行，避免加载实体再 Association().Clear() 的额外查询开销。
func (r *menuRepository) ClearRoleMenus(ctx context.Context, menuID uint) error {
	return r.getDB(ctx).
		Table("admin_role_menus").
		Where("admin_menu_id = ?", menuID).
		Delete(nil).Error
}

func (r *menuRepository) GetByID(ctx context.Context, id uint) (*systemEntity.Menu, error) {
	var menu systemEntity.Menu
	if err := r.getDB(ctx).Preload("Buttons").First(&menu, id).Error; err != nil {
		return nil, err
	}
	return &menu, nil
}

func (r *menuRepository) GetByRouteName(ctx context.Context, routeName string) (*systemEntity.Menu, error) {
	var menu systemEntity.Menu
	if err := r.getDB(ctx).Where("route_name = ?", routeName).First(&menu).Error; err != nil {
		return nil, err
	}
	return &menu, nil
}

func (r *menuRepository) List(ctx context.Context, query *MenuRepoQuery) ([]*systemEntity.Menu, int64, error) {
	var menus []*systemEntity.Menu
	var total int64

	db := r.getDB(ctx).Model(&systemEntity.Menu{})

	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Status != nil && *query.Status != "" {
		db = db.Where("status = ?", *query.Status)
	}
	if query.ParentID != nil {
		db = db.Where("parent_id = ?", *query.ParentID)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if query.Current <= 0 {
		query.Current = 1
	}
	if query.Size <= 0 {
		query.Size = 50 // 菜单列表默认页大小为 50（菜单数据量较大，故使用 50 而非 entity.DefaultPageSize=20）
	}

	if err := db.Order("order_by ASC, id ASC").Scopes(pagination.Paginate(query.Current, query.Size)).Find(&menus).Error; err != nil {
		return nil, 0, err
	}

	return menus, total, nil
}

func (r *menuRepository) GetTree(ctx context.Context) ([]*systemEntity.Menu, error) {
	var menus []*systemEntity.Menu
	err := r.getDB(ctx).
		Where("status = ?", entity.StatusEnabled).
		Order("order_by ASC, id ASC").
		Find(&menus).Error
	if err != nil {
		return nil, err
	}
	return menus, nil
}

func (r *menuRepository) GetAll(ctx context.Context) ([]systemEntity.Menu, error) {
	var menus []systemEntity.Menu
	err := r.getDB(ctx).Order("order_by ASC, id ASC").Find(&menus).Error
	return menus, err
}

func (r *menuRepository) GetAllWithButtons(ctx context.Context) ([]systemEntity.Menu, error) {
	var menus []systemEntity.Menu
	err := r.getDB(ctx).Preload("Buttons").Order("order_by ASC, id ASC").Find(&menus).Error
	return menus, err
}

func (r *menuRepository) GetAllWithApis(ctx context.Context) ([]systemEntity.Menu, error) {
	var menus []systemEntity.Menu
	err := r.getDB(ctx).Preload("Apis").Order("order_by ASC, id ASC").Find(&menus).Error
	return menus, err
}

func (r *menuRepository) ExistsByRouteName(ctx context.Context, routeName string, excludeID ...uint) (bool, error) {
	var count int64
	db := r.getDB(ctx).Model(&systemEntity.Menu{}).Where("route_name = ?", routeName)
	if len(excludeID) > 0 {
		db = db.Where("id != ?", excludeID[0])
	}
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *menuRepository) HasChildren(ctx context.Context, id uint) (bool, error) {
	var count int64
	if err := r.getDB(ctx).Model(&systemEntity.Menu{}).Where("parent_id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *menuRepository) GetByRoleID(ctx context.Context, roleID uint) ([]*systemEntity.Menu, error) {
	var menus []*systemEntity.Menu
	err := r.getDB(ctx).
		Joins("JOIN admin_role_menus ON admin_menu.id = admin_role_menus.admin_menu_id").
		Where("admin_role_menus.admin_role_id = ?", roleID).
		Find(&menus).Error
	if err != nil {
		return nil, err
	}
	return menus, nil
}

func (r *menuRepository) GetByRoleCodes(ctx context.Context, roleCodes []string) ([]*systemEntity.Menu, error) {
	var menus []*systemEntity.Menu
	if len(roleCodes) == 0 {
		return menus, nil
	}

	err := r.getDB(ctx).
		Distinct("admin_menu.*").
		Joins("JOIN admin_role_menus ON admin_menu.id = admin_role_menus.admin_menu_id").
		Joins("JOIN admin_role ON admin_role_menus.admin_role_id = admin_role.id").
		Where("admin_role.code IN ?", roleCodes).
		Where("admin_menu.status = ?", entity.StatusEnabled).
		Order("admin_menu.order_by ASC, admin_menu.id ASC").
		Find(&menus).Error

	return menus, err
}

func (r *menuRepository) GetAllPages(ctx context.Context) ([]*systemEntity.Menu, error) {
	var menus []*systemEntity.Menu
	err := r.getDB(ctx).
		Where("type = ?", systemEntity.MenuTypePage).
		Where("component IS NOT NULL AND component != ''").
		Find(&menus).Error
	if err != nil {
		return nil, err
	}
	return menus, nil
}
