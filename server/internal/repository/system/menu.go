package system

import (
	"context"

	"gorm.io/gorm"

	systemEntity "silentorder/internal/domain/entity/system"
)

type MenuRepository interface {
	Create(ctx context.Context, menu *systemEntity.Menu) error
	Update(ctx context.Context, menu *systemEntity.Menu) error
	Delete(ctx context.Context, id uint) error
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

func (r *menuRepository) Create(ctx context.Context, menu *systemEntity.Menu) error {
	return r.db.WithContext(ctx).Create(menu).Error
}

func (r *menuRepository) Update(ctx context.Context, menu *systemEntity.Menu) error {
	return r.db.WithContext(ctx).Save(menu).Error
}

func (r *menuRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&systemEntity.Menu{}, id).Error
}

func (r *menuRepository) GetByID(ctx context.Context, id uint) (*systemEntity.Menu, error) {
	var menu systemEntity.Menu
	if err := r.db.WithContext(ctx).Preload("Buttons").First(&menu, id).Error; err != nil {
		return nil, err
	}
	return &menu, nil
}

func (r *menuRepository) GetByRouteName(ctx context.Context, routeName string) (*systemEntity.Menu, error) {
	var menu systemEntity.Menu
	if err := r.db.WithContext(ctx).Where("route_name = ?", routeName).First(&menu).Error; err != nil {
		return nil, err
	}
	return &menu, nil
}

func (r *menuRepository) List(ctx context.Context, query *MenuRepoQuery) ([]*systemEntity.Menu, int64, error) {
	var menus []*systemEntity.Menu
	var total int64

	db := r.db.WithContext(ctx).Model(&systemEntity.Menu{})

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
		query.Size = 50
	}

	offset := (query.Current - 1) * query.Size
	if err := db.Order("order_by ASC, id ASC").Offset(offset).Limit(query.Size).Find(&menus).Error; err != nil {
		return nil, 0, err
	}

	return menus, total, nil
}

func (r *menuRepository) GetTree(ctx context.Context) ([]*systemEntity.Menu, error) {
	var menus []*systemEntity.Menu
	err := r.db.WithContext(ctx).
		Where("status = ?", systemEntity.MenuStatusEnabled).
		Order("order_by ASC, id ASC").
		Find(&menus).Error
	if err != nil {
		return nil, err
	}
	return menus, nil
}

func (r *menuRepository) GetAll(ctx context.Context) ([]systemEntity.Menu, error) {
	var menus []systemEntity.Menu
	err := r.db.WithContext(ctx).Order("order_by ASC, id ASC").Find(&menus).Error
	return menus, err
}

func (r *menuRepository) GetAllWithButtons(ctx context.Context) ([]systemEntity.Menu, error) {
	var menus []systemEntity.Menu
	err := r.db.WithContext(ctx).Preload("Buttons").Order("order_by ASC, id ASC").Find(&menus).Error
	return menus, err
}

func (r *menuRepository) GetAllWithApis(ctx context.Context) ([]systemEntity.Menu, error) {
	var menus []systemEntity.Menu
	err := r.db.WithContext(ctx).Preload("Apis").Order("order_by ASC, id ASC").Find(&menus).Error
	return menus, err
}

func (r *menuRepository) ExistsByRouteName(ctx context.Context, routeName string, excludeID ...uint) (bool, error) {
	var count int64
	db := r.db.WithContext(ctx).Model(&systemEntity.Menu{}).Where("route_name = ?", routeName)
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
	if err := r.db.WithContext(ctx).Model(&systemEntity.Menu{}).Where("parent_id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *menuRepository) GetByRoleID(ctx context.Context, roleID uint) ([]*systemEntity.Menu, error) {
	var menus []*systemEntity.Menu
	err := r.db.WithContext(ctx).
		Joins("JOIN admin_role_menus ON admin_menu.id = admin_role_menus.admin_menu_id").
		Where("admin_role_menus.admin_role_id = ?", roleID).
		Find(&menus).Error
	if err != nil {
		return nil, err
	}
	return menus, nil
}

func (r *menuRepository) GetAllPages(ctx context.Context) ([]*systemEntity.Menu, error) {
	var menus []*systemEntity.Menu
	err := r.db.WithContext(ctx).
		Where("type = ?", systemEntity.MenuTypePage).
		Where("component IS NOT NULL AND component != ''").
		Find(&menus).Error
	if err != nil {
		return nil, err
	}
	return menus, nil
}
