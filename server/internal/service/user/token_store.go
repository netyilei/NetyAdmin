package user

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	userEntity "NetyAdmin/internal/domain/entity/user"
	"NetyAdmin/internal/pkg/cache"
	userRepo "NetyAdmin/internal/repository/user"
)

// TokenStore 是会话令牌哈希的存储抽象。
//
// 设计原则（RULES.md §0.1）：单一最优实现。
// DB 是唯一真相源（token 哈希表），Redis 缓存作为加速层在 store 内部组合，
// 调用方不感知缓存存在。这避免了 dbTokenStore / cacheTokenStore 双实现
// 运行时切换的补丁形态。
//
// admin 与 user 共用同一 store 实现，通过 userID 自然隔离（admin 端使用
// service/system/admin.go 中的 AdminTokenKey() 生成 key），不再依赖
// "a:" 字符串前缀这种隐式约定。
type TokenStore interface {
	Create(ctx context.Context, hash *userEntity.AdminToken) error
	Get(ctx context.Context, userID, tokenHash string) (*userEntity.AdminToken, error)
	Delete(ctx context.Context, userID, tokenHash string) error
	DeleteAll(ctx context.Context, userID string) error
}

// tokenStore 唯一实现：DB 持久化 + Redis 缓存加速。
//
// 缓存层级（铁律：非 Fast 方法只走 L2，绝不碰 L1）：
// token 是安全敏感数据，登出/踢人/刷新时必须立即全节点失效。
// 用非 Fast 方法（Set/Get/Delete），数据只写入 L2 (Redis 共享层)。
// Redis 删除一次即对整个集群立即生效，无 PubSub 窗口期，无需广播。
// token 永不进 L1 (BigCache 本地内存)，避免多机 L1 同步窗口期的安全风险。
type tokenStore struct {
	repo      userRepo.UserRepository
	cacheSlow cache.SecurityCache
}

// NewTokenStore 构造会话令牌存储。
// repo 是 DB 真相源，cacheSlow 是可选的加速层（为 nil 时退化为纯 DB）。
func NewTokenStore(repo userRepo.UserRepository, cacheSlow cache.SecurityCache) TokenStore {
	return &tokenStore{repo: repo, cacheSlow: cacheSlow}
}

func (s *tokenStore) Create(ctx context.Context, hash *userEntity.AdminToken) error {
	if err := s.repo.CreateTokenHash(ctx, hash); err != nil {
		return fmt.Errorf("repo.CreateTokenHash: %w", err)
	}
	// 缓存加速：将"会话有效"标记写入 L2 (Redis)，命中时免去 DB 查询。
	// 用非 Fast 的 Set（铁律：只走 L2，绝不碰 L1），带 tag 便于 DeleteAll 批量失效。
	if s.cacheSlow != nil {
		ttl := time.Until(hash.ExpiredAt)
		if ttl <= 0 {
			ttl = time.Hour
		}
		key := cache.KeyUserTokenHash(hash.UserID, hash.TokenHash)
		tag := cache.TagUserToken(hash.UserID)
		if err := s.cacheSlow.Set(ctx, key, "1", ttl, tag); err != nil {
			slog.Warn("set token cache failed", "key", key, "err", err)
		}
	}
	return nil
}

func (s *tokenStore) Get(ctx context.Context, userID, tokenHash string) (*userEntity.AdminToken, error) {
	// 缓存命中：直接返回占位实体（会话仍有效）
	if s.cacheSlow != nil {
		key := cache.KeyUserTokenHash(userID, tokenHash)
		var val string
		if err := s.cacheSlow.Get(ctx, key, &val); err == nil && val != "" {
			return &userEntity.AdminToken{UserID: userID, TokenHash: tokenHash}, nil
		}
	}
	// 缓存未命中：回源 DB
	return s.repo.GetTokenHash(ctx, userID, tokenHash)
}

func (s *tokenStore) Delete(ctx context.Context, userID, tokenHash string) error {
	if s.cacheSlow != nil {
		key := cache.KeyUserTokenHash(userID, tokenHash)
		// 用非 Fast 的 Delete（铁律：只走 L2）。
		// token 只存在 L2 (Redis 共享层)，删除一次即对整个集群立即生效，
		// 无 PubSub 窗口期，无需广播。
		if err := s.cacheSlow.Delete(ctx, key); err != nil {
			slog.Warn("delete token cache failed", "key", key, "err", err)
		}
	}
	return s.repo.DeleteTokenHash(ctx, userID, tokenHash)
}

func (s *tokenStore) DeleteAll(ctx context.Context, userID string) error {
	if s.cacheSlow != nil {
		tag := cache.TagUserToken(userID)
		if err := s.cacheSlow.InvalidateByTags(ctx, tag); err != nil {
			slog.Error("invalidate cache failed", "tag", tag, "err", err)
		}
	}
	return s.repo.DeleteAllTokenHashes(ctx, userID)
}
