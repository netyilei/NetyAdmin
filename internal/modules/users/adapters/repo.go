// repo.go：users 持久化（PG——用户表；email/username/phone 唯一约束查重）。
package adapters

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/modules/users/domain"
)

// UserRepo 实现 application.UserRepo（PG）。
type UserRepo struct {
	pool *database.Pool
}

// NewUserRepo 创建。
func NewUserRepo(pool *database.Pool) *UserRepo { return &UserRepo{pool: pool} }

const userCols = `id, email, username, phone, password_hash, status, token_version, created_at, updated_at`

func scanUser(row interface{ Scan(...any) error }) (*domain.User, error) {
	var u domain.User
	if err := row.Scan(&u.ID, &u.Email, &u.Username, &u.Phone, &u.PasswordHash,
		&u.Status, &u.TokenVersion, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

// isUnique 判定唯一约束冲突（PG 23505）。
func isUnique(err error) bool { return database.IsUniqueViolation(err) }

// notFound 精确分流（R3/D1——DB 故障不得伪装 NotFound；storage isNoRows 同模式）。
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrUserNotFound
	}
	return err
}

// Create 实现 application.UserRepo。
func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO users
		(id, email, username, phone, password_hash, status, token_version, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		u.ID, u.Email, u.Username, u.Phone, u.PasswordHash, u.Status, u.TokenVersion, u.CreatedAt, u.UpdatedAt)
	if isUnique(err) {
		if strings.Contains(err.Error(), "uk_users_username") {
			return domain.ErrUsernameTaken
		}
		return domain.ErrEmailTaken
	}
	if err != nil {
		return wrap("create user", err)
	}
	return nil
}

// GetByID 实现 application.UserRepo。
func (r *UserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE id=$1`, id))
	if err != nil {
		return nil, notFound(err) // R3/D1：pgx.ErrNoRows 精确分流——DB 故障原样上抛
	}
	return u, nil
}

// GetByEmail 实现 application.UserRepo。
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE email=$1`, email))
	if err != nil {
		return nil, notFound(err) // R3/D1：pgx.ErrNoRows 精确分流——DB 故障原样上抛
	}
	return u, nil
}

// GetByUsername 实现 application.UserRepo。
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE username=$1`, username))
	if err != nil {
		return nil, notFound(err) // R3/D1：pgx.ErrNoRows 精确分流——DB 故障原样上抛
	}
	return u, nil
}

// GetByPhone 实现 application.UserRepo。
func (r *UserRepo) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE phone=$1`, phone))
	if err != nil {
		return nil, notFound(err) // R3/D1：pgx.ErrNoRows 精确分流——DB 故障原样上抛
	}
	return u, nil
}

// UpdateUsername 实现 application.UserRepo。
func (r *UserRepo) UpdateUsername(ctx context.Context, id, username string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET username=$2, updated_at=now() WHERE id=$1`, id, username)
	if err != nil {
		if isUnique(err) {
			return domain.ErrUsernameTaken
		}
		return wrap("update username", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// UpdatePhone 实现 application.UserRepo。
func (r *UserRepo) UpdatePhone(ctx context.Context, id, phone string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET phone=$2, updated_at=now() WHERE id=$1`, id, phone)
	if err != nil {
		if isUnique(err) {
			return domain.ErrPhoneTaken
		}
		return wrap("update phone", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// SetStatus 实现 application.UserRepo。
func (r *UserRepo) SetStatus(ctx context.Context, id string, status int16) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET status=$2, updated_at=now() WHERE id=$1`, id, status)
	if err != nil {
		return wrap("set status", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// SetTokenVersion 实现 application.UserRepo（版本推进——禁用/敏感变更）。
func (r *UserRepo) SetTokenVersion(ctx context.Context, id string, version int64) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET token_version=$2, updated_at=now() WHERE id=$1`, id, version)
	if err != nil {
		return wrap("set token version", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// List 实现 application.UserRepo。
func (r *UserRepo) List(ctx context.Context, offset, limit int) ([]*domain.User, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, wrap("count users", err)
	}
	rows, err := r.pool.Query(ctx, `SELECT `+userCols+` FROM users
		ORDER BY created_at DESC OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return nil, 0, wrap("list users", err)
	}
	defer rows.Close()
	var out []*domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

func wrap(op string, err error) error {
	return fmt.Errorf("users: %s: %w", op, err)
}
