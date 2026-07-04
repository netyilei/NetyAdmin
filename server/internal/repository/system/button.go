package system

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/pagination"
)

type ButtonRepository interface {
	Create(ctx context.Context, button *systemEntity.Button) error
	Update(ctx context.Context, button *systemEntity.Button) error
	Delete(ctx context.Context, id uint) error
	DeleteByMenuID(ctx context.Context, menuID uint) error
	// ClearRoleButtons 清理 admin_role_buttons 关联表中指定 button 的全部角色关联。
	// 用于 button Delete 的 TM 事务编排，支持 context 传播事务。
	ClearRoleButtons(ctx context.Context, buttonID uint) error
	// ClearRoleButtonsByMenuID 清理 admin_role_buttons 关联表中指定 menu 下全部 button 的角色关联。
	// 用于 menu Delete 的级联清理，通过子查询按 menu_id 定位 button 再删除其角色关联，支持 context 传播事务。
	ClearRoleButtonsByMenuID(ctx context.Context, menuID uint) error
	GetByID(ctx context.Context, id uint) (*systemEntity.Button, error)
	GetByCode(ctx context.Context, code string) (*systemEntity.Button, error)
	List(ctx context.Context, query *ButtonRepoQuery) ([]*systemEntity.Button, int64, error)
	GetByMenuID(ctx context.Context, menuID uint) ([]*systemEntity.Button, error)
	GetByMenuIDs(ctx context.Context, menuIDs []uint) ([]*systemEntity.Button, error)
	GetByRoleID(ctx context.Context, roleID uint) ([]*systemEntity.Button, error)
	GetAll(ctx context.Context) ([]*systemEntity.Button, error)
	ExistsByCode(ctx context.Context, code string, excludeID ...uint) (bool, error)
}

type ButtonRepoQuery struct {
	Label   string
	Code    string
	MenuID  *uint
	Current int
	Size    int
}

type buttonRepository struct {
	db *gorm.DB
}

func NewButtonRepository(db *gorm.DB) ButtonRepository {
	return &buttonRepository{db: db}
}

// getDB 取当前 context 下的 *gorm.DB：若 ctx 中存在事务句柄则复用事务，否则回退到 r.db。
func (r *buttonRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *buttonRepository) Create(ctx context.Context, button *systemEntity.Button) error {
	return r.getDB(ctx).Create(button).Error
}

func (r *buttonRepository) Update(ctx context.Context, button *systemEntity.Button) error {
	return r.getDB(ctx).Save(button).Error
}

func (r *buttonRepository) Delete(ctx context.Context, id uint) error {
	return r.getDB(ctx).Unscoped().Delete(&systemEntity.Button{}, id).Error
}

func (r *buttonRepository) DeleteByMenuID(ctx context.Context, menuID uint) error {
	return r.getDB(ctx).Where("menu_id = ?", menuID).Unscoped().Delete(&systemEntity.Button{}).Error
}

// ClearRoleButtons 清理 admin_role_buttons 关联表中指定 button 的全部角色关联。
// 用于 button Delete 的 TM 事务编排，支持 context 传播事务。
// 使用原生 SQL 直接删除关联表行，避免加载实体再 Association().Clear() 的额外查询开销。
func (r *buttonRepository) ClearRoleButtons(ctx context.Context, buttonID uint) error {
	return r.getDB(ctx).
		Table("admin_role_buttons").
		Where("admin_button_id = ?", buttonID).
		Delete(nil).Error
}

// ClearRoleButtonsByMenuID 清理 admin_role_buttons 关联表中指定 menu 下全部 button 的角色关联。
// 用于 menu Delete 的级联清理：通过子查询按 menu_id 定位 admin_button.id，再删除其角色关联行，
// 避免「button 行已硬删除但 admin_role_buttons 仍残留指向已删 button 的孤儿引用」。
// 支持 context 传播事务。
func (r *buttonRepository) ClearRoleButtonsByMenuID(ctx context.Context, menuID uint) error {
	return r.getDB(ctx).
		Table("admin_role_buttons").
		Where("admin_button_id IN (SELECT id FROM admin_button WHERE menu_id = ?)", menuID).
		Delete(nil).Error
}

func (r *buttonRepository) GetByID(ctx context.Context, id uint) (*systemEntity.Button, error) {
	var button systemEntity.Button
	err := r.getDB(ctx).First(&button, id).Error
	if err != nil {
		return nil, err
	}
	return &button, nil
}

func (r *buttonRepository) GetByCode(ctx context.Context, code string) (*systemEntity.Button, error) {
	var button systemEntity.Button
	err := r.getDB(ctx).Where("code = ?", code).First(&button).Error
	if err != nil {
		return nil, err
	}
	return &button, nil
}

func (r *buttonRepository) List(ctx context.Context, query *ButtonRepoQuery) ([]*systemEntity.Button, int64, error) {
	var buttons []*systemEntity.Button
	var total int64

	db := r.getDB(ctx).Model(&systemEntity.Button{})

	if query.Label != "" {
		db = db.Where("label LIKE ?", "%"+query.Label+"%")
	}
	if query.Code != "" {
		db = db.Where("code LIKE ?", "%"+query.Code+"%")
	}
	if query.MenuID != nil {
		db = db.Where("menu_id = ?", *query.MenuID)
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

	if err := db.Order("id DESC").Scopes(pagination.Paginate(query.Current, query.Size)).Find(&buttons).Error; err != nil {
		return nil, 0, err
	}

	return buttons, total, nil
}

func (r *buttonRepository) GetByMenuID(ctx context.Context, menuID uint) ([]*systemEntity.Button, error) {
	var buttons []*systemEntity.Button
	err := r.getDB(ctx).Where("menu_id = ?", menuID).Find(&buttons).Error
	return buttons, err
}

func (r *buttonRepository) GetByMenuIDs(ctx context.Context, menuIDs []uint) ([]*systemEntity.Button, error) {
	var buttons []*systemEntity.Button
	err := r.getDB(ctx).Where("menu_id IN ?", menuIDs).Find(&buttons).Error
	return buttons, err
}

func (r *buttonRepository) ExistsByCode(ctx context.Context, code string, excludeID ...uint) (bool, error) {
	var count int64
	db := r.getDB(ctx).Model(&systemEntity.Button{}).Where("code = ?", code)
	if len(excludeID) > 0 {
		db = db.Where("id != ?", excludeID[0])
	}
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *buttonRepository) GetByRoleID(ctx context.Context, roleID uint) ([]*systemEntity.Button, error) {
	var buttons []*systemEntity.Button
	err := r.getDB(ctx).
		Joins("JOIN admin_role_buttons ON admin_button.id = admin_role_buttons.admin_button_id").
		Where("admin_role_buttons.admin_role_id = ?", roleID).
		Find(&buttons).Error
	if err != nil {
		return nil, err
	}
	return buttons, nil
}

func (r *buttonRepository) GetAll(ctx context.Context) ([]*systemEntity.Button, error) {
	var buttons []*systemEntity.Button
	err := r.getDB(ctx).Find(&buttons).Error
	return buttons, err
}
