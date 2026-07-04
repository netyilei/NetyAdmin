package system

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/database"
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
	// 用于改密/禁用/删除等敏感操作，使旧 token 携带的版本号失效。
	// 支持 context 传播事务：若 ctx 中携带 *Tx 则复用事务句柄。
	IncrementTokenVersion(ctx context.Context, id uint) error
	// ClearRoles 清理 admin_user_roles 关联表中指定管理员的全部角色关联。
	// 用于 Delete/DeleteBatch 的 TM 事务编排，支持 context 传播事务。
	ClearRoles(ctx context.Context, adminID uint) error
	Delete(ctx context.Context, id uint) error
	UpdateRoles(ctx context.Context, adminID uint, roleIDs []uint) error
	// GetAuthStateByID 仅查询鉴权所需字段（token_version, status），无 Preload。
	// 用于 JWTAuth 中间件的高频路径，避免 GetByID 触发 Roles/Roles.Buttons/
	// CreatedByUser/UpdatedByUser 四个 Preload 的过度 JOIN。
	GetAuthStateByID(ctx context.Context, id uint) (*AdminAuthState, error)
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

// getDB 取当前 context 下的 *gorm.DB：若 ctx 中存在事务句柄则复用事务，否则回退到 r.db。
func (r *adminRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *adminRepository) Create(ctx context.Context, admin *systemEntity.Admin) error {
	return r.getDB(ctx).Create(admin).Error
}

func (r *adminRepository) GetByID(ctx context.Context, id uint) (*systemEntity.Admin, error) {
	var admin systemEntity.Admin
	if err := r.getDB(ctx).
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
	if err := r.getDB(ctx).
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
	if err := r.getDB(ctx).Preload("Roles").Preload("Roles.Buttons").Where("username = ?", username).First(&admin).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *adminRepository) ExistsByUsername(ctx context.Context, username string, excludeID ...uint) (bool, error) {
	query := r.getDB(ctx).Model(&systemEntity.Admin{}).Where("username = ?", username)
	if len(excludeID) > 0 {
		query = query.Where("id <> ?", excludeID[0])
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func (r *adminRepository) List(ctx context.Context, query *AdminRepoQuery) ([]systemEntity.Admin, int64, error) {
	if query.Current <= 0 {
		query.Current = 1
	}
	if query.Size <= 0 {
		query.Size = entity.DefaultPageSize
	}

	var admins []systemEntity.Admin
	var total int64

	db := r.getDB(ctx).Model(&systemEntity.Admin{})

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
	return r.getDB(ctx).Save(admin).Error
}

func (r *adminRepository) UpdateLastLoginAt(ctx context.Context, id uint, lastLoginAt string) error {
	return r.getDB(ctx).Model(&systemEntity.Admin{}).
		Where("id = ?", id).
		UpdateColumn("last_login_at", lastLoginAt).Error
}

// IncrementTokenVersion 原子递增管理员 token_version（BUG #5）。
func (r *adminRepository) IncrementTokenVersion(ctx context.Context, id uint) error {
	return r.getDB(ctx).Model(&systemEntity.Admin{}).
		Where("id = ?", id).
		UpdateColumn("token_version", gorm.Expr("token_version + ?", 1)).Error
}

// ClearRoles 清理 admin_user_roles 关联表中指定管理员的全部角色关联。
// 用于 Service 层 TM 事务编排（Delete/DeleteBatch），支持 context 传播事务。
// 使用原生 SQL 直接删除关联表行，避免加载实体再 Association().Clear() 的额外查询开销。
func (r *adminRepository) ClearRoles(ctx context.Context, adminID uint) error {
	return r.getDB(ctx).
		Table("admin_user_roles").
		Where("admin_user_id = ?", adminID).
		Delete(nil).Error
}

// Delete 软删除管理员主数据。
// 注意：本方法仅删除主数据，不再清理 admin_user_roles 关联表（清理职责已上移到 Service 层 TM 事务）。
// Service 层 Delete/DeleteBatch 应在 TM 事务内先调 ClearRoles 再调 Delete，保证原子性。
func (r *adminRepository) Delete(ctx context.Context, id uint) error {
	return r.getDB(ctx).Delete(&systemEntity.Admin{}, id).Error
}

// UpdateRoles 替换管理员的角色关联。
// 不再自管理事务：当 ctx 携带 TM 事务句柄时复用外层事务，否则走连接池。
// 调用方（Service 层）需在需要原子性时自行用 TM 事务包裹。
func (r *adminRepository) UpdateRoles(ctx context.Context, adminID uint, roleIDs []uint) error {
	db := r.getDB(ctx)
	var admin systemEntity.Admin
	if err := db.First(&admin, adminID).Error; err != nil {
		return err
	}

	var roles []systemEntity.Role
	if len(roleIDs) > 0 {
		if err := db.Find(&roles, roleIDs).Error; err != nil {
			return err
		}
	}

	return db.Model(&admin).Association("Roles").Replace(roles)
}
