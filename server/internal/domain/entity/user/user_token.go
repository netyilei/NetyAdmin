package user

import "time"

// UserToken stores a single client-side session, one row per (user_id, platform).
//
// Design — per-platform "kick-in" semantics:
//   - Each Login UPSERTs the row and atomically increments TokenVersion;
//     the new token carries the new version in its claims (PlatTokenVersion / "ptv").
//   - The middleware compares claims.PlatTokenVersion < DB TokenVersion → old session
//     of the SAME platform is rejected, while sessions on other platforms keep working
//     (e.g. logging in on mobile kicks the previous mobile session but leaves web alone).
//
// TokenVersion is independent of users.token_version: admin-side sensitive operations
// (password change / disable / delete) still bump users.token_version to invalidate
// every platform at once; both checks run in the middleware.
type UserToken struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	UserID           string     `gorm:"column:user_id;size:26;not null;index" json:"userId"`
	Platform         string     `gorm:"column:platform;size:50;not null;uniqueIndex:uniq_user_tokens_user_id_platform,priority:2" json:"platform"`
	TokenVersion     uint64     `gorm:"column:token_version;not null;default:0" json:"tokenVersion"`
	AccessHash       string     `gorm:"column:access_hash;size:64;not null;default:''" json:"-"`
	RefreshHash      string     `gorm:"column:refresh_hash;size:64;not null;default:''" json:"-"`
	AccessExpiresAt  *time.Time `gorm:"column:access_expires_at" json:"accessExpiresAt"`
	RefreshExpiresAt *time.Time `gorm:"column:refresh_expires_at" json:"refreshExpiresAt"`
	CreatedAt        time.Time  `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (UserToken) TableName() string {
	return "user_tokens"
}
