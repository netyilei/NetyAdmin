package user

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	userEntity "NetyAdmin/internal/domain/entity/user"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/pagination"
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
	Delete(ctx context.Context, id string) error
	DeleteBatch(ctx context.Context, ids []string) error
	UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error
	// IncrementTokenVersion 原子递增用户的 token_version（BUG #5）。
	// 用于改密/禁用/删除等敏感操作，使旧 token 携带的版本号失效。
	// 支持 context 传播事务：若 ctx 中携带 *Tx 则复用事务句柄。
	IncrementTokenVersion(ctx context.Context, id string) error

	// Token Hash 相关
	CreateTokenHash(ctx context.Context, hash *userEntity.UserTokenHash) error
	GetTokenHash(ctx context.Context, userID, tokenHash string) (*userEntity.UserTokenHash, error)
	DeleteTokenHash(ctx context.Context, userID, tokenHash string) error
	DeleteAllTokenHashes(ctx context.Context, userID string) error
	// DeleteExpiredTokenHashes 物理删除所有已过期的 token hash 记录（expired_at < NOW()）。
	// 返回受影响行数。供 token_hash_cleanup Job 定时调用，避免 user_token_hashes 表无限堆积。
	DeleteExpiredTokenHashes(ctx context.Context) (int64, error)
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
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// getDB 从 context 中获取事务 DB，若不存在则回退到默认 db。
func (r *userRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *userRepository) Create(ctx context.Context, user *userEntity.User) error {
	return r.getDB(ctx).Create(user).Error
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*userEntity.User, error) {
	var user userEntity.User
	if err := r.getDB(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*userEntity.User, error) {
	var user userEntity.User
	if err := r.getDB(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByPhone(ctx context.Context, phone string) (*userEntity.User, error) {
	var user userEntity.User
	if err := r.getDB(ctx).Where("phone = ?", phone).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*userEntity.User, error) {
	var user userEntity.User
	if err := r.getDB(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
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
	query := r.getDB(ctx).Model(&userEntity.User{}).Where("phone = ?", phone)
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

	db = db.Scopes(pagination.Paginate(query.Current, query.Size))

	if err := db.Order("created_at DESC").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *userRepository) SearchForAutocomplete(ctx context.Context, keyword string, limit int) ([]userEntity.User, error) {
	var users []userEntity.User
	db := r.getDB(ctx).Model(&userEntity.User{}).
		Where("username LIKE ? OR email LIKE ? OR phone LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%").
		Limit(limit).
		Order("id DESC")
	if err := db.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *userRepository) Update(ctx context.Context, user *userEntity.User) error {
	return r.getDB(ctx).Save(user).Error
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

// IncrementTokenVersion 原子递增用户 token_version（BUG #5）。
// 使用 UpdateColumn + gorm.Expr 保证并发安全，避免 Save 全字段覆盖。
func (r *userRepository) IncrementTokenVersion(ctx context.Context, id string) error {
	return r.getDB(ctx).Model(&userEntity.User{}).
		Where("id = ?", id).
		UpdateColumn("token_version", gorm.Expr("token_version + ?", 1)).Error
}

func (r *userRepository) CreateTokenHash(ctx context.Context, hash *userEntity.UserTokenHash) error {
	return r.getDB(ctx).Create(hash).Error
}

func (r *userRepository) GetTokenHash(ctx context.Context, userID, tokenHash string) (*userEntity.UserTokenHash, error) {
	var hash userEntity.UserTokenHash
	if err := r.getDB(ctx).Where("user_id = ? AND token_hash = ?", userID, tokenHash).First(&hash).Error; err != nil {
		return nil, err
	}
	return &hash, nil
}

func (r *userRepository) DeleteTokenHash(ctx context.Context, userID, tokenHash string) error {
	return r.getDB(ctx).Where("user_id = ? AND token_hash = ?", userID, tokenHash).Delete(&userEntity.UserTokenHash{}).Error
}

func (r *userRepository) DeleteAllTokenHashes(ctx context.Context, userID string) error {
	return r.getDB(ctx).Where("user_id = ?", userID).Delete(&userEntity.UserTokenHash{}).Error
}

// DeleteExpiredTokenHashes 物理删除所有 expired_at < NOW() 的 token hash 记录。
// user_token_hashes 表无 soft_delete 字段，Delete 即硬删除。
// 利用 idx_user_token_expired 索引加速范围扫描。
func (r *userRepository) DeleteExpiredTokenHashes(ctx context.Context) (int64, error) {
	res := r.getDB(ctx).Where("expired_at < NOW()", nil).Delete(&userEntity.UserTokenHash{})
	return res.RowsAffected, res.Error
}
