package user

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"

	userEntity "NetyAdmin/internal/domain/entity/user"
	userRepo "NetyAdmin/internal/repository/user"
)

type TokenStore interface {
	Create(ctx context.Context, hash *userEntity.UserTokenHash) error
	Get(ctx context.Context, userID, tokenHash string) (*userEntity.UserTokenHash, error)
	Delete(ctx context.Context, userID, tokenHash string) error
	DeleteAll(ctx context.Context, userID string) error
}

type dbTokenStore struct {
	repo userRepo.UserRepository
}

func NewDBTokenStore(repo userRepo.UserRepository) TokenStore {
	return &dbTokenStore{repo: repo}
}

func (s *dbTokenStore) Create(ctx context.Context, hash *userEntity.UserTokenHash) error {
	return s.repo.CreateTokenHash(ctx, hash)
}

func (s *dbTokenStore) Get(ctx context.Context, userID, tokenHash string) (*userEntity.UserTokenHash, error) {
	return s.repo.GetTokenHash(ctx, userID, tokenHash)
}

func (s *dbTokenStore) Delete(ctx context.Context, userID, tokenHash string) error {
	return s.repo.DeleteTokenHash(ctx, userID, tokenHash)
}

func (s *dbTokenStore) DeleteAll(ctx context.Context, userID string) error {
	return s.repo.DeleteAllTokenHashes(ctx, userID)
}

type cacheTokenStore struct {
	cacheMgr cache.LazyCacheManager
	repo     userRepo.UserRepository
}

func NewCacheTokenStore(cacheMgr cache.LazyCacheManager, repo userRepo.UserRepository) TokenStore {
	return &cacheTokenStore{cacheMgr: cacheMgr, repo: repo}
}

func (s *cacheTokenStore) Create(ctx context.Context, hash *userEntity.UserTokenHash) error {
	if err := s.repo.CreateTokenHash(ctx, hash); err != nil {
		return err
	}
	ttl := time.Until(hash.ExpiredAt)
	if ttl <= 0 {
		ttl = time.Hour
	}
	key := cache.KeyUserTokenHash(hash.UserID, hash.TokenHash)
	_ = s.cacheMgr.Set(ctx, key, "1", ttl)
	return nil
}

func (s *cacheTokenStore) Get(ctx context.Context, userID, tokenHash string) (*userEntity.UserTokenHash, error) {
	key := cache.KeyUserTokenHash(userID, tokenHash)
	var val string
	err := s.cacheMgr.Get(ctx, key, &val)
	if err == nil && val != "" {
		return &userEntity.UserTokenHash{UserID: userID, TokenHash: tokenHash}, nil
	}
	return s.repo.GetTokenHash(ctx, userID, tokenHash)
}

func (s *cacheTokenStore) Delete(ctx context.Context, userID, tokenHash string) error {
	key := cache.KeyUserTokenHash(userID, tokenHash)
	_ = s.cacheMgr.Delete(ctx, key)
	return s.repo.DeleteTokenHash(ctx, userID, tokenHash)
}

func (s *cacheTokenStore) DeleteAll(ctx context.Context, userID string) error {
	_ = s.cacheMgr.InvalidateByTags(ctx, fmt.Sprintf("user:token:%s", userID))
	return s.repo.DeleteAllTokenHashes(ctx, userID)
}

func NewTokenStoreFromConfig(watcher configsync.ConfigWatcher, repo userRepo.UserRepository, cacheMgr cache.LazyCacheManager) TokenStore {
	val, _ := watcher.GetConfig("user_config", "login_storage")
	if val == "cache" {
		return NewCacheTokenStore(cacheMgr, repo)
	}
	return NewDBTokenStore(repo)
}

func ParseTokenExpire(watcher configsync.ConfigWatcher) int {
	val, _ := watcher.GetConfig("user_config", "token_expire")
	if val == "" {
		return 86400
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return 86400
	}
	return n
}
