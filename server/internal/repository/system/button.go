package system

import (
	"context"

	"gorm.io/gorm"

	systemEntity "netyadmin/internal/domain/entity/system"
)

type ButtonRepository interface {
	Create(ctx context.Context, button *systemEntity.Button) error
	Update(ctx context.Context, button *systemEntity.Button) error
	Delete(ctx context.Context, id uint) error
	DeleteByMenuID(ctx context.Context, menuID uint) error
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

func (r *buttonRepository) Create(ctx context.Context, button *systemEntity.Button) error {
	return r.db.WithContext(ctx).Create(button).Error
}

func (r *buttonRepository) Update(ctx context.Context, button *systemEntity.Button) error {
	return r.db.WithContext(ctx).Save(button).Error
}

func (r *buttonRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&systemEntity.Button{}, id).Error
}

func (r *buttonRepository) DeleteByMenuID(ctx context.Context, menuID uint) error {
	return r.db.WithContext(ctx).Where("menu_id = ?", menuID).Delete(&systemEntity.Button{}).Error
}

func (r *buttonRepository) GetByID(ctx context.Context, id uint) (*systemEntity.Button, error) {
	var button systemEntity.Button
	err := r.db.WithContext(ctx).First(&button, id).Error
	if err != nil {
		return nil, err
	}
	return &button, nil
}

func (r *buttonRepository) GetByCode(ctx context.Context, code string) (*systemEntity.Button, error) {
	var button systemEntity.Button
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&button).Error
	if err != nil {
		return nil, err
	}
	return &button, nil
}

func (r *buttonRepository) List(ctx context.Context, query *ButtonRepoQuery) ([]*systemEntity.Button, int64, error) {
	var buttons []*systemEntity.Button
	var total int64

	db := r.db.WithContext(ctx).Model(&systemEntity.Button{})

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
		query.Size = 10
	}

	offset := (query.Current - 1) * query.Size
	if err := db.Order("id DESC").Offset(offset).Limit(query.Size).Find(&buttons).Error; err != nil {
		return nil, 0, err
	}

	return buttons, total, nil
}

func (r *buttonRepository) GetByMenuID(ctx context.Context, menuID uint) ([]*systemEntity.Button, error) {
	var buttons []*systemEntity.Button
	err := r.db.WithContext(ctx).Where("menu_id = ?", menuID).Find(&buttons).Error
	return buttons, err
}

func (r *buttonRepository) GetByMenuIDs(ctx context.Context, menuIDs []uint) ([]*systemEntity.Button, error) {
	var buttons []*systemEntity.Button
	err := r.db.WithContext(ctx).Where("menu_id IN ?", menuIDs).Find(&buttons).Error
	return buttons, err
}

func (r *buttonRepository) ExistsByCode(ctx context.Context, code string, excludeID ...uint) (bool, error) {
	var count int64
	db := r.db.WithContext(ctx).Model(&systemEntity.Button{}).Where("code = ?", code)
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
	err := r.db.WithContext(ctx).
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
	err := r.db.WithContext(ctx).Find(&buttons).Error
	return buttons, err
}
