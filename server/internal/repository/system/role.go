package system

import (
	"context"

	"gorm.io/gorm"

	systemEntity "NetyAdmin/internal/domain/entity/system"
)

type RoleRepository interface {
	Create(ctx context.Context, role *systemEntity.Role) error
	Update(ctx context.Context, role *systemEntity.Role) error
	Delete(ctx context.Context, id uint) error
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

func (r *roleRepository) Create(ctx context.Context, role *systemEntity.Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

func (r *roleRepository) Update(ctx context.Context, role *systemEntity.Role) error {
	return r.db.WithContext(ctx).Save(role).Error
}

func (r *roleRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&systemEntity.Role{}, id).Error
}

func (r *roleRepository) GetByID(ctx context.Context, id uint) (*systemEntity.Role, error) {
	var role systemEntity.Role
	err := r.db.WithContext(ctx).
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
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *roleRepository) List(ctx context.Context, query *RoleRepoQuery) ([]*systemEntity.Role, int64, error) {
	var roles []*systemEntity.Role
	var total int64

	db := r.db.WithContext(ctx).Model(&systemEntity.Role{})

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
		query.Size = 10
	}

	offset := (query.Current - 1) * query.Size
	if err := db.Order("id DESC").Offset(offset).Limit(query.Size).Find(&roles).Error; err != nil {
		return nil, 0, err
	}

	return roles, total, nil
}

func (r *roleRepository) ExistsByCode(ctx context.Context, code string, excludeID ...uint) (bool, error) {
	var count int64
	db := r.db.WithContext(ctx).Model(&systemEntity.Role{}).Where("code = ?", code)
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
	err := r.db.WithContext(ctx).Where("status = ?", systemEntity.RoleStatusEnabled).Find(&roles).Error
	return roles, err
}

func (r *roleRepository) GetByCodes(ctx context.Context, codes []string) ([]*systemEntity.Role, error) {
	var roles []*systemEntity.Role
	if len(codes) == 0 {
		return roles, nil
	}
	err := r.db.WithContext(ctx).Where("code IN ?", codes).Find(&roles).Error
	return roles, err
}
