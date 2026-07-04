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
	Create(ctx context.Context, hash *userEntity.UserTokenHash) error
	Get(ctx context.Context, userID, tokenHash string) (*userEntity.UserTokenHash, error)
	Delete(ctx context.Context, userID, tokenHash string) error
	DeleteAll(ctx context.Context, userID string) error
}

// tokenStore 唯一实现：DB 持久化 + Redis 缓存加速。
type tokenStore struct {
	repo     userRepo.UserRepository
	cacheMgr cache.LazyCacheManager
}

// NewTokenStore 构造会话令牌存储。
// repo 是 DB 真相源，cacheMgr 是可选的加速层（为 nil 时退化为纯 DB）。
func NewTokenStore(repo userRepo.UserRepository, cacheMgr cache.LazyCacheManager) TokenStore {
	return &tokenStore{repo: repo, cacheMgr: cacheMgr}
}

func (s *tokenStore) Create(ctx context.Context, hash *userEntity.UserTokenHash) error {
	if err := s.repo.CreateTokenHash(ctx, hash); err != nil {
		return fmt.Errorf("repo.CreateTokenHash: %w", err)
	}
	// 缓存加速：将"会话有效"标记写入缓存，命中时免去 DB 查询
	if s.cacheMgr != nil {
		ttl := time.Until(hash.ExpiredAt)
		if ttl <= 0 {
			ttl = time.Hour
		}
		key := cache.KeyUserTokenHash(hash.UserID, hash.TokenHash)
		tag := cache.TagUserToken(hash.UserID)
		if err := s.cacheMgr.SetFast(ctx, key, "1", []string{tag}, ttl); err != nil {
			slog.Warn("set fast cache failed", "key", key, "err", err)
		}
	}
	return nil
}

func (s *tokenStore) Get(ctx context.Context, userID, tokenHash string) (*userEntity.UserTokenHash, error) {
	// 缓存命中：直接返回占位实体（会话仍有效）
	if s.cacheMgr != nil {
		key := cache.KeyUserTokenHash(userID, tokenHash)
		var val string
		if err := s.cacheMgr.Get(ctx, key, &val); err == nil && val != "" {
			return &userEntity.UserTokenHash{UserID: userID, TokenHash: tokenHash}, nil
		}
	}
	// 缓存未命中：回源 DB
	return s.repo.GetTokenHash(ctx, userID, tokenHash)
}

func (s *tokenStore) Delete(ctx context.Context, userID, tokenHash string) error {
	if s.cacheMgr != nil {
		key := cache.KeyUserTokenHash(userID, tokenHash)
		if err := s.cacheMgr.Delete(ctx, key); err != nil {
			slog.Warn("delete cache failed", "key", key, "err", err)
		}
	}
	return s.repo.DeleteTokenHash(ctx, userID, tokenHash)
}

func (s *tokenStore) DeleteAll(ctx context.Context, userID string) error {
	if s.cacheMgr != nil {
		tag := cache.TagUserToken(userID)
		if err := s.cacheMgr.InvalidateByTags(ctx, tag); err != nil {
			slog.Error("invalidate cache failed", "tag", tag, "err", err)
		}
	}
	return s.repo.DeleteAllTokenHashes(ctx, userID)
}
