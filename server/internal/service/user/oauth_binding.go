package user

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	userEntity "NetyAdmin/internal/domain/entity/user"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
)

// oauthCacheModule is the cache module name for OAuth binding data.
const oauthCacheModule = "oauth_binding"

// oauthCacheTTL is the cache TTL for OAuth binding lookups.
// Binding relationships change infrequently; 30 minutes balances cache
// freshness with DB load reduction for high-frequency login lookups.
const oauthCacheTTL = 30 * time.Minute

// OAuthBindingDTO is the service-layer representation of an OAuth binding,
// decoupled from the entity layer. Per server-architecture.md §5.4, DTOs
// contain only business fields — no persistence fields like ID/CreatedAt.
type OAuthBindingDTO struct {
	UserID   string `json:"userId"`
	Provider string `json:"provider"`
	OpenID   string `json:"openId"`
	UnionID  string `json:"unionId"`
}

// OAuthBindingService provides third-party OAuth account binding operations.
//
// Downstream projects implement the provider-specific adapter (calling
// WeChat/Alipay/GitHub/Apple APIs to exchange code for openid), then delegate
// binding storage and lookup to this service — avoiding re-implementation of
// binding relationship management in every project.
//
// Read paths (FindByOpenID, FindByUnionID) are cached via ConfigCache (L1+L2
// chain) because binding relationships change infrequently and are read on
// every OAuth login. Write paths (Bind, Unbind) invalidate the affected
// cache keys after transaction commit.
//
// Usage flow (e.g. WeChat login):
//  1. Client sends WeChat auth code to a downstream handler.
//  2. Downstream adapter calls WeChat API → gets openid + unionid.
//  3. Call FindByOpenID("wechat", openid) → if found, login the bound user.
//  4. If not found, create user (via UserRepository) then call Bind().
type OAuthBindingService interface {
	// FindByOpenID looks up a binding by provider + openid.
	// Returns nil, nil if not bound (not an error).
	FindByOpenID(ctx context.Context, provider, openid string) (*OAuthBindingDTO, error)
	// FindByUnionID looks up by provider + unionid (WeChat unionid scenario).
	// Returns nil, nil if not found.
	FindByUnionID(ctx context.Context, provider, unionid string) (*OAuthBindingDTO, error)
	// Bind creates a new OAuth binding. Returns errorx.CodeOAuthAlreadyBound if
	// the (provider, openid) pair is already bound to another user.
	Bind(ctx context.Context, userID, provider, openid, unionid string) error
	// Unbind removes a binding by userID + provider.
	Unbind(ctx context.Context, userID, provider string) error
	// ListByUserID returns all OAuth bindings for a user.
	ListByUserID(ctx context.Context, userID string) ([]OAuthBindingDTO, error)
}

// OAuthBindingRepo is the narrow repository interface needed by OAuthBindingService.
// By depending on this instead of the full UserRepository, the service follows
// the Interface Segregation Principle and is easier to mock in tests.
type OAuthBindingRepo interface {
	FindOAuthBinding(ctx context.Context, provider, openid string) (*userEntity.UserOAuthBinding, error)
	FindOAuthBindingByUnionID(ctx context.Context, provider, unionid string) (*userEntity.UserOAuthBinding, error)
	FindOAuthBindingByUserProvider(ctx context.Context, userID, provider string) (*userEntity.UserOAuthBinding, error)
	CreateOAuthBinding(ctx context.Context, binding *userEntity.UserOAuthBinding) error
	DeleteOAuthBinding(ctx context.Context, userID, provider string) error
	ListOAuthBindings(ctx context.Context, userID string) ([]userEntity.UserOAuthBinding, error)
}

type oauthBindingService struct {
	repo  OAuthBindingRepo
	tm    database.TxManager
	cache cache.ConfigCache
}

func NewOAuthBindingService(repo OAuthBindingRepo, tm database.TxManager, cache cache.ConfigCache) OAuthBindingService {
	return &oauthBindingService{repo: repo, tm: tm, cache: cache}
}

func (s *oauthBindingService) FindByOpenID(ctx context.Context, provider, openid string) (*OAuthBindingDTO, error) {
	key := cache.KeyOAuthBindingByOpenID(provider, openid)
	tags := []string{cache.TagOAuthBinding}

	var dto OAuthBindingDTO
	err := s.cache.FetchFast(ctx, key, oauthCacheModule, tags, oauthCacheTTL, &dto, func() (interface{}, error) {
		binding, err := s.repo.FindOAuthBinding(ctx, provider, openid)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			slog.Error("OAuthBindingService.FindByOpenID: repo lookup failed", "provider", provider, "error", err)
			return nil, err
		}
		return toDTO(binding), nil
	})
	if err != nil {
		return nil, err
	}
	// FetchFast returns empty dto when loader returns nil (not bound)
	if dto.UserID == "" {
		return nil, nil
	}
	return &dto, nil
}

func (s *oauthBindingService) FindByUnionID(ctx context.Context, provider, unionid string) (*OAuthBindingDTO, error) {
	key := cache.KeyOAuthBindingByUnionID(provider, unionid)
	tags := []string{cache.TagOAuthBinding}

	var dto OAuthBindingDTO
	err := s.cache.FetchFast(ctx, key, oauthCacheModule, tags, oauthCacheTTL, &dto, func() (interface{}, error) {
		binding, err := s.repo.FindOAuthBindingByUnionID(ctx, provider, unionid)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			slog.Error("OAuthBindingService.FindByUnionID: repo lookup failed", "provider", provider, "error", err)
			return nil, err
		}
		return toDTO(binding), nil
	})
	if err != nil {
		return nil, err
	}
	if dto.UserID == "" {
		return nil, nil
	}
	return &dto, nil
}

func (s *oauthBindingService) Bind(ctx context.Context, userID, provider, openid, unionid string) error {
	if err := s.tm.WithTransaction(ctx, func(txCtx context.Context) error {
		existing, err := s.repo.FindOAuthBinding(txCtx, provider, openid)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Error("OAuthBindingService.Bind: check existing failed", "provider", provider, "error", err)
			return err
		}
		if existing != nil && existing.UserID != userID {
			return errorx.New(errorx.CodeOAuthAlreadyBound, "该第三方账号已绑定其他用户")
		}
		if existing != nil {
			return nil
		}

		binding := &userEntity.UserOAuthBinding{
			UserID:   userID,
			Provider: provider,
			OpenID:   openid,
			UnionID:  unionid,
		}
		if err := s.repo.CreateOAuthBinding(txCtx, binding); err != nil {
			if isUniqueViolation(err) {
				return errorx.New(errorx.CodeOAuthAlreadyBound, "该第三方账号已绑定其他用户")
			}
			slog.Error("OAuthBindingService.Bind: create failed", "provider", provider, "error", err)
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Invalidate cache after successful commit.
	// New binding means previous "not found" cache entries are now stale.
	s.invalidateBindingCache(ctx, provider, openid, unionid, userID)
	return nil
}

func (s *oauthBindingService) Unbind(ctx context.Context, userID, provider string) error {
	// Look up the binding before deletion to get openid/unionid for cache invalidation.
	binding, err := s.repo.FindOAuthBindingByUserProvider(ctx, userID, provider)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Error("OAuthBindingService.Unbind: lookup failed", "userID", userID, "provider", provider, "error", err)
		return err
	}

	if err := s.repo.DeleteOAuthBinding(ctx, userID, provider); err != nil {
		slog.Error("OAuthBindingService.Unbind: delete failed", "userID", userID, "provider", provider, "error", err)
		return err
	}

	// Invalidate specific cache keys using openid/unionid from the deleted binding.
	if binding != nil {
		s.invalidateBindingCache(ctx, provider, binding.OpenID, binding.UnionID, userID)
	} else {
		// Binding not found (already unbound) — best-effort tag invalidation
		s.invalidateBindingCacheByUserID(ctx, userID)
	}
	return nil
}

func (s *oauthBindingService) ListByUserID(ctx context.Context, userID string) ([]OAuthBindingDTO, error) {
	bindings, err := s.repo.ListOAuthBindings(ctx, userID)
	if err != nil {
		slog.Error("OAuthBindingService.ListByUserID failed", "userID", userID, "error", err)
		return nil, err
	}
	result := make([]OAuthBindingDTO, len(bindings))
	for i, b := range bindings {
		result[i] = *toDTO(&b)
	}
	return result, nil
}

// invalidateBindingCache removes cache entries for a specific binding.
// Called after Bind and Unbind to clear stale entries.
func (s *oauthBindingService) invalidateBindingCache(ctx context.Context, provider, openid, unionid, userID string) {
	// Delete specific openid/unionid keys (they may have cached "not found" or "bound" data)
	if err := s.cache.DeleteFast(ctx, cache.KeyOAuthBindingByOpenID(provider, openid)); err != nil {
		slog.Warn("OAuthBindingService: cache delete failed for openid key", "provider", provider, "error", err)
	}
	if unionid != "" {
		if err := s.cache.DeleteFast(ctx, cache.KeyOAuthBindingByUnionID(provider, unionid)); err != nil {
			slog.Warn("OAuthBindingService: cache delete failed for unionid key", "provider", provider, "error", err)
		}
	}
	// Also invalidate by userID tag for safety (covers any future per-user tagged entries)
	if err := s.cache.InvalidateByTags(ctx, cache.TagOAuthBindingByUserID(userID)); err != nil {
		slog.Warn("OAuthBindingService: cache tag invalidation failed", "userID", userID, "error", err)
	}
}

// invalidateBindingCacheByUserID removes all cached bindings for a user by tag.
// Used as a fallback when the binding's openid/unionid is unknown.
func (s *oauthBindingService) invalidateBindingCacheByUserID(ctx context.Context, userID string) {
	if err := s.cache.InvalidateByTags(ctx, cache.TagOAuthBindingByUserID(userID)); err != nil {
		slog.Warn("OAuthBindingService: cache tag invalidation failed", "userID", userID, "error", err)
	}
}

// toDTO converts an entity to its DTO representation.
func toDTO(b *userEntity.UserOAuthBinding) *OAuthBindingDTO {
	if b == nil {
		return nil
	}
	return &OAuthBindingDTO{
		UserID:   b.UserID,
		Provider: b.Provider,
		OpenID:   b.OpenID,
		UnionID:  b.UnionID,
	}
}

// isUniqueViolation checks whether err is a PostgreSQL unique constraint violation.
// Uses pgconn.PgError type assertion with code "23505" for reliable detection.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
