package system

import (
	"context"

	"gorm.io/gorm"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/pagination"
)

type AdminRepository interface {
	Create(ctx context.Context, admin *systemEntity.Admin) error
	GetByID(ctx context.Context, id uint) (*systemEntity.Admin, error)
	GetByUsername(ctx context.Context, username string) (*systemEntity.Admin, error)
	ExistsByUsername(ctx context.Context, username string, excludeID ...uint) (bool, error)
	List(ctx context.Context, query *AdminRepoQuery) ([]systemEntity.Admin, int64, error)
	Update(ctx context.Context, admin *systemEntity.Admin) error
	UpdateLastLoginAt(ctx context.Context, id uint, lastLoginAt string) error
	// IncrementTokenVersion 原子递增管理员 token_version（BUG #5）。
	IncrementTokenVersion(ctx context.Context, id uint) error
	Delete(ctx context.Context, id uint) error
	UpdateRoles(ctx context.Context, adminID uint, roleIDs []uint) error
}

type AdminRepoQuery struct {
	Current  int
	Size     int
	Username string
	Nickname string
	Gender   *string
	Phone    string
	Email    string
	Status   *string
}

type adminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) AdminRepository {
	return &adminRepository{db: db}
}

func (r *adminRepository) Create(ctx context.Context, admin *systemEntity.Admin) error {
	return r.db.WithContext(ctx).Create(admin).Error
}

func (r *adminRepository) GetByID(ctx context.Context, id uint) (*systemEntity.Admin, error) {
	var admin systemEntity.Admin
	if err := r.db.WithContext(ctx).
		Preload("Roles").Preload("Roles.Buttons").
		Preload("CreatedByUser").Preload("UpdatedByUser").
		First(&admin, id).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *adminRepository) GetByUsername(ctx context.Context, username string) (*systemEntity.Admin, error) {
	var admin systemEntity.Admin
	// 注意：根据实体的 GORM Tag，列名应为 username
	if err := r.db.WithContext(ctx).Preload("Roles").Preload("Roles.Buttons").Where("username = ?", username).First(&admin).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *adminRepository) ExistsByUsername(ctx context.Context, username string, excludeID ...uint) (bool, error) {
	query := r.db.WithContext(ctx).Model(&systemEntity.Admin{}).Where("username = ?", username)
	if len(excludeID) > 0 {
		query = query.Where("id <> ?", excludeID[0])
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func (r *adminRepository) List(ctx context.Context, query *AdminRepoQuery) ([]systemEntity.Admin, int64, error) {
	var admins []systemEntity.Admin
	var total int64

	db := r.db.WithContext(ctx).Model(&systemEntity.Admin{})

	if query.Username != "" {
		db = db.Where("username LIKE ?", "%"+query.Username+"%")
	}
	if query.Nickname != "" {
		db = db.Where("nickname LIKE ?", "%"+query.Nickname+"%")
	}
	if query.Gender != nil && *query.Gender != "" {
		db = db.Where("gender = ?", *query.Gender)
	}
	if query.Phone != "" {
		db = db.Where("phone LIKE ?", "%"+query.Phone+"%")
	}
	if query.Email != "" {
		db = db.Where("email LIKE ?", "%"+query.Email+"%")
	}
	if query.Status != nil && *query.Status != "" {
		db = db.Where("status = ?", *query.Status)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Order("id DESC").Scopes(pagination.Paginate(query.Current, query.Size)).
		Preload("Roles").
		Preload("CreatedByUser").
		Preload("UpdatedByUser").
		Find(&admins).Error; err != nil {
		return nil, 0, err
	}

	return admins, total, nil
}

func (r *adminRepository) Update(ctx context.Context, admin *systemEntity.Admin) error {
	return r.db.WithContext(ctx).Save(admin).Error
}

func (r *adminRepository) UpdateLastLoginAt(ctx context.Context, id uint, lastLoginAt string) error {
	return r.db.WithContext(ctx).Model(&systemEntity.Admin{}).
		Where("id = ?", id).
		UpdateColumn("last_login_at", lastLoginAt).Error
}

// IncrementTokenVersion 原子递增管理员 token_version（BUG #5）。
func (r *adminRepository) IncrementTokenVersion(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&systemEntity.Admin{}).
		Where("id = ?", id).
		UpdateColumn("token_version", gorm.Expr("token_version + ?", 1)).Error
}

func (r *adminRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先清理 many2many 角色关联，避免关联表残留数据
		var admin systemEntity.Admin
		if err := tx.First(&admin, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&admin).Association("Roles").Clear(); err != nil {
			return err
		}
		return tx.Delete(&systemEntity.Admin{}, id).Error
	})
}

func (r *adminRepository) UpdateRoles(ctx context.Context, adminID uint, roleIDs []uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var admin systemEntity.Admin
		if err := tx.First(&admin, adminID).Error; err != nil {
			return err
		}

		var roles []systemEntity.Role
		if len(roleIDs) > 0 {
			if err := tx.Find(&roles, roleIDs).Error; err != nil {
				return err
			}
		}

		return tx.Model(&admin).Association("Roles").Replace(roles)
	})
}
