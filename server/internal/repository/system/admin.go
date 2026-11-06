package system

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/like"
	"NetyAdmin/internal/pkg/pagination"
	"NetyAdmin/internal/pkg/pii"
)

type AdminRepository interface {
	Create(ctx context.Context, admin *systemEntity.Admin) error
	GetByID(ctx context.Context, id uint) (*systemEntity.Admin, error)
	GetByUsername(ctx context.Context, username string) (*systemEntity.Admin, error)
	ExistsByUsername(ctx context.Context, username string, excludeID ...uint) (bool, error)
	List(ctx context.Context, query *AdminRepoQuery) ([]systemEntity.Admin, int64, error)
	Update(ctx context.Context, admin *systemEntity.Admin) error
	// UpdatePhone 管理员换手机号专用（repo 内加密，service 不接触密钥/列名）。
	UpdatePhone(ctx context.Context, id uint, phone string) error
	// UpdateEmail 管理员换邮箱专用（同 UpdatePhone 语义）。
	UpdateEmail(ctx context.Context, id uint, email string) error
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
	db       *gorm.DB
	phonePII *pii.Cipher
}

func NewAdminRepository(db *gorm.DB, phonePII *pii.Cipher) AdminRepository {
	return &adminRepository{db: db, phonePII: phonePII}
}

// fillPhone 读后解密回填（语义同 users repo：解密失败按空号降级）。
func (r *adminRepository) fillPhone(a *systemEntity.Admin) {
	if a == nil || a.PhoneEnc == "" {
		return
	}
	plain, err := r.phonePII.Open(a.PhoneEnc)
	if err != nil {
		slog.Error("decrypt admin_user.phone failed, degrade to empty", "adminID", a.ID, "err", err)
		return
	}
	a.Phone = plain
}

func (r *adminRepository) fillPII(a *systemEntity.Admin) {
	r.fillPhone(a)
	r.fillEmail(a)
}

func (r *adminRepository) fillPhones(admins []systemEntity.Admin) {
	for i := range admins {
		r.fillPII(&admins[i])
	}
}

// fillEmail 读后解密回填邮箱（语义同 users repo）。
func (r *adminRepository) fillEmail(a *systemEntity.Admin) {
	if a == nil || a.EmailEnc == "" {
		return
	}
	plain, err := r.phonePII.OpenEmail(a.EmailEnc)
	if err != nil {
		slog.Error("decrypt admin_user.email failed, degrade to empty", "adminID", a.ID, "err", err)
		return
	}
	a.Email = plain
}

// sealEmail 写前加密邮箱（掩码防御语义同 users repo）。
func (r *adminRepository) sealEmail(a *systemEntity.Admin) error {
	if pii.IsMasked(a.Email) {
		a.Email = ""
		return nil
	}
	// 空明文 = "未变更"（语义同 users repo：防解密失败静默清空）
	if a.Email == "" {
		return nil
	}
	enc, idx, err := r.phonePII.SealEmail(a.Email)
	if err != nil {
		return err
	}
	a.EmailEnc, a.EmailIdx = enc, idx
	a.Email = ""
	return nil
}

// sealPhone 写前加密（语义同 users repo：明文列退役，掩码值防御性保留三列）。
func (r *adminRepository) sealPhone(a *systemEntity.Admin) error {
	if pii.IsMasked(a.Phone) {
		a.Phone = ""
		return nil
	}
	// 空明文 = "未变更"（语义同 users repo：防解密失败静默清空）
	if a.Phone == "" {
		return nil
	}
	enc, idx, tail4, err := r.phonePII.Seal(a.Phone)
	if err != nil {
		return err
	}
	a.PhoneEnc, a.PhoneIdx, a.PhoneTail4 = enc, idx, tail4
	a.Phone = ""
	return nil
}

// getDB 取当前 context 下的 *gorm.DB：若 ctx 中存在事务句柄则复用事务，否则回退到 r.db。
func (r *adminRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *adminRepository) Create(ctx context.Context, admin *systemEntity.Admin) error {
	if err := r.sealPhone(admin); err != nil {
		return err
	}
	if err := r.sealEmail(admin); err != nil {
		return err
	}
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
	r.fillPII(&admin)
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
	r.fillPII(&admin)
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
		db = db.Where("username LIKE ?", like.LikeContains(query.Username))
	}
	if query.Nickname != "" {
		db = db.Where("nickname LIKE ?", like.LikeContains(query.Nickname))
	}
	if query.Gender != nil && *query.Gender != "" {
		db = db.Where("gender = ?", *query.Gender)
	}
	if query.Phone != "" {
		// 加密下检索语义 = 完整号精确（盲索引）/ 尾号 4 位精确（与 users 一致）
		p := pii.Normalize(query.Phone)
		switch {
		case len(p) == 4 && isAllDigitsSys(p):
			db = db.Where("phone_tail4 = ?", p)
		case pii.Validate(p) == nil:
			db = db.Where("phone_idx = ?", r.phonePII.Index(p))
		}
	}
	if query.Email != "" {
		if pii.IsValidEmail(query.Email) {
			db = db.Where("email_idx = ?", r.phonePII.IndexEmail(query.Email))
		}
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

	for i := range admins {
		r.fillPII(&admins[i])
	}
	return admins, total, nil
}

// Update 全字段更新管理员（Omit token_version）。
//
// token_version 只能经 IncrementTokenVersion 原子递增，禁止随 Save 回写
// 旧快照值（否则同事务内 Increment 的结果被覆盖，改密/禁用后旧 JWT 永不失效）。
func (r *adminRepository) Update(ctx context.Context, admin *systemEntity.Admin) error {
	if err := r.sealPhone(admin); err != nil {
		return err
	}
	if err := r.sealEmail(admin); err != nil {
		return err
	}
	return r.getDB(ctx).Omit("token_version").Save(admin).Error
}

// UpdateEmail 管理员换邮箱专用（repo 内 SealEmail，列级更新，明文列置空）。
func (r *adminRepository) UpdateEmail(ctx context.Context, id uint, email string) error {
	enc, idx, err := r.phonePII.SealEmail(email)
	if err != nil {
		return err
	}
	return r.getDB(ctx).Model(&systemEntity.Admin{}).Where("id = ?", id).Updates(map[string]interface{}{
		"email_enc": enc,
		"email_idx": idx,
		"email":     "",
	}).Error
}

// UpdatePhone 管理员换手机号专用（repo 内 Seal，列级更新，明文列置空）。
func (r *adminRepository) UpdatePhone(ctx context.Context, id uint, phone string) error {
	enc, idx, tail4, err := r.phonePII.Seal(phone)
	if err != nil {
		return err
	}
	return r.getDB(ctx).Model(&systemEntity.Admin{}).Where("id = ?", id).Updates(map[string]interface{}{
		"phone_enc":   enc,
		"phone_idx":   idx,
		"phone_tail4": tail4,
		"phone":       "",
	}).Error
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

// isAllDigitsSys 判断字符串是否全为数字（admin 列表尾号筛选用）。
func isAllDigitsSys(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}
