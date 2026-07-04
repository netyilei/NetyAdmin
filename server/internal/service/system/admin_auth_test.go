// admin_auth_test.go 管理员认证 Service 单元测试基线。
//
// 覆盖范围（Task 9.1）：
//   - Login：成功登录 / 密码错误 / 用户不存在 / 用户已禁用 / 账户已锁定
//   - Logout：refresh token 写入黑名单
//   - RefreshToken：成功刷新 / token 无效（解析失败）/ token 已加入黑名单 / 用户已禁用
//
// Mock 策略：
//   - adminRepo / cacheMgr / tokenStore 使用手写 mock 结构体（项目无 testify/mock 依赖）
//   - jwt 使用真实 *jwt.JWT 实例（已知 secret），更贴近真实行为且无需打桩 ParseToken / GenerateToken
//   - password 使用真实 bcrypt（预计算 hash 加速）
//
// 注意：本测试只验证 Service 编排逻辑，不验证 token 内容的密码学正确性。
package system

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	userEntity "NetyAdmin/internal/domain/entity/user"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/password"
	"NetyAdmin/internal/pkg/pubsub"
	systemRepo "NetyAdmin/internal/repository/system"
	userService "NetyAdmin/internal/service/user"
)

// ============== mockCacheMgr：cache.LazyCacheManager 内存实现 ==============
//
// 仅对 admin_auth 路径用到的方法（Get/Set/Delete/Exists/Incr）实现真实行为，
// 其余方法返回零值——admin_auth 不会调用它们；保留接口完整性仅为通过编译。
type mockCacheMgr struct {
	mu       sync.Mutex
	values   map[string]string
	counters map[string]int64
	incrErr  error
	getErr   error
}

func newMockCacheMgr() *mockCacheMgr {
	return &mockCacheMgr{
		values:   make(map[string]string),
		counters: make(map[string]int64),
	}
}

func (m *mockCacheMgr) Get(_ context.Context, key string, dest interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		err := m.getErr
		m.getErr = nil
		return err
	}
	val, ok := m.values[key]
	if !ok {
		return errors.New("not found")
	}
	if s, ok := dest.(*string); ok {
		*s = val
	}
	return nil
}

func (m *mockCacheMgr) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := value.(string); ok {
		m.values[key] = s
	} else {
		m.values[key] = fmt.Sprintf("%v", value)
	}
	return nil
}

func (m *mockCacheMgr) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, key)
	delete(m.counters, key)
	return nil
}

func (m *mockCacheMgr) Exists(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.values[key]
	if !ok {
		_, ok = m.counters[key]
	}
	return ok, nil
}

func (m *mockCacheMgr) Incr(_ context.Context, key string, _ time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.incrErr != nil {
		err := m.incrErr
		m.incrErr = nil
		return 0, err
	}
	m.counters[key]++
	return m.counters[key], nil
}

// 以下方法 admin_auth 不使用，仅满足接口签名
func (m *mockCacheMgr) Fetch(_ context.Context, _ string, _ string, _ []string, _ time.Duration, _ interface{}, _ func() (interface{}, error)) error {
	return nil
}
func (m *mockCacheMgr) FetchFast(_ context.Context, _ string, _ string, _ []string, _ time.Duration, _ interface{}, _ func() (interface{}, error)) error {
	return nil
}
func (m *mockCacheMgr) InvalidateByTags(_ context.Context, _ ...string) error                  { return nil }
func (m *mockCacheMgr) SetFast(_ context.Context, _ string, _ interface{}, _ []string, _ time.Duration) error {
	return nil
}
func (m *mockCacheMgr) SetNX(_ context.Context, _ string, _ interface{}, _ time.Duration) (bool, error) {
	return false, nil
}
func (m *mockCacheMgr) GetFast(_ context.Context, _ string, _ []string, _ time.Duration, _ interface{}) error {
	return nil
}
func (m *mockCacheMgr) DeleteFast(_ context.Context, _ string) error                     { return nil }
func (m *mockCacheMgr) DeleteAndBroadcast(_ context.Context, _ string) error             { return nil }
func (m *mockCacheMgr) InvalidateL1ByTags(_ context.Context, _ ...string) error          { return nil }
func (m *mockCacheMgr) InvalidateL1ByKey(_ context.Context, _ string) error              { return nil }
func (m *mockCacheMgr) SetEventBus(_ pubsub.EventBus)                                    {}
func (m *mockCacheMgr) IsCacheEnabled(_ string) bool                                      { return true }
func (m *mockCacheMgr) GetRedisClient() *redis.Client                                     { return nil }

var _ cache.LazyCacheManager = (*mockCacheMgr)(nil)

// ============== mockTokenStore：userService.TokenStore 内存实现 ==============
type mockTokenStore struct {
	createCalls  int
	deleteCalls  int
	deleteAllErr error
	createErr    error
}

func (s *mockTokenStore) Create(_ context.Context, _ *userEntity.UserTokenHash) error {
	if s.createErr != nil {
		err := s.createErr
		s.createErr = nil
		return err
	}
	s.createCalls++
	return nil
}

func (s *mockTokenStore) Delete(_ context.Context, _, _ string) error {
	if s.deleteAllErr != nil {
		return s.deleteAllErr
	}
	s.deleteCalls++
	return nil
}

func (s *mockTokenStore) Get(_ context.Context, _, _ string) (*userEntity.UserTokenHash, error) {
	return nil, errors.New("not implemented")
}

func (s *mockTokenStore) DeleteAll(_ context.Context, _ string) error {
	if s.deleteAllErr != nil {
		return s.deleteAllErr
	}
	return nil
}

var _ userService.TokenStore = (*mockTokenStore)(nil)

// ============== mockAdminRepo：systemRepo.AdminRepository 内存实现 ==============
//
// 仅实现 admin_auth 用到的方法（GetByUsername/GetByID/UpdateLastLoginAt），
// 其余方法返回零值或 nil。
type mockAdminRepo struct {
	ByUsername         *systemEntity.Admin
	GetByUsernameErr   error
	AdminByID          *systemEntity.Admin
	GetByIDErr         error
	UpdateLastLoginErr error
}

func (r *mockAdminRepo) GetByUsername(_ context.Context, _ string) (*systemEntity.Admin, error) {
	if r.GetByUsernameErr != nil {
		return nil, r.GetByUsernameErr
	}
	return r.ByUsername, nil
}

func (r *mockAdminRepo) GetByID(_ context.Context, _ uint) (*systemEntity.Admin, error) {
	if r.GetByIDErr != nil {
		return nil, r.GetByIDErr
	}
	return r.AdminByID, nil
}

func (r *mockAdminRepo) UpdateLastLoginAt(_ context.Context, _ uint, _ string) error {
	return r.UpdateLastLoginErr
}

// 以下方法 admin_auth 不使用，仅满足接口签名
func (r *mockAdminRepo) Create(_ context.Context, _ *systemEntity.Admin) error { return nil }
func (r *mockAdminRepo) ExistsByUsername(_ context.Context, _ string, _ ...uint) (bool, error) {
	return false, nil
}
func (r *mockAdminRepo) List(_ context.Context, _ *systemRepo.AdminRepoQuery) ([]systemEntity.Admin, int64, error) {
	return nil, 0, nil
}
func (r *mockAdminRepo) Update(_ context.Context, _ *systemEntity.Admin) error              { return nil }
func (r *mockAdminRepo) IncrementTokenVersion(_ context.Context, _ uint) error              { return nil }
func (r *mockAdminRepo) ClearRoles(_ context.Context, _ uint) error                        { return nil }
func (r *mockAdminRepo) Delete(_ context.Context, _ uint) error                             { return nil }
func (r *mockAdminRepo) UpdateRoles(_ context.Context, _ uint, _ []uint) error              { return nil }
func (r *mockAdminRepo) GetAuthStateByID(_ context.Context, _ uint) (*systemRepo.AdminAuthState, error) {
	return nil, nil
}

var _ systemRepo.AdminRepository = (*mockAdminRepo)(nil)

// ============== 测试夹具与共享 helpers ==============

// testAdminJWTSecret 是测试用 JWT secret（满足长度 ≥16 + 2 类字符的强度要求）
const testAdminJWTSecret = "TestSecret-ForJWT-2025-ABC!@#def"

// newTestAdminService 构造一个 adminService + 配套 mocks，每个用例独立。
func newTestAdminService(t *testing.T) (*adminService, *mockAdminRepo, *mockCacheMgr, *mockTokenStore, *jwt.JWT) {
	t.Helper()
	j, err := jwt.New(testAdminJWTSecret, 1) // 1 小时过期
	require.NoError(t, err)

	repo := &mockAdminRepo{}
	cacheMgr := newMockCacheMgr()
	store := &mockTokenStore{}

	svc := &adminService{
		adminRepo:  repo,
		jwt:        j,
		cacheMgr:   cacheMgr,
		tokenStore: store,
	}
	return svc, repo, cacheMgr, store, j
}

// hashedTestPassword 返回 "Admin@12345" 的 bcrypt hash（仅计算一次，加速用例）
var (
	hashedTestPasswordOnce sync.Once
	hashedTestPasswordVal  string
)

func hashedTestPassword(t *testing.T) string {
	t.Helper()
	hashedTestPasswordOnce.Do(func() {
		h, err := password.Hash("Admin@12345")
		if err != nil {
			t.Fatalf("precompute password hash: %v", err)
		}
		hashedTestPasswordVal = h
	})
	return hashedTestPasswordVal
}

// enabledAdmin 构造一个启用状态的管理员（含预计算密码 hash）
func enabledAdmin(id uint, username string) *systemEntity.Admin {
	return &systemEntity.Admin{
		Model:        entity.Model{ID: id},
		Username:     username,
		Password:     hashedTestPasswordVal,
		Status:       entity.StatusEnabled,
		TokenVersion: 1,
	}
}

// ============== Login 测试 ==============

// TestAdminLogin_Success 验证登录成功路径：生成 token + 写入 tokenStore
func TestAdminLogin_Success(t *testing.T) {
	// 强制初始化预计算 hash（避免首次调用 panic）
	_ = hashedTestPassword(t)

	svc, repo, _, store, _ := newTestAdminService(t)
	repo.ByUsername = enabledAdmin(1, "admin")

	vo, err := svc.Login(context.Background(), &systemDto.LoginReq{
		Username: "admin",
		Password: "Admin@12345",
	})

	require.NoError(t, err)
	assert.NotNil(t, vo)
	assert.NotEmpty(t, vo.Token, "应生成 access token")
	assert.NotEmpty(t, vo.RefreshToken, "应生成 refresh token")
	assert.Equal(t, 2, store.createCalls, "应写入 access + refresh 两个 hash")
}

// TestAdminLogin_UserNotFound 验证用户不存在时返回统一文案避免枚举
func TestAdminLogin_UserNotFound(t *testing.T) {
	svc, repo, _, _, _ := newTestAdminService(t)
	repo.ByUsername = nil
	repo.GetByUsernameErr = gorm.ErrRecordNotFound

	_, err := svc.Login(context.Background(), &systemDto.LoginReq{
		Username: "ghost",
		Password: "any",
	})

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr), "应为 BizError")
	assert.Equal(t, errorx.CodeUserNotFound, bizErr.Code, "错误码应保留 CodeUserNotFound")
	assert.Equal(t, "用户名或密码错误", bizErr.Message, "msg 应统一为「用户名或密码错误」避免枚举")
}

// TestAdminLogin_PasswordWrong 验证密码错误时返回统一文案避免枚举
func TestAdminLogin_PasswordWrong(t *testing.T) {
	_ = hashedTestPassword(t)
	svc, repo, _, _, _ := newTestAdminService(t)
	repo.ByUsername = enabledAdmin(1, "admin")

	_, err := svc.Login(context.Background(), &systemDto.LoginReq{
		Username: "admin",
		Password: "wrong-password",
	})

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodePasswordWrong, bizErr.Code, "错误码应保留 CodePasswordWrong 便于审计")
	assert.Equal(t, "用户名或密码错误", bizErr.Message, "msg 应统一避免枚举")
}

// TestAdminLogin_UserDisabled 验证禁用账户返回统一文案避免枚举
func TestAdminLogin_UserDisabled(t *testing.T) {
	_ = hashedTestPassword(t)
	svc, repo, _, _, _ := newTestAdminService(t)
	admin := enabledAdmin(2, "disabled")
	admin.Status = entity.StatusDisabled
	repo.ByUsername = admin

	_, err := svc.Login(context.Background(), &systemDto.LoginReq{
		Username: "disabled",
		Password: "Admin@12345",
	})

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUserDisabled, bizErr.Code, "错误码应保留 CodeUserDisabled 便于审计")
	assert.Equal(t, "用户名或密码错误", bizErr.Message, "msg 应统一避免枚举")
}

// TestAdminLogin_AccountLocked 验证账户已锁定时拒绝登录
func TestAdminLogin_AccountLocked(t *testing.T) {
	_ = hashedTestPassword(t)
	svc, repo, cacheMgr, _, _ := newTestAdminService(t)
	repo.ByUsername = enabledAdmin(1, "admin")
	// 预设 lockKey 命中 → 账户已锁定
	require.NoError(t, cacheMgr.Set(context.Background(),
		"admin:login:lock:admin", "1", 15*time.Minute))

	_, err := svc.Login(context.Background(), &systemDto.LoginReq{
		Username: "admin",
		Password: "Admin@12345",
	})

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUserLocked, bizErr.Code)
}

// ============== Logout 测试 ==============

// TestAdminLogout_WritesRefreshTokenToBlacklist 验证 Logout 将 refresh token 写入黑名单
func TestAdminLogout_WritesRefreshTokenToBlacklist(t *testing.T) {
	_ = hashedTestPassword(t)
	svc, repo, cacheMgr, store, j := newTestAdminService(t)
	admin := enabledAdmin(3, "logout")
	repo.AdminByID = admin

	// 先签发一个真实 refresh token，便于 Logout 解析 ExpiresAt
	claims := j.NewAdminClaims(admin.ID, admin.Username, nil, jwt.RefreshToken, admin.TokenVersion)
	refresh, err := j.GenerateToken(claims)
	require.NoError(t, err)

	err = svc.Logout(context.Background(), admin.ID, "access-token-stub", refresh)
	require.NoError(t, err)

	// 验证：access token hash 被删除
	assert.Equal(t, 1, store.deleteCalls, "应删除 access token hash")
	// 验证：refresh token 加入黑名单
	blacklistKey := "auth:blacklist:refresh:" + refresh
	exists, err := cacheMgr.Exists(context.Background(), blacklistKey)
	require.NoError(t, err)
	assert.True(t, exists, "refresh token 应写入黑名单")
}

// TestAdminLogout_NoRefreshToken_NoBlacklist 验证 refreshToken 为空时不写黑名单
func TestAdminLogout_NoRefreshToken_NoBlacklist(t *testing.T) {
	svc, repo, _, store, _ := newTestAdminService(t)
	repo.AdminByID = enabledAdmin(4, "logout2")

	err := svc.Logout(context.Background(), 4, "access-stub", "")
	require.NoError(t, err)
	assert.Equal(t, 1, store.deleteCalls, "应仍调用 access hash 删除")
}

// ============== RefreshToken 测试 ==============

// TestAdminRefreshToken_Success 验证刷新成功：签发新对、旧 refresh 加入黑名单、tokenStore 替换
func TestAdminRefreshToken_Success(t *testing.T) {
	_ = hashedTestPassword(t)
	svc, repo, cacheMgr, store, j := newTestAdminService(t)
	admin := enabledAdmin(5, "refresh")
	repo.AdminByID = admin

	// 签发一个真实 refresh token（未过期 + 未加入黑名单 + token_version 匹配）
	claims := j.NewAdminClaims(admin.ID, admin.Username, nil, jwt.RefreshToken, admin.TokenVersion)
	refresh, err := j.GenerateToken(claims)
	require.NoError(t, err)

	vo, err := svc.RefreshToken(context.Background(), refresh)

	require.NoError(t, err)
	assert.NotNil(t, vo)
	assert.NotEmpty(t, vo.Token, "应签发新 access token")
	assert.NotEmpty(t, vo.RefreshToken, "应签发新 refresh token")
	assert.NotEqual(t, refresh, vo.RefreshToken, "新 refresh token 应不同于旧值")

	// 验证：旧 refresh token 加入黑名单
	blacklistKey := "auth:blacklist:refresh:" + refresh
	exists, _ := cacheMgr.Exists(context.Background(), blacklistKey)
	assert.True(t, exists, "旧 refresh token 应加入黑名单")

	// 验证：tokenStore 替换会话（先 Delete 旧 refresh，再 Create 新 access + 新 refresh）
	assert.Equal(t, 1, store.deleteCalls, "应 Delete 旧 refresh hash")
	assert.Equal(t, 2, store.createCalls, "应 Create 新 access + refresh 两个 hash")
}

// TestAdminRefreshToken_InvalidTokenRejected 验证无效 token（解析失败）被拒绝
//
// 注：jwt 库对过期 token 与格式错误 token 都走 ParseToken 失败路径，
// 因此用一个格式错误的 token 覆盖「过期」「格式错误」「签名不匹配」等场景。
func TestAdminRefreshToken_InvalidTokenRejected(t *testing.T) {
	_ = hashedTestPassword(t)
	svc, _, _, _, _ := newTestAdminService(t)

	_, err := svc.RefreshToken(context.Background(), "invalid.refresh.token")
	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUnauthorized, bizErr.Code)
	assert.Contains(t, bizErr.Message, "刷新令牌无效")
}

// TestAdminRefreshToken_BlacklistedRejected 验证 refresh token 在黑名单中被拒绝
func TestAdminRefreshToken_BlacklistedRejected(t *testing.T) {
	_ = hashedTestPassword(t)
	svc, repo, cacheMgr, _, j := newTestAdminService(t)
	admin := enabledAdmin(6, "blacklisted")
	repo.AdminByID = admin

	claims := j.NewAdminClaims(admin.ID, admin.Username, nil, jwt.RefreshToken, admin.TokenVersion)
	refresh, err := j.GenerateToken(claims)
	require.NoError(t, err)

	// 预设黑名单命中
	require.NoError(t, cacheMgr.Set(context.Background(),
		"auth:blacklist:refresh:"+refresh, "1", time.Hour))

	_, err = svc.RefreshToken(context.Background(), refresh)
	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUnauthorized, bizErr.Code)
	assert.Contains(t, bizErr.Message, "已失效")
}

// TestAdminRefreshToken_UserDisabled 验证禁用账户的 refresh token 被拒绝
func TestAdminRefreshToken_UserDisabled(t *testing.T) {
	_ = hashedTestPassword(t)
	svc, repo, _, _, j := newTestAdminService(t)
	admin := enabledAdmin(7, "refresh-disabled")
	admin.Status = entity.StatusDisabled
	repo.AdminByID = admin

	claims := j.NewAdminClaims(admin.ID, admin.Username, nil, jwt.RefreshToken, admin.TokenVersion)
	refresh, err := j.GenerateToken(claims)
	require.NoError(t, err)

	_, err = svc.RefreshToken(context.Background(), refresh)
	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUserDisabled, bizErr.Code)
}
