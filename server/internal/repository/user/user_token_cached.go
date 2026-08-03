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
	// 缓存命中：直接返回缓存的 UserToken 行（含 token_version + access_hash）
	var cached user.UserToken
	if err := r.cacheSlow.Get(ctx, key, &cached); err == nil {
		return &cached, nil
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
	if err := r.cacheSlow.Set(ctx, key, got, r.ttl, tags...); err != nil {
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
