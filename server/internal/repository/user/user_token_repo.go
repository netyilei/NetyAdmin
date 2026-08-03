package user

import (
	"context"
	"time"

	"gorm.io/gorm"

	userEntity "NetyAdmin/internal/domain/entity/user"
	"NetyAdmin/internal/pkg/database"
)

// UserTokenRepository manages client-side multi-device sessions (user_tokens table).
// One row per (user_id, platform); each Login UPSERTs the row and atomically
// increments TokenVersion, invalidating older tokens on the SAME platform only
// (per-platform "kick-in" — other platforms keep working).
type UserTokenRepository interface {
	// UpsertAndIncrement inserts a new (user_id, platform) row with token_version=1,
	// or increments the existing row's token_version by 1 and overwrites the hashes.
	// Returns the resulting token_version (always >= 1).
	// Atomic via ON CONFLICT … DO UPDATE (PG row lock on the unique row).
	UpsertAndIncrement(ctx context.Context, t *userEntity.UserToken) (uint64, error)
	// GetByPlatform returns the (user_id, platform) row, or (nil, gorm.ErrRecordNotFound).
	GetByPlatform(ctx context.Context, userID, platform string) (*userEntity.UserToken, error)
	// UpdateAccessHash overwrites the access_hash / access_expires_at of the
	// (user_id, platform) row WITHOUT bumping token_version. Used by RefreshToken
	// to keep the session alive on the same device.
	UpdateAccessHash(ctx context.Context, userID, platform, accessHash string, accessExpiresAt time.Time) error
	// UpdateHashes overwrites access_hash + refresh_hash + both expiries WITHOUT
	// bumping token_version. Used by Login right after UpsertAndIncrement to
	// persist the just-issued token hashes on the same row that was bumped.
	UpdateHashes(ctx context.Context, userID, platform, accessHash, refreshHash string, accessExpiresAt, refreshExpiresAt time.Time) error
	// ClearHashes clears access_hash + refresh_hash of the (user_id, platform) row
	// on Logout so the next request fails tokenStore-independent hash validation.
	ClearHashes(ctx context.Context, userID, platform string) error
	// DeleteExpired removes rows whose access_expires_at < NOW() (and refresh has
	// also expired). Used by the cleanup job to bound table growth.
	DeleteExpired(ctx context.Context) (int64, error)
}

type userTokenRepository struct {
	db *gorm.DB
}

func NewUserTokenRepository(db *gorm.DB) UserTokenRepository {
	return &userTokenRepository{db: db}
}

func (r *userTokenRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

// UpsertAndIncrement uses raw SQL so the increment and hash overwrite run in one
// statement (no read-modify-write race). Returning gives us the new version.
func (r *userTokenRepository) UpsertAndIncrement(ctx context.Context, t *userEntity.UserToken) (uint64, error) {
	var version uint64
	err := r.getDB(ctx).Raw(`
INSERT INTO user_tokens (user_id, platform, token_version, access_hash, refresh_hash, access_expires_at, refresh_expires_at)
VALUES (?, ?, 1, ?, ?, ?, ?)
ON CONFLICT (user_id, platform) DO UPDATE SET
    token_version      = user_tokens.token_version + 1,
    access_hash        = EXCLUDED.access_hash,
    refresh_hash       = EXCLUDED.refresh_hash,
    access_expires_at  = EXCLUDED.access_expires_at,
    refresh_expires_at = EXCLUDED.refresh_expires_at,
    updated_at         = NOW()
RETURNING token_version`,
		t.UserID, t.Platform, t.AccessHash, t.RefreshHash, t.AccessExpiresAt, t.RefreshExpiresAt,
	).Row().Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func (r *userTokenRepository) GetByPlatform(ctx context.Context, userID, platform string) (*userEntity.UserToken, error) {
	var t userEntity.UserToken
	if err := r.getDB(ctx).Where("user_id = ? AND platform = ?", userID, platform).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *userTokenRepository) UpdateAccessHash(ctx context.Context, userID, platform, accessHash string, accessExpiresAt time.Time) error {
	return r.getDB(ctx).Model(&userEntity.UserToken{}).
		Where("user_id = ? AND platform = ?", userID, platform).
		Updates(map[string]interface{}{
			"access_hash":       accessHash,
			"access_expires_at": accessExpiresAt,
			"updated_at":        gorm.Expr("NOW()"),
		}).Error
}

func (r *userTokenRepository) UpdateHashes(ctx context.Context, userID, platform, accessHash, refreshHash string, accessExpiresAt, refreshExpiresAt time.Time) error {
	return r.getDB(ctx).Model(&userEntity.UserToken{}).
		Where("user_id = ? AND platform = ?", userID, platform).
		Updates(map[string]interface{}{
			"access_hash":        accessHash,
			"refresh_hash":       refreshHash,
			"access_expires_at":  accessExpiresAt,
			"refresh_expires_at": refreshExpiresAt,
			"updated_at":         gorm.Expr("NOW()"),
		}).Error
}

func (r *userTokenRepository) ClearHashes(ctx context.Context, userID, platform string) error {
	return r.getDB(ctx).Model(&userEntity.UserToken{}).
		Where("user_id = ? AND platform = ?", userID, platform).
		Updates(map[string]interface{}{
			"access_hash":  "",
			"refresh_hash": "",
			"updated_at":   gorm.Expr("NOW()"),
		}).Error
}

func (r *userTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	// A row is fully expired when BOTH tokens are past their expiry (or NULL access expiry,
	// treated defensively as "expired" since a real session always has an access expiry).
	res := r.getDB(ctx).
		Where("access_expires_at IS NULL OR (access_expires_at < NOW() AND (refresh_expires_at IS NULL OR refresh_expires_at < NOW()))").
		Delete(&userEntity.UserToken{})
	return res.RowsAffected, res.Error
}
