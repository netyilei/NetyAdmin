package user

import (
	"context"
	"log/slog"
	"time"

	"NetyAdmin/internal/domain/entity/user"
	"NetyAdmin/internal/pkg/cache"
)

// cachedUserTokenRepository wraps a UserTokenRepository with an L2 (Redis) cache layer.
//
// Design mirrors admin tokenStore: DB is the source of truth, Redis accelerates reads.
// Why L2-only (never L1): token data is security-sensitive — invalidation must take
// effect cluster-wide immediately (login kicks, logout, password change). L1 (BigCache)
// has a multi-node sync window that is unacceptable for security data.
//
// Cache granularity:
//   - Key:   KeyUserToken(userID, platform)  — one entry per (user, platform).
//   - Value: the UserToken row (token_version + access_hash), used by middleware for
//     both version check and hash check in one lookup.
//   - Tags:  TagUserTokenByUser(userID)              — invalidate all platforms of a user
//     (admin sensitive ops: password change / disable / delete).
//     TagUserTokenByPlatform(userID, platform) — invalidate one platform only
//     (login kick-in: same platform re-login).
//
// Invalidation contract: every write path (UpsertAndIncrement / UpdateHashes /
// ClearHashes / DeleteExpired) invalidates the affected entries. TTL is the safety net
// for PubSub cross-node invalidation lag (typically <100ms, TTL bounds worst case).
//
// Known TTL-bounded races (accepted, documented for maintainers):
//   - Read-revive: a concurrent Get may load the OLD row, miss the write's
//     invalidation, and back-fill the stale value — stale for up to the 30s TTL
//     (e.g. a logged-out token could pass hash check for ≤30s). Writes win on
//     the DB path; the middleware's version checks bound the impact.
//   - Rolling deploy: cache entries written by pre-entry-struct code lack the
//     hash fields, deserialize as empty strings, and fail the fail-closed hash
//     check until they expire (≤30s) — self-healing, no action needed.
type cachedUserTokenRepository struct {
	inner     UserTokenRepository
	cacheSlow cache.SecurityCache
	ttl       time.Duration
}

// NewCachedUserTokenRepository wraps inner with an L2 cache layer.
// cacheSlow == nil → degrades to pure DB (inner only), for environments without Redis.
func NewCachedUserTokenRepository(inner UserTokenRepository, cacheSlow cache.SecurityCache) UserTokenRepository {
	if cacheSlow == nil {
		return inner
	}
	return &cachedUserTokenRepository{
		inner:     inner,
		cacheSlow: cacheSlow,
		ttl:       30 * time.Second, // 与 admin 鉴权状态缓存 TTL 一致（30s 平衡 DB QPS 与失效延迟）
	}
}

func (r *cachedUserTokenRepository) invalidatePlatform(ctx context.Context, userID, platform string) {
	tags := []string{
		cache.TagUserTokenByPlatform(userID, platform),
		cache.TagUserTokenByUser(userID),
	}
	if err := r.cacheSlow.InvalidateByTags(ctx, tags...); err != nil {
		// 缓存失效失败不阻断主流程——TTL 兜底（30s 后自然过期），但记录错误便于排查。
		slog.Error("invalidate user_tokens cache failed", "userID", userID, "platform", platform, "tags", tags, "err", err)
	}
}

func (r *cachedUserTokenRepository) invalidateUser(ctx context.Context, userID string) {
	tag := cache.TagUserTokenByUser(userID)
	if err := r.cacheSlow.InvalidateByTags(ctx, tag); err != nil {
		slog.Error("invalidate user_tokens cache (by user) failed", "userID", userID, "err", err)
	}
}

// userTokenCacheEntry 缓存层的显式序列化结构。
//
// 不能直接缓存 user.UserToken：其 AccessHash/RefreshHash 标注 json:"-"
// （避免经 API 响应泄露哈希），而缓存层恰好用 JSON 序列化——直接存会把
// 哈希丢成空串，缓存命中的行再也过不了中间件的 fail-closed 哈希校验
// （历史教训：旧校验"空值跳过"正是为绕过此数据丢失而存在的 fail-open）。
type userTokenCacheEntry struct {
	ID               uint       `json:"id"`
	UserID           string     `json:"userId"`
	Platform         string     `json:"platform"`
	TokenVersion     uint64     `json:"tokenVersion"`
	AccessHash       string     `json:"accessHash"`
	RefreshHash      string     `json:"refreshHash"`
	AccessExpiresAt  *time.Time `json:"accessExpiresAt"`
	RefreshExpiresAt *time.Time `json:"refreshExpiresAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

func newUserTokenCacheEntry(t *user.UserToken) userTokenCacheEntry {
	return userTokenCacheEntry{
		ID: t.ID, UserID: t.UserID, Platform: t.Platform,
		TokenVersion: t.TokenVersion,
		AccessHash:   t.AccessHash, RefreshHash: t.RefreshHash,
		AccessExpiresAt: t.AccessExpiresAt, RefreshExpiresAt: t.RefreshExpiresAt,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func (e userTokenCacheEntry) toUserToken() *user.UserToken {
	return &user.UserToken{
		ID: e.ID, UserID: e.UserID, Platform: e.Platform,
		TokenVersion: e.TokenVersion,
		AccessHash:   e.AccessHash, RefreshHash: e.RefreshHash,
		AccessExpiresAt: e.AccessExpiresAt, RefreshExpiresAt: e.RefreshExpiresAt,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func (r *cachedUserTokenRepository) UpsertAndIncrement(ctx context.Context, t *user.UserToken) (uint64, error) {
	v, err := r.inner.UpsertAndIncrement(ctx, t)
	if err != nil {
		return 0, err
	}
	// 顶号场景：同 platform 重新登录，旧会话缓存必须立即失效。
	// 仅失效当前 platform（跨 platform 不受影响），同时带 user 级 tag 保持一致。
	r.invalidatePlatform(ctx, t.UserID, t.Platform)
	return v, nil
}

func (r *cachedUserTokenRepository) GetByPlatform(ctx context.Context, userID, platform string) (*user.UserToken, error) {
	key := cache.KeyUserToken(userID, platform)
	// 缓存命中：返回缓存的 UserToken 行（含 token_version + 两个 hash，经专用 entry 序列化）
	var cached userTokenCacheEntry
	if err := r.cacheSlow.Get(ctx, key, &cached); err == nil {
		return cached.toUserToken(), nil
	}
	// 缓存未命中：回源 DB，并回填缓存（双重 tag：user 级 + platform 级，便于精准失效）
	got, err := r.inner.GetByPlatform(ctx, userID, platform)
	if err != nil {
		// 行不存在（gorm.ErrRecordNotFound）不回填缓存，避免缓存穿透占位
		return nil, err
	}
	tags := []string{
		cache.TagUserTokenByUser(userID),
		cache.TagUserTokenByPlatform(userID, platform),
	}
	if err := r.cacheSlow.Set(ctx, key, newUserTokenCacheEntry(got), r.ttl, tags...); err != nil {
		// 回填失败不阻断鉴权（仅失去加速，下次回源）
		slog.Warn("set user_tokens cache failed", "key", key, "err", err)
	}
	return got, nil
}

func (r *cachedUserTokenRepository) UpdateAccessHash(ctx context.Context, userID, platform, accessHash string, accessExpiresAt time.Time) error {
	if err := r.inner.UpdateAccessHash(ctx, userID, platform, accessHash, accessExpiresAt); err != nil {
		return err
	}
	r.invalidatePlatform(ctx, userID, platform)
	return nil
}

func (r *cachedUserTokenRepository) UpdateHashes(ctx context.Context, userID, platform, accessHash, refreshHash string, accessExpiresAt, refreshExpiresAt time.Time) error {
	if err := r.inner.UpdateHashes(ctx, userID, platform, accessHash, refreshHash, accessExpiresAt, refreshExpiresAt); err != nil {
		return err
	}
	r.invalidatePlatform(ctx, userID, platform)
	return nil
}

func (r *cachedUserTokenRepository) ClearHashes(ctx context.Context, userID, platform string) error {
	if err := r.inner.ClearHashes(ctx, userID, platform); err != nil {
		return err
	}
	r.invalidatePlatform(ctx, userID, platform)
	return nil
}

func (r *cachedUserTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	// 批量删除过期行——无法精准失效每条缓存，依赖 TTL 兜底（30s 后过期缓存自然回源发现行已删）
	return r.inner.DeleteExpired(ctx)
}

// 编译期保证实现 UserTokenRepository 接口
var _ UserTokenRepository = (*cachedUserTokenRepository)(nil)
