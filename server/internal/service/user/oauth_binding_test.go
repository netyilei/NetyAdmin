package user

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	userEntity "NetyAdmin/internal/domain/entity/user"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
)

// mockOAuthRepo is a focused mock for OAuthBindingService tests.
type mockOAuthRepo struct {
	findByOpenID    *userEntity.UserOAuthBinding
	findByOpenIDErr error

	findByUnionID    *userEntity.UserOAuthBinding
	findByUnionIDErr error

	findByUserProvider    *userEntity.UserOAuthBinding
	findByUserProviderErr error

	createErr error

	deletedUserProvider map[string]bool
	listResult          []userEntity.UserOAuthBinding
	listErr             error
}

func (m *mockOAuthRepo) FindOAuthBinding(_ context.Context, _, _ string) (*userEntity.UserOAuthBinding, error) {
	return m.findByOpenID, m.findByOpenIDErr
}
func (m *mockOAuthRepo) FindOAuthBindingByUnionID(_ context.Context, _, _ string) (*userEntity.UserOAuthBinding, error) {
	return m.findByUnionID, m.findByUnionIDErr
}
func (m *mockOAuthRepo) FindOAuthBindingByUserProvider(_ context.Context, _, _ string) (*userEntity.UserOAuthBinding, error) {
	return m.findByUserProvider, m.findByUserProviderErr
}
func (m *mockOAuthRepo) CreateOAuthBinding(_ context.Context, _ *userEntity.UserOAuthBinding) error {
	return m.createErr
}
func (m *mockOAuthRepo) DeleteOAuthBinding(_ context.Context, userID, provider string) error {
	if m.deletedUserProvider == nil {
		m.deletedUserProvider = make(map[string]bool)
	}
	m.deletedUserProvider[userID+":"+provider] = true
	return nil
}
func (m *mockOAuthRepo) ListOAuthBindings(_ context.Context, _ string) ([]userEntity.UserOAuthBinding, error) {
	return m.listResult, m.listErr
}

// mockConfigCache is a minimal ConfigCache mock for OAuth tests.
// FetchFast executes the loader directly (simulating cache miss → DB).
// DeleteFast and InvalidateByTags are tracked via fields.
type mockConfigCache struct {
	mu              sync.Mutex
	deletedKeys     map[string]bool
	invalidatedTags map[string]bool
}

func newMockConfigCache() *mockConfigCache {
	return &mockConfigCache{
		deletedKeys:     make(map[string]bool),
		invalidatedTags: make(map[string]bool),
	}
}

func (m *mockConfigCache) FetchFast(_ context.Context, _ string, _ string, _ []string, _ time.Duration, v interface{}, loader func() (interface{}, error)) error {
	val, err := loader()
	if err != nil {
		return err
	}
	if val == nil {
		return nil
	}
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (m *mockConfigCache) SetFast(_ context.Context, _ string, _ interface{}, _ []string, _ time.Duration) error {
	return nil
}
func (m *mockConfigCache) GetFast(_ context.Context, _ string, _ []string, _ time.Duration, _ interface{}) error {
	return errors.New("cache miss")
}
func (m *mockConfigCache) DeleteFast(_ context.Context, key string) error {
	m.mu.Lock()
	m.deletedKeys[key] = true
	m.mu.Unlock()
	return nil
}
func (m *mockConfigCache) InvalidateByTags(_ context.Context, tags ...string) error {
	m.mu.Lock()
	for _, t := range tags {
		m.invalidatedTags[t] = true
	}
	m.mu.Unlock()
	return nil
}
func (m *mockConfigCache) IsCacheEnabled(_ string) bool { return true }

// --- Tests ---

func TestOAuthBindingService_FindByOpenID(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := &mockOAuthRepo{
			findByOpenID: &userEntity.UserOAuthBinding{
				UserID: "u1", Provider: "wechat", OpenID: "ox1", CreatedAt: time.Now(),
			},
		}
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, newMockConfigCache())

		dto, err := svc.FindByOpenID(context.Background(), "wechat", "ox1")
		require.NoError(t, err)
		require.NotNil(t, dto)
		assert.Equal(t, "u1", dto.UserID)
		assert.Equal(t, "wechat", dto.Provider)
		assert.Equal(t, "ox1", dto.OpenID)
	})

	t.Run("not found returns nil", func(t *testing.T) {
		repo := &mockOAuthRepo{findByOpenIDErr: gorm.ErrRecordNotFound}
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, newMockConfigCache())

		dto, err := svc.FindByOpenID(context.Background(), "wechat", "ox1")
		require.NoError(t, err)
		assert.Nil(t, dto)
	})

	t.Run("db error propagates", func(t *testing.T) {
		repo := &mockOAuthRepo{findByOpenIDErr: errors.New("connection lost")}
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, newMockConfigCache())

		dto, err := svc.FindByOpenID(context.Background(), "wechat", "ox1")
		require.Error(t, err)
		assert.Nil(t, dto)
	})
}

func TestOAuthBindingService_FindByUnionID(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := &mockOAuthRepo{
			findByUnionID: &userEntity.UserOAuthBinding{
				UserID: "u2", Provider: "wechat", UnionID: "ux1",
			},
		}
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, newMockConfigCache())

		dto, err := svc.FindByUnionID(context.Background(), "wechat", "ux1")
		require.NoError(t, err)
		require.NotNil(t, dto)
		assert.Equal(t, "u2", dto.UserID)
		assert.Equal(t, "ux1", dto.UnionID)
	})

	t.Run("not found returns nil", func(t *testing.T) {
		repo := &mockOAuthRepo{findByUnionIDErr: gorm.ErrRecordNotFound}
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, newMockConfigCache())

		dto, err := svc.FindByUnionID(context.Background(), "wechat", "ux1")
		require.NoError(t, err)
		assert.Nil(t, dto)
	})
}

func TestOAuthBindingService_Bind(t *testing.T) {
	t.Run("success — new binding", func(t *testing.T) {
		repo := &mockOAuthRepo{}
		cc := newMockConfigCache()
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, cc)

		err := svc.Bind(context.Background(), "u1", "wechat", "ox1", "ux1")
		require.NoError(t, err)

		// Verify cache invalidation was called
		assert.True(t, cc.deletedKeys[cache.KeyOAuthBindingByOpenID("wechat", "ox1")])
		assert.True(t, cc.deletedKeys[cache.KeyOAuthBindingByUnionID("wechat", "ux1")])
		assert.True(t, cc.invalidatedTags[cache.TagOAuthBindingByUserID("u1")])
	})

	t.Run("idempotent — already bound to same user", func(t *testing.T) {
		repo := &mockOAuthRepo{
			findByOpenID: &userEntity.UserOAuthBinding{UserID: "u1", Provider: "wechat", OpenID: "ox1"},
		}
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, newMockConfigCache())

		err := svc.Bind(context.Background(), "u1", "wechat", "ox1", "ux1")
		require.NoError(t, err)
	})

	t.Run("reject — already bound to another user", func(t *testing.T) {
		repo := &mockOAuthRepo{
			findByOpenID: &userEntity.UserOAuthBinding{UserID: "u2", Provider: "wechat", OpenID: "ox1"},
		}
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, newMockConfigCache())

		err := svc.Bind(context.Background(), "u1", "wechat", "ox1", "ux1")
		require.Error(t, err)
		bizErr, ok := err.(*errorx.BizError)
		require.True(t, ok, "expected BizError")
		assert.Equal(t, errorx.CodeOAuthAlreadyBound, bizErr.Code)
	})

	t.Run("concurrent — pgconn 23505 mapped to CodeOAuthAlreadyBound", func(t *testing.T) {
		repo := &mockOAuthRepo{
			createErr: &pgconn.PgError{Code: "23505", Message: "duplicate key"},
		}
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, newMockConfigCache())

		err := svc.Bind(context.Background(), "u1", "wechat", "ox1", "ux1")
		require.Error(t, err)
		bizErr, ok := err.(*errorx.BizError)
		require.True(t, ok, "expected BizError")
		assert.Equal(t, errorx.CodeOAuthAlreadyBound, bizErr.Code)
	})

	t.Run("non-23505 pgconn error propagates as-is", func(t *testing.T) {
		repo := &mockOAuthRepo{
			createErr: &pgconn.PgError{Code: "23503", Message: "foreign key violation"},
		}
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, newMockConfigCache())

		err := svc.Bind(context.Background(), "u1", "wechat", "ox1", "ux1")
		require.Error(t, err)
		_, isBizErr := err.(*errorx.BizError)
		assert.False(t, isBizErr, "should not be BizError for non-23505 pgconn error")
	})

	t.Run("db error on check propagates", func(t *testing.T) {
		repo := &mockOAuthRepo{
			findByOpenIDErr: errors.New("connection lost"),
		}
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, newMockConfigCache())

		err := svc.Bind(context.Background(), "u1", "wechat", "ox1", "ux1")
		require.Error(t, err)
		_, isBizErr := err.(*errorx.BizError)
		assert.False(t, isBizErr, "should not be BizError for db failure")
	})
}

func TestOAuthBindingService_Unbind(t *testing.T) {
	t.Run("success — invalidates specific cache keys", func(t *testing.T) {
		repo := &mockOAuthRepo{
			findByUserProvider: &userEntity.UserOAuthBinding{
				UserID: "u1", Provider: "wechat", OpenID: "ox1", UnionID: "ux1",
			},
		}
		cc := newMockConfigCache()
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, cc)

		err := svc.Unbind(context.Background(), "u1", "wechat")
		require.NoError(t, err)
		assert.True(t, repo.deletedUserProvider["u1:wechat"])

		// Verify specific openid/unionid keys were deleted
		assert.True(t, cc.deletedKeys[cache.KeyOAuthBindingByOpenID("wechat", "ox1")])
		assert.True(t, cc.deletedKeys[cache.KeyOAuthBindingByUnionID("wechat", "ux1")])
		assert.True(t, cc.invalidatedTags[cache.TagOAuthBindingByUserID("u1")])
	})

	t.Run("binding not found — fallback to tag invalidation", func(t *testing.T) {
		repo := &mockOAuthRepo{
			findByUserProviderErr: gorm.ErrRecordNotFound,
		}
		cc := newMockConfigCache()
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, cc)

		err := svc.Unbind(context.Background(), "u1", "wechat")
		require.NoError(t, err)
		// Should not have deleted specific keys (binding unknown)
		assert.Empty(t, cc.deletedKeys)
		// Should have invalidated by userID tag
		assert.True(t, cc.invalidatedTags[cache.TagOAuthBindingByUserID("u1")])
	})
}

func TestOAuthBindingService_ListByUserID(t *testing.T) {
	t.Run("returns DTOs", func(t *testing.T) {
		repo := &mockOAuthRepo{
			listResult: []userEntity.UserOAuthBinding{
				{UserID: "u1", Provider: "wechat", OpenID: "ox1", CreatedAt: time.Now()},
				{UserID: "u1", Provider: "alipay", OpenID: "ax1", CreatedAt: time.Now()},
			},
		}
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, newMockConfigCache())

		dtos, err := svc.ListByUserID(context.Background(), "u1")
		require.NoError(t, err)
		assert.Len(t, dtos, 2)
		assert.Equal(t, "wechat", dtos[0].Provider)
		assert.Equal(t, "alipay", dtos[1].Provider)
	})

	t.Run("empty list", func(t *testing.T) {
		repo := &mockOAuthRepo{}
		svc := NewOAuthBindingService(repo, &database.MockTxManager{}, newMockConfigCache())

		dtos, err := svc.ListByUserID(context.Background(), "u1")
		require.NoError(t, err)
		assert.Empty(t, dtos)
	})
}
