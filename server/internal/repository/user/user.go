package user

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	userEntity "NetyAdmin/internal/domain/entity/user"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/like"
	"NetyAdmin/internal/pkg/pagination"
	"NetyAdmin/internal/pkg/pii"
)

type UserRepository interface {
	Create(ctx context.Context, user *userEntity.User) error
	GetByID(ctx context.Context, id string) (*userEntity.User, error)
	GetByUsername(ctx context.Context, username string) (*userEntity.User, error)
	GetByPhone(ctx context.Context, phone string) (*userEntity.User, error)
	GetByEmail(ctx context.Context, email string) (*userEntity.User, error)
	ExistsByUsername(ctx context.Context, username string, excludeID ...string) (bool, error)
	ExistsByPhone(ctx context.Context, phone string, excludeID ...string) (bool, error)
	ExistsByEmail(ctx context.Context, email string, excludeID ...string) (bool, error)
	List(ctx context.Context, query *UserRepoQuery) ([]userEntity.User, int64, error)
	SearchForAutocomplete(ctx context.Context, keyword string, limit int) ([]userEntity.User, error)
	Update(ctx context.Context, user *userEntity.User) error
	// UpdatePhone 换手机号专用（repo 内加密，service 不接触密钥/列名）。
	UpdatePhone(ctx context.Context, id string, phone string) error
	Delete(ctx context.Context, id string) error
	DeleteBatch(ctx context.Context, ids []string) error
	UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error
	// IncrementTokenVersion 原子递增用户的 token_version（BUG #5）。
	// 用于改密/禁用/删除等敏感操作，使旧 token 携带的版本号失效。
	// 支持 context 传播事务：若 ctx 中携带 *Tx 则复用事务句柄。
	IncrementTokenVersion(ctx context.Context, id string) error

	// AdminToken (admin-side session storage) — typed AdminToken maps to admin_tokens table.
	CreateTokenHash(ctx context.Context, hash *userEntity.AdminToken) error
	GetTokenHash(ctx context.Context, userID, tokenHash string) (*userEntity.AdminToken, error)
	DeleteTokenHash(ctx context.Context, userID, tokenHash string) error
	DeleteAllTokenHashes(ctx context.Context, userID string) error
	// DeleteExpiredTokenHashes 物理删除所有 expired_at < NOW() 的 admin token 记录。
	// 返回受影响行数。供 token_hash_cleanup Job 定时调用，避免 admin_tokens 表无限堆积。
	DeleteExpiredTokenHashes(ctx context.Context) (int64, error)

	// --- OAuth binding ---

	// FindOAuthBinding looks up a third-party binding by provider + openid.
	// Returns gorm.ErrRecordNotFound if not bound.
	FindOAuthBinding(ctx context.Context, provider, openid string) (*userEntity.UserOAuthBinding, error)
	// FindOAuthBindingByUnionID looks up by provider + unionid (WeChat unionid scenario).
	FindOAuthBindingByUnionID(ctx context.Context, provider, unionid string) (*userEntity.UserOAuthBinding, error)
	// FindOAuthBindingByUserProvider looks up by userID + provider.
	// Used by Unbind to find the binding (for cache invalidation) before deleting.
	FindOAuthBindingByUserProvider(ctx context.Context, userID, provider string) (*userEntity.UserOAuthBinding, error)
	// CreateOAuthBinding inserts a new OAuth binding record.
	CreateOAuthBinding(ctx context.Context, binding *userEntity.UserOAuthBinding) error
	// DeleteOAuthBinding removes a binding by userID + provider (unbind).
	DeleteOAuthBinding(ctx context.Context, userID, provider string) error
	// ListOAuthBindings returns all OAuth bindings for a user.
	ListOAuthBindings(ctx context.Context, userID string) ([]userEntity.UserOAuthBinding, error)
}

type UserRepoQuery struct {
	Current  int
	Size     int
	Username string
	Nickname string
	Gender   *string
	Phone    string
	Email    string
	Status   *string
}

type userRepository struct {
	db       *gorm.DB
	phonePII *pii.Cipher
}

func NewUserRepository(db *gorm.DB, phonePII *pii.Cipher) UserRepository {
	return &userRepository{db: db, phonePII: phonePII}
}

// getDB 从 context 中获取事务 DB，若不存在则回退到默认 db。
func (r *userRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

// fillPhone 读后解密回填：PhoneEnc → 实体 Phone（明文，出口由 service 统一掩码）。
// 解密失败（密钥轮换/脏数据）按空号降级并告警——单行脏数据不得阻断列表查询；
// 唯一性/检索走盲索引列，不受解密失败影响。
func (r *userRepository) fillPhone(u *userEntity.User) {
	if u == nil || u.PhoneEnc == "" {
		return
	}
	plain, err := r.phonePII.Open(u.PhoneEnc)
	if err != nil {
		slog.Error("decrypt users.phone failed, degrade to empty", "userID", u.ID, "err", err)
		return
	}
	u.Phone = plain
}

func (r *userRepository) fillPhones(users []userEntity.User) {
	for i := range users {
		r.fillPhone(&users[i])
	}
}

// sealPhone 写前加密：明文 → (phone_enc, phone_idx, phone_tail4) 三列，
// 并把明文列置空（phone 明文列已退役，唯一写明文的只剩惰性迁移任务）。
// 已掩码值不应到达 repo（service 层防御），此处防御性保留三列原值。
func (r *userRepository) sealPhone(u *userEntity.User) error {
	if pii.IsMasked(u.Phone) {
		u.Phone = ""
		return nil
	}
	enc, idx, tail4, err := r.phonePII.Seal(u.Phone)
	if err != nil {
		return err
	}
	u.PhoneEnc, u.PhoneIdx, u.PhoneTail4 = enc, idx, tail4
	u.Phone = ""
	return nil
}

func (r *userRepository) Create(ctx context.Context, user *userEntity.User) error {
	if err := r.sealPhone(user); err != nil {
		return err
	}
	return r.getDB(ctx).Create(user).Error
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*userEntity.User, error) {
	var user userEntity.User
	if err := r.getDB(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	r.fillPhone(&user)
	return &user, nil
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*userEntity.User, error) {
	var user userEntity.User
	if err := r.getDB(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	r.fillPhone(&user)
	return &user, nil
}

func (r *userRepository) GetByPhone(ctx context.Context, phone string) (*userEntity.User, error) {
	var user userEntity.User
	// 精确匹配走盲索引列（明文列已退役）
	if err := r.getDB(ctx).Where("phone_idx = ?", r.phonePII.Index(phone)).First(&user).Error; err != nil {
		return nil, err
	}
	r.fillPhone(&user)
	return &user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*userEntity.User, error) {
	var user userEntity.User
	if err := r.getDB(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	r.fillPhone(&user)
	return &user, nil
}

func (r *userRepository) ExistsByUsername(ctx context.Context, username string, excludeID ...string) (bool, error) {
	query := r.getDB(ctx).Model(&userEntity.User{}).Where("username = ?", username)
	if len(excludeID) > 0 {
		query = query.Where("id <> ?", excludeID[0])
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func (r *userRepository) ExistsByPhone(ctx context.Context, phone string, excludeID ...string) (bool, error) {
	query := r.getDB(ctx).Model(&userEntity.User{}).Where("phone_idx = ?", r.phonePII.Index(phone))
	if len(excludeID) > 0 {
		query = query.Where("id <> ?", excludeID[0])
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func (r *userRepository) ExistsByEmail(ctx context.Context, email string, excludeID ...string) (bool, error) {
	query := r.getDB(ctx).Model(&userEntity.User{}).Where("email = ?", email)
	if len(excludeID) > 0 {
		query = query.Where("id <> ?", excludeID[0])
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func (r *userRepository) List(ctx context.Context, query *UserRepoQuery) ([]userEntity.User, int64, error) {
	if query.Current <= 0 {
		query.Current = 1
	}
	if query.Size <= 0 {
		query.Size = entity.DefaultPageSize
	}

	var users []userEntity.User
	var total int64

	db := r.getDB(ctx).Model(&userEntity.User{})

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
		cond, arg, ok := phoneSearchCondition(r.phonePII, query.Phone)
		if ok {
			db = db.Where(cond, arg)
		}
	}
	if query.Email != "" {
		db = db.Where("email LIKE ?", like.LikeContains(query.Email))
	}
	if query.Status != nil && *query.Status != "" {
		db = db.Where("status = ?", *query.Status)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	db = db.Scopes(pagination.Paginate(query.Current, query.Size))

	if err := db.Order("created_at DESC").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	r.fillPhones(users)
	return users, total, nil
}

func (r *userRepository) SearchForAutocomplete(ctx context.Context, keyword string, limit int) ([]userEntity.User, error) {
	var users []userEntity.User
	db := r.getDB(ctx).Model(&userEntity.User{}).
		Where("username LIKE ? OR email LIKE ?", like.LikeContains(keyword), like.LikeContains(keyword))
	// keyword 为手机号形态（完整号/尾号）时追加盲索引/尾号精确匹配；
	// 中缀模糊在加密下不可恢复（见 pii 包注释）
	if _, arg, ok := phoneSearchCondition(r.phonePII, keyword); ok {
		db = db.Or("(phone_idx = ? OR phone_tail4 = ?)", arg, arg)
	}
	if err := db.Limit(limit).Order("id DESC").Find(&users).Error; err != nil {
		return nil, err
	}
	r.fillPhones(users)
	return users, nil
}

// Update 全字段更新用户（Omit token_version）。
//
// token_version 是会话失效的版本号，只能经 IncrementTokenVersion 原子递增，
// 禁止随 Save 回写：调用方传入的实体通常是事务外 GetByID 读出的旧快照，
// 若 Save 携带 token_version 会把同事务内 Increment 的结果覆盖回旧值，
// 导致改密/禁用后旧 JWT 永不失效（GORM Save 对主键非零实体是全字段 UPDATE）。
func (r *userRepository) Update(ctx context.Context, user *userEntity.User) error {
	if err := r.sealPhone(user); err != nil {
		return err
	}
	return r.getDB(ctx).Omit("token_version").Save(user).Error
}

func (r *userRepository) Delete(ctx context.Context, id string) error {
	return r.getDB(ctx).Delete(&userEntity.User{}, "id = ?", id).Error
}

func (r *userRepository) DeleteBatch(ctx context.Context, ids []string) error {
	return r.getDB(ctx).Delete(&userEntity.User{}, "id IN ?", ids).Error
}

func (r *userRepository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.getDB(ctx).Model(&userEntity.User{}).Where("id = ?", id).Updates(fields).Error
}

// UpdatePhone 换手机号专用：repo 内 Seal 并列级更新三列（明文列置空）。
// service 层不接触密钥与列名——加密是持久层细节。
// 并发换号竞态由 phone_idx 部分唯一索引兜底（冲突转唯一约束错误）。
func (r *userRepository) UpdatePhone(ctx context.Context, id string, phone string) error {
	enc, idx, tail4, err := r.phonePII.Seal(phone)
	if err != nil {
		return err
	}
	return r.getDB(ctx).Model(&userEntity.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"phone_enc":   enc,
		"phone_idx":   idx,
		"phone_tail4": tail4,
		"phone":       "",
	}).Error
}

// IncrementTokenVersion 原子递增用户 token_version（BUG #5）。
// 使用 UpdateColumn + gorm.Expr 保证并发安全，避免 Save 全字段覆盖。
func (r *userRepository) IncrementTokenVersion(ctx context.Context, id string) error {
	return r.getDB(ctx).Model(&userEntity.User{}).
		Where("id = ?", id).
		UpdateColumn("token_version", gorm.Expr("token_version + ?", 1)).Error
}

func (r *userRepository) CreateTokenHash(ctx context.Context, hash *userEntity.AdminToken) error {
	return r.getDB(ctx).Create(hash).Error
}

func (r *userRepository) GetTokenHash(ctx context.Context, userID, tokenHash string) (*userEntity.AdminToken, error) {
	var hash userEntity.AdminToken
	if err := r.getDB(ctx).Where("user_id = ? AND token_hash = ?", userID, tokenHash).First(&hash).Error; err != nil {
		return nil, err
	}
	return &hash, nil
}

func (r *userRepository) DeleteTokenHash(ctx context.Context, userID, tokenHash string) error {
	return r.getDB(ctx).Where("user_id = ? AND token_hash = ?", userID, tokenHash).Delete(&userEntity.AdminToken{}).Error
}

func (r *userRepository) DeleteAllTokenHashes(ctx context.Context, userID string) error {
	return r.getDB(ctx).Where("user_id = ?", userID).Delete(&userEntity.AdminToken{}).Error
}

// DeleteExpiredTokenHashes 物理删除所有 expired_at < NOW() 的 admin token 记录。
// admin_tokens 表无 soft_delete 字段，Delete 即硬删除。
// 利用 idx_admin_tokens_expired 索引加速范围扫描。
func (r *userRepository) DeleteExpiredTokenHashes(ctx context.Context) (int64, error) {
	res := r.getDB(ctx).Where("expired_at < NOW()", nil).Delete(&userEntity.AdminToken{})
	return res.RowsAffected, res.Error
}

// --- OAuth binding implementations ---

func (r *userRepository) FindOAuthBinding(ctx context.Context, provider, openid string) (*userEntity.UserOAuthBinding, error) {
	var binding userEntity.UserOAuthBinding
	if err := r.getDB(ctx).Where("provider = ? AND openid = ?", provider, openid).First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func (r *userRepository) FindOAuthBindingByUnionID(ctx context.Context, provider, unionid string) (*userEntity.UserOAuthBinding, error) {
	var binding userEntity.UserOAuthBinding
	if err := r.getDB(ctx).Where("provider = ? AND unionid = ?", provider, unionid).First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func (r *userRepository) FindOAuthBindingByUserProvider(ctx context.Context, userID, provider string) (*userEntity.UserOAuthBinding, error) {
	var binding userEntity.UserOAuthBinding
	if err := r.getDB(ctx).Where("user_id = ? AND provider = ?", userID, provider).First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func (r *userRepository) CreateOAuthBinding(ctx context.Context, binding *userEntity.UserOAuthBinding) error {
	return r.getDB(ctx).Create(binding).Error
}

func (r *userRepository) DeleteOAuthBinding(ctx context.Context, userID, provider string) error {
	return r.getDB(ctx).Where("user_id = ? AND provider = ?", userID, provider).Delete(&userEntity.UserOAuthBinding{}).Error
}

func (r *userRepository) ListOAuthBindings(ctx context.Context, userID string) ([]userEntity.UserOAuthBinding, error) {
	var bindings []userEntity.UserOAuthBinding
	if err := r.getDB(ctx).Where("user_id = ?", userID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

// phoneSearchCondition 把手机号筛选输入翻译为加密下的检索条件：
//   - 归一化后 11 位手机号 → 盲索引精确（"phone_idx = ?"）
//   - 4 位纯数字        → 尾号精确（"phone_tail4 = ?"）
//   - 其它形态          → 不支持（返回 ok=false，调用方跳过该条件——
//     中缀模糊在加密下无法安全保留，宁缺毋滥）
func phoneSearchCondition(c *pii.Cipher, input string) (cond string, arg string, ok bool) {
	p := pii.Normalize(input)
	if p == "" {
		return "", "", false
	}
	if len(p) == 4 && isAllDigits(p) {
		return "phone_tail4 = ?", p, true
	}
	if err := pii.Validate(p); err == nil {
		return "phone_idx = ?", c.Index(p), true
	}
	return "", "", false
}

func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}
