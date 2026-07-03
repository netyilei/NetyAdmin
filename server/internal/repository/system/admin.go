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
	// GetAuthStateByID 仅查询鉴权所需字段（token_version, status），无 Preload。
	// 用于 JWTAuth 中间件的高频路径，避免 GetByID 触发 Roles/Roles.Buttons/
	// CreatedByUser/UpdatedByUser 四个 Preload 的过度 JOIN。
	GetAuthStateByID(ctx context.Context, id uint) (*AdminAuthState, error)
	// DeleteWithTokenInvalidation 单事务原子完成「清理角色关联 + 递增 token_version + 软删除」。
	// 用于 admin Delete/DeleteBatch 的 fail-closed 改造，与 user 侧语义对齐。
	DeleteWithTokenInvalidation(ctx context.Context, id uint) error
}

// AdminAuthState 是鉴权中间件所需的账户最小字段集。
type AdminAuthState struct {
	TokenVersion uint64
	Status       string
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

// GetAuthStateByID 仅查询鉴权所需的 token_version 与 status 字段，无 Preload。
// 用于 JWTAuth 中间件的高频路径，避免每次鉴权触发 4 个 Preload 的过度 JOIN。
func (r *adminRepository) GetAuthStateByID(ctx context.Context, id uint) (*AdminAuthState, error) {
	var state AdminAuthState
	if err := r.db.WithContext(ctx).
		Model(&systemEntity.Admin{}).
		Select("token_version", "status").
		Where("id = ?", id).
		Take(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
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

// DeleteWithTokenInvalidation 单事务原子完成「清理角色关联 + 递增 token_version + 软删除」。
//
// 设计动机（与 user 侧对齐）：
//   - 旧实现：invalidateAdminTokens（Inc）与 adminRepo.Delete 分离，Inc 成功+Delete 失败时
//     版本号已变但账号还在，旧 token 立即失效但用户未被删（中间态）
//   - 新实现：三步收敛到单事务，任一失败整体回滚，调用方据此实现 fail-closed
//
// 事务内顺序：清理角色关联 → 递增 token_version → 软删除。
// 先 Inc 再 Delete 的好处：若 Delete 失败，版本号已递增使旧 token 失效（fail-safe）。
func (r *adminRepository) DeleteWithTokenInvalidation(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 清理 many2many 角色关联（保留原 Delete 的事务行为）
		var admin systemEntity.Admin
		if err := tx.First(&admin, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&admin).Association("Roles").Clear(); err != nil {
			return err
		}
		// 2. 递增 token_version（使旧 token 失效，纵深防御）
		if err := tx.Model(&systemEntity.Admin{}).
			Where("id = ?", id).
			UpdateColumn("token_version", gorm.Expr("token_version + ?", 1)).Error; err != nil {
			return err
		}
		// 3. 软删除管理员主数据
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
