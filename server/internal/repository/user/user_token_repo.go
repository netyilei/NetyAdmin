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
	// UpdateAccessHash overwrites the access_hash / access_expires_at WITHOUT
	// bumping token_version, but ONLY while the row's token_version still equals
	// expectedTokenVersion (optimistic guard: a concurrent Login bumping the
	// version makes this write a no-op error instead of clobbering its hashes).
	// PRECONDITION: the caller read the row (GetByPlatform / UpsertAndIncrement)
	// and passes the version it observed. Returns gorm.ErrRecordNotFound when
	// the row is missing or the version moved on.
	UpdateAccessHash(ctx context.Context, userID, platform string, expectedTokenVersion uint64, accessHash string, accessExpiresAt time.Time) error
	// UpdateHashes overwrites access_hash + refresh_hash + both expiries WITHOUT
	// bumping token_version, but ONLY when the row is still exactly the state
	// the caller validated: token_version == expectedTokenVersion AND
	// refresh_hash == expectedRefreshHash. This optimistic guard makes
	// concurrent session mutations fail loudly instead of clobbering each
	// other: a concurrent Login bumps the version, a Logout (or a competing
	// refresh that won the rotation) changes the refresh hash — any of them
	// turns this write into a 0-row result.
	//   - Login passes the version returned by UpsertAndIncrement and "" (the
	//     row it just upserted carries empty hashes).
	//   - RefreshToken passes the (TokenVersion, RefreshHash) it validated.
	// Returns gorm.ErrRecordNotFound when the row is missing OR moved on —
	// callers MUST fail closed (a blind overwrite would corrupt the winning
	// session's hashes).
	UpdateHashes(ctx context.Context, userID, platform string, expectedTokenVersion uint64, expectedRefreshHash, accessHash, refreshHash string, accessExpiresAt, refreshExpiresAt time.Time) error
	// ClearHashes clears access_hash + refresh_hash of the (user_id, platform)
	// row on Logout, but ONLY while the row still holds expectedAccessHash —
	// i.e. the caller is logging out the session that currently owns the row.
	// If a newer login/refresh already replaced the hashes, this stale logout
	// is a no-op: it can never wipe a newer session's hashes.
	// Idempotent: 0-row (row missing, session replaced, or already cleared)
	// is SUCCESS, not an error.
	ClearHashes(ctx context.Context, userID, platform, expectedAccessHash string) error
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

func (r *userTokenRepository) UpdateAccessHash(ctx context.Context, userID, platform string, expectedTokenVersion uint64, accessHash string, accessExpiresAt time.Time) error {
	res := r.getDB(ctx).Model(&userEntity.UserToken{}).
		Where("user_id = ? AND platform = ? AND token_version = ?", userID, platform, expectedTokenVersion).
		Updates(map[string]interface{}{
			"access_hash":       accessHash,
			"access_expires_at": accessExpiresAt,
			"updated_at":        gorm.Expr("NOW()"),
		})
	return requireUpdatedRow(res)
}

func (r *userTokenRepository) UpdateHashes(ctx context.Context, userID, platform string, expectedTokenVersion uint64, expectedRefreshHash, accessHash, refreshHash string, accessExpiresAt, refreshExpiresAt time.Time) error {
	res := r.getDB(ctx).Model(&userEntity.UserToken{}).
		Where("user_id = ? AND platform = ? AND token_version = ? AND refresh_hash = ?",
			userID, platform, expectedTokenVersion, expectedRefreshHash).
		Updates(map[string]interface{}{
			"access_hash":        accessHash,
			"refresh_hash":       refreshHash,
			"access_expires_at":  accessExpiresAt,
			"refresh_expires_at": refreshExpiresAt,
			"updated_at":         gorm.Expr("NOW()"),
		})
	return requireUpdatedRow(res)
}

// requireUpdatedRow 要求 UPDATE 至少命中一行。
//
// 纯 UPDATE 命中 0 行不会返回错误，但数据也未写入——调用方会误以为
// token hash 已落库（注册自动登录未建 user_tokens 行的事故根因）。
// 按 Repository 红线返回原始 GORM 错误，由 Service 层决定业务语义。
func requireUpdatedRow(res *gorm.DB) error {
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *userTokenRepository) ClearHashes(ctx context.Context, userID, platform, expectedAccessHash string) error {
	// 幂等：0 行（行不存在 / 会话已被新登录或刷新替换 / 已清理过）视为成功。
	// WHERE access_hash = ? 保证登出动作只清"当前仍属于该会话"的行，
	// 并发的新 Login/Refresh 写入的 hash 不会被旧 token 的登出误清。
	return r.getDB(ctx).Model(&userEntity.UserToken{}).
		Where("user_id = ? AND platform = ? AND access_hash = ?", userID, platform, expectedAccessHash).
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
