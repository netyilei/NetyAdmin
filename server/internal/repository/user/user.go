package user

import (
	"context"

	"gorm.io/gorm"

	userEntity "NetyAdmin/internal/domain/entity/user"
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

	// Token Hash 相关
	CreateTokenHash(ctx context.Context, hash *userEntity.UserTokenHash) error
	GetTokenHash(ctx context.Context, userID, tokenHash string) (*userEntity.UserTokenHash, error)
	DeleteTokenHash(ctx context.Context, userID, tokenHash string) error
	DeleteAllTokenHashes(ctx context.Context, userID string) error
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

func (r *userRepository) Create(ctx context.Context, user *userEntity.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*userEntity.User, error) {
	var user userEntity.User
	if err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*userEntity.User, error) {
	var user userEntity.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByPhone(ctx context.Context, phone string) (*userEntity.User, error) {
	var user userEntity.User
	if err := r.db.WithContext(ctx).Where("phone = ?", phone).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*userEntity.User, error) {
	var user userEntity.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) ExistsByUsername(ctx context.Context, username string, excludeID ...string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&userEntity.User{}).Where("username = ?", username)
	if len(excludeID) > 0 {
		query = query.Where("id <> ?", excludeID[0])
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func (r *userRepository) ExistsByPhone(ctx context.Context, phone string, excludeID ...string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&userEntity.User{}).Where("phone = ?", phone)
	if len(excludeID) > 0 {
		query = query.Where("id <> ?", excludeID[0])
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func (r *userRepository) ExistsByEmail(ctx context.Context, email string, excludeID ...string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&userEntity.User{}).Where("email = ?", email)
	if len(excludeID) > 0 {
		query = query.Where("id <> ?", excludeID[0])
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func (r *userRepository) List(ctx context.Context, query *UserRepoQuery) ([]userEntity.User, int64, error) {
	var users []userEntity.User
	var total int64

	db := r.db.WithContext(ctx).Model(&userEntity.User{})

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

	if query.Current > 0 && query.Size > 0 {
		db = db.Offset((query.Current - 1) * query.Size).Limit(query.Size)
	}

	if err := db.Order("created_at DESC").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *userRepository) SearchForAutocomplete(ctx context.Context, keyword string, limit int) ([]userEntity.User, error) {
	var users []userEntity.User
	db := r.db.WithContext(ctx).Model(&userEntity.User{}).
		Where("username LIKE ? OR email LIKE ? OR phone LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%").
		Limit(limit).
		Order("id DESC")
	if err := db.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *userRepository) Update(ctx context.Context, user *userEntity.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&userEntity.User{}, "id = ?", id).Error
}

func (r *userRepository) DeleteBatch(ctx context.Context, ids []string) error {
	return r.db.WithContext(ctx).Delete(&userEntity.User{}, "id IN ?", ids).Error
}

func (r *userRepository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&userEntity.User{}).Where("id = ?", id).Updates(fields).Error
}

func (r *userRepository) CreateTokenHash(ctx context.Context, hash *userEntity.UserTokenHash) error {
	return r.db.WithContext(ctx).Create(hash).Error
}

func (r *userRepository) GetTokenHash(ctx context.Context, userID, tokenHash string) (*userEntity.UserTokenHash, error) {
	var hash userEntity.UserTokenHash
	if err := r.db.WithContext(ctx).Where("user_id = ? AND token_hash = ?", userID, tokenHash).First(&hash).Error; err != nil {
		return nil, err
	}
	return &hash, nil
}

func (r *userRepository) DeleteTokenHash(ctx context.Context, userID, tokenHash string) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND token_hash = ?", userID, tokenHash).Delete(&userEntity.UserTokenHash{}).Error
}

func (r *userRepository) DeleteAllTokenHashes(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&userEntity.UserTokenHash{}).Error
}
