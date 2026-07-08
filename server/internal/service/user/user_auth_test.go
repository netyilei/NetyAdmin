// user_auth_test.go 客户端用户认证 Service 单元测试基线。
//
// 覆盖范围（Task 9.2）：
//   - Login：成功登录 / 用户不存在 / 用户已禁用 / 密码错误 / 账户已锁定
//   - Logout：refresh token 写入黑名单
//   - RefreshToken：成功刷新 / token 无效 / token 已加入黑名单 / 用户已禁用
//
// Mock 策略：与 admin_auth_test.go 一致——
//   - repo / cacheSlow / tokenStore / verifySvc / configWatcher / captchaStore 使用手写 mock
//   - jwt 使用真实 *jwt.JWT 实例（RS256 + 测试生成的 RSA 密钥对）
//   - password 使用真实 bcrypt（预计算 hash 加速）
//   - tm 设为 nil（Login/Logout/RefreshToken 不使用 TM；ChangePassword 才用，不在本测试范围）
//
// 注：图形验证码 / 短信验证码场景在 Login 路径上由 configWatcher + verifySvc 控制，
// 通过设置 captcha_config.user_login_enabled="false" 关闭图形验证码，
// verifySvc.GetVerifyConfig 返回 nil 跳过短信验证码，从而聚焦于核心鉴权路径。
package user

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mojocn/base64Captcha"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	userEntity "NetyAdmin/internal/domain/entity/user"
	clientDto "NetyAdmin/internal/interface/client/dto/v1"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/password"
	userRepo "NetyAdmin/internal/repository/user"
)

// ============== mockUserCacheMgr：cache.SecurityCache 内存实现 ==============
type mockUserCacheMgr struct {
	mu       sync.Mutex
	values   map[string]string
	counters map[string]int64
	incrErr  error
}

func newMockUserCacheMgr() *mockUserCacheMgr {
	return &mockUserCacheMgr{
		values:   make(map[string]string),
		counters: make(map[string]int64),
	}
}

func (m *mockUserCacheMgr) Get(_ context.Context, key string, dest interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	val, ok := m.values[key]
	if !ok {
		return errors.New("not found")
	}
	if s, ok := dest.(*string); ok {
		*s = val
	}
	return nil
}

func (m *mockUserCacheMgr) Set(_ context.Context, key string, value interface{}, _ time.Duration, _ ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := value.(string); ok {
		m.values[key] = s
	} else {
		m.values[key] = fmt.Sprintf("%v", value)
	}
	return nil
}

func (m *mockUserCacheMgr) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, key)
	delete(m.counters, key)
	return nil
}

func (m *mockUserCacheMgr) Exists(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.values[key]
	if !ok {
		_, ok = m.counters[key]
	}
	return ok, nil
}

func (m *mockUserCacheMgr) Incr(_ context.Context, key string, _ time.Duration) (int64, error) {
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

// 以下方法 user_auth 不使用，仅满足接口签名
func (m *mockUserCacheMgr) Fetch(_ context.Context, _ string, _ string, _ []string, _ time.Duration, _ interface{}, _ func() (interface{}, error)) error {
	return nil
}
func (m *mockUserCacheMgr) InvalidateByTags(_ context.Context, _ ...string) error { return nil }
func (m *mockUserCacheMgr) SetNX(_ context.Context, _ string, _ interface{}, _ time.Duration) (bool, error) {
	return false, nil
}
func (m *mockUserCacheMgr) IsCacheEnabled(_ string) bool { return true }

var _ cache.SecurityCache = (*mockUserCacheMgr)(nil)

// ============== mockUserTokenStore：TokenStore 内存实现 ==============
type mockUserTokenStore struct {
	createCalls  int
	deleteCalls  int
	createErr    error
	deleteErr    error
}

func (s *mockUserTokenStore) Create(_ context.Context, _ *userEntity.UserTokenHash) error {
	if s.createErr != nil {
		err := s.createErr
		s.createErr = nil
		return err
	}
	s.createCalls++
	return nil
}

func (s *mockUserTokenStore) Delete(_ context.Context, _, _ string) error {
	if s.deleteErr != nil {
		err := s.deleteErr
		s.deleteErr = nil
		return err
	}
	s.deleteCalls++
	return nil
}

func (s *mockUserTokenStore) Get(_ context.Context, _, _ string) (*userEntity.UserTokenHash, error) {
	return nil, errors.New("not implemented")
}

func (s *mockUserTokenStore) DeleteAll(_ context.Context, _ string) error { return nil }

var _ TokenStore = (*mockUserTokenStore)(nil)

// ============== mockUserRepo：userRepo.UserRepository 内存实现 ==============
//
// 仅实现 user_auth 用到的方法（GetByUsername/GetByID/UpdateFields）。
type mockUserRepo struct {
	ByUsername       *userEntity.User
	GetByUsernameErr error
	UserByID         *userEntity.User
	GetByIDErr       error
	UpdateFieldsErr  error
}

func (r *mockUserRepo) GetByUsername(_ context.Context, _ string) (*userEntity.User, error) {
	if r.GetByUsernameErr != nil {
		return nil, r.GetByUsernameErr
	}
	return r.ByUsername, nil
}

func (r *mockUserRepo) GetByID(_ context.Context, _ string) (*userEntity.User, error) {
	if r.GetByIDErr != nil {
		return nil, r.GetByIDErr
	}
	return r.UserByID, nil
}

func (r *mockUserRepo) UpdateFields(_ context.Context, _ string, _ map[string]interface{}) error {
	return r.UpdateFieldsErr
}

// 以下方法 user_auth 不使用，仅满足接口签名
func (r *mockUserRepo) Create(_ context.Context, _ *userEntity.User) error { return nil }
func (r *mockUserRepo) GetByPhone(_ context.Context, _ string) (*userEntity.User, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *mockUserRepo) GetByEmail(_ context.Context, _ string) (*userEntity.User, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *mockUserRepo) ExistsByUsername(_ context.Context, _ string, _ ...string) (bool, error) {
	return false, nil
}
func (r *mockUserRepo) ExistsByPhone(_ context.Context, _ string, _ ...string) (bool, error) {
	return false, nil
}
func (r *mockUserRepo) ExistsByEmail(_ context.Context, _ string, _ ...string) (bool, error) {
	return false, nil
}
func (r *mockUserRepo) List(_ context.Context, _ *userRepo.UserRepoQuery) ([]userEntity.User, int64, error) {
	return nil, 0, nil
}
func (r *mockUserRepo) SearchForAutocomplete(_ context.Context, _ string, _ int) ([]userEntity.User, error) {
	return nil, nil
}
func (r *mockUserRepo) Update(_ context.Context, _ *userEntity.User) error              { return nil }
func (r *mockUserRepo) Delete(_ context.Context, _ string) error                        { return nil }
func (r *mockUserRepo) DeleteBatch(_ context.Context, _ []string) error                { return nil }
func (r *mockUserRepo) IncrementTokenVersion(_ context.Context, _ string) error         { return nil }
func (r *mockUserRepo) CreateTokenHash(_ context.Context, _ *userEntity.UserTokenHash) error { return nil }
func (r *mockUserRepo) GetTokenHash(_ context.Context, _, _ string) (*userEntity.UserTokenHash, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *mockUserRepo) DeleteTokenHash(_ context.Context, _, _ string) error            { return nil }
func (r *mockUserRepo) DeleteAllTokenHashes(_ context.Context, _ string) error          { return nil }
func (r *mockUserRepo) DeleteExpiredTokenHashes(_ context.Context) (int64, error)       { return 0, nil }

var _ userRepo.UserRepository = (*mockUserRepo)(nil)

// ============== mockVerifySvc：VerificationService 内存实现 ==============
//
// 默认行为：GetVerifyConfig 返回 (nil, nil)，使 Login 跳过短信/邮箱验证码路径，
// 聚焦于密码鉴权核心逻辑。需要测试验证码场景时，覆写 VerifyConfig 字段。
type mockVerifySvc struct {
	verifyConfig *VerifyConfig
	verifyResult bool
	verifyErr    error
}

func (s *mockVerifySvc) GetVerifyConfig(_ context.Context, _ string) (*VerifyConfig, error) {
	return s.verifyConfig, nil
}

func (s *mockVerifySvc) GetSceneCaptchaConfig(_ context.Context, _ string) (*SceneCaptchaConfig, error) {
	return nil, nil
}

func (s *mockVerifySvc) SendCode(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (s *mockVerifySvc) VerifyCode(_ context.Context, _, _, _ string) (bool, error) {
	return s.verifyResult, s.verifyErr
}

func (s *mockVerifySvc) VerifyAndClearCode(_ context.Context, _, _, _ string) (bool, error) {
	return s.verifyResult, s.verifyErr
}

var _ VerificationService = (*mockVerifySvc)(nil)

// ============== mockConfigWatcher：configsync.ConfigWatcher 内存实现 ==============
type mockConfigWatcher struct {
	configs map[string]string
}

func (w *mockConfigWatcher) GetConfig(group, key string) (string, bool) {
	v, ok := w.configs[group+":"+key]
	return v, ok
}

func (w *mockConfigWatcher) GetGroupConfigs(group string) map[string]string {
	out := make(map[string]string)
	for k, v := range w.configs {
		// 简化实现，不严格按 group 解析
		out[k] = v
	}
	return out
}

func (w *mockConfigWatcher) IsCacheEnabled(_ string) bool { return true }
func (w *mockConfigWatcher) ForceReload(_ context.Context) error { return nil }

var _ configsync.ConfigWatcher = (*mockConfigWatcher)(nil)

// ============== mockCaptchaStore：base64Captcha.Store 内存实现 ==============
type mockCaptchaStore struct {
	verifyResult bool
}

func (s *mockCaptchaStore) Set(_, _ string) error                 { return nil }
func (s *mockCaptchaStore) Get(_ string, _ bool) string           { return "" }
func (s *mockCaptchaStore) Verify(_ string, _ string, _ bool) bool {
	return s.verifyResult
}

var _ base64Captcha.Store = (*mockCaptchaStore)(nil)

// ============== 测试夹具 ==============

// testUserRSAPrivKey / testUserRSAPubKey 是测试用 RSA 密钥对（RS256 签名）。
// 在 newTestUserClientService 中惰性生成一次，后续用例复用。
var (
	testUserRSAPrivKey *rsa.PrivateKey
	testUserRSAPubKey  *rsa.PublicKey
	testUserRSAOnce    sync.Once
)

func newTestUserClientService(t *testing.T) (*userClientService, *mockUserRepo, *mockUserCacheMgr, *mockUserTokenStore, *mockVerifySvc, *jwt.JWT) {
	t.Helper()
	testUserRSAOnce.Do(func() {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("生成测试 RSA 密钥对失败: %v", err)
		}
		testUserRSAPrivKey = priv
		testUserRSAPubKey = &priv.PublicKey
	})
	// access TTL 1h（方便测试中 token 不易过期），refresh TTL 2h
	j, err := jwt.New(testUserRSAPrivKey, testUserRSAPubKey, time.Hour, 2*time.Hour)
	require.NoError(t, err)

	repo := &mockUserRepo{}
	cacheMgr := newMockUserCacheMgr()
	store := &mockUserTokenStore{}
	verifySvc := &mockVerifySvc{} // 默认 verifyConfig=nil → 跳过短信验证码
	watcher := &mockConfigWatcher{
		configs: map[string]string{
			// 默认关闭图形验证码（聚焦核心鉴权路径）
			"captcha_config:user_login_enabled": "false",
			// 默认登录失败重试配置
			"user_config:login_max_retry":     "5",
			"user_config:login_lock_duration":  "900", // 15min
		},
	}

	svc := &userClientService{
		userBase: userBase{
			repo:          repo,
			jwt:           j,
			verifySvc:     verifySvc,
			configWatcher: watcher,
			captchaStore:  &mockCaptchaStore{verifyResult: false},
			tokenStore:    store,
			cacheSlow:      cacheMgr,
		},
	}
	return svc, repo, cacheMgr, store, verifySvc, j
}

// hashedUserPassword 返回 "User@12345" 的 bcrypt hash（仅计算一次）
var (
	hashedUserPasswordOnce sync.Once
	hashedUserPasswordVal  string
)

func hashedUserPassword(t *testing.T) string {
	t.Helper()
	hashedUserPasswordOnce.Do(func() {
		h, err := password.Hash("User@12345")
		if err != nil {
			t.Fatalf("precompute user password hash: %v", err)
		}
		hashedUserPasswordVal = h
	})
	return hashedUserPasswordVal
}

// enabledUser 构造一个启用状态的用户
func enabledUser(id, username string) *userEntity.User {
	return &userEntity.User{
		ID:           id,
		Username:     username,
		Password:     hashedUserPasswordVal,
		Status:       entity.StatusEnabled,
		TokenVersion: 1,
	}
}

// ============== Login 测试 ==============

func TestUserLogin_Success(t *testing.T) {
	_ = hashedUserPassword(t)
	svc, repo, _, store, _, _ := newTestUserClientService(t)
	repo.ByUsername = enabledUser("01HTESTUSERLOGIN0001", "alice")

	vo, err := svc.Login(context.Background(), &clientDto.UserLoginReq{
		Username: "alice",
		Password: "User@12345",
	}, "127.0.0.1")

	require.NoError(t, err)
	assert.NotNil(t, vo)
	assert.NotEmpty(t, vo.AccessToken)
	assert.NotEmpty(t, vo.RefreshToken)
	assert.Equal(t, 2, store.createCalls, "应写入 access + refresh 两个 hash")
}

func TestUserLogin_UserNotFound(t *testing.T) {
	svc, repo, _, _, _, _ := newTestUserClientService(t)
	repo.ByUsername = nil
	repo.GetByUsernameErr = gorm.ErrRecordNotFound

	_, err := svc.Login(context.Background(), &clientDto.UserLoginReq{
		Username: "ghost",
		Password: "any",
	}, "127.0.0.1")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUserNotFound, bizErr.Code, "错误码应保留 CodeUserNotFound 便于审计")
	assert.Equal(t, "用户名或密码错误", bizErr.Message, "msg 应统一避免枚举")
}

func TestUserLogin_PasswordWrong(t *testing.T) {
	_ = hashedUserPassword(t)
	svc, repo, _, _, _, _ := newTestUserClientService(t)
	repo.ByUsername = enabledUser("01HTESTUSERLOGIN0002", "bob")

	_, err := svc.Login(context.Background(), &clientDto.UserLoginReq{
		Username: "bob",
		Password: "wrong-password",
	}, "127.0.0.1")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodePasswordWrong, bizErr.Code)
	assert.Equal(t, "用户名或密码错误", bizErr.Message)
}

func TestUserLogin_UserDisabled(t *testing.T) {
	_ = hashedUserPassword(t)
	svc, repo, _, _, _, _ := newTestUserClientService(t)
	user := enabledUser("01HTESTUSERLOGIN0003", "disabled")
	user.Status = entity.StatusDisabled
	repo.ByUsername = user

	_, err := svc.Login(context.Background(), &clientDto.UserLoginReq{
		Username: "disabled",
		Password: "User@12345",
	}, "127.0.0.1")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUserDisabled, bizErr.Code)
	assert.Equal(t, "用户名或密码错误", bizErr.Message)
}

func TestUserLogin_AccountLocked(t *testing.T) {
	_ = hashedUserPassword(t)
	svc, repo, cacheMgr, _, _, _ := newTestUserClientService(t)
	user := enabledUser("01HTESTUSERLOGIN0004", "locked")
	repo.ByUsername = user

	// 预设登录锁命中
	require.NoError(t, cacheMgr.Set(context.Background(),
		"auth:lock:"+user.ID, "1", 15*time.Minute))

	_, err := svc.Login(context.Background(), &clientDto.UserLoginReq{
		Username: "locked",
		Password: "User@12345",
	}, "127.0.0.1")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUserLocked, bizErr.Code)
}

// ============== Logout 测试 ==============

func TestUserLogout_WritesRefreshTokenToBlacklist(t *testing.T) {
	_ = hashedUserPassword(t)
	svc, repo, cacheMgr, store, _, j := newTestUserClientService(t)
	user := enabledUser("01HTESTUSERLOGOUT0001", "logout")
	repo.UserByID = user

	claims := j.NewUserClaims(user.ID, "web", jwt.RefreshToken, user.TokenVersion)
	refresh, err := j.GenerateToken(claims)
	require.NoError(t, err)

	err = svc.Logout(context.Background(), user.ID, "access-stub", refresh)
	require.NoError(t, err)

	assert.Equal(t, 1, store.deleteCalls, "应删除 access token hash")
	blacklistKey := "auth:blacklist:refresh:" + refresh
	exists, _ := cacheMgr.Exists(context.Background(), blacklistKey)
	assert.True(t, exists, "refresh token 应写入黑名单")
}

func TestUserLogout_NoRefreshToken(t *testing.T) {
	svc, repo, _, store, _, _ := newTestUserClientService(t)
	user := enabledUser("01HTESTUSERLOGOUT0002", "logout2")
	repo.UserByID = user

	err := svc.Logout(context.Background(), user.ID, "access-stub", "")
	require.NoError(t, err)
	assert.Equal(t, 1, store.deleteCalls)
}

// ============== RefreshToken 测试 ==============

func TestUserRefreshToken_Success(t *testing.T) {
	_ = hashedUserPassword(t)
	svc, repo, cacheMgr, store, _, j := newTestUserClientService(t)
	user := enabledUser("01HTESTUSERREFRESH001", "refresh")
	repo.UserByID = user

	claims := j.NewUserClaims(user.ID, "web", jwt.RefreshToken, user.TokenVersion)
	refresh, err := j.GenerateToken(claims)
	require.NoError(t, err)

	vo, err := svc.RefreshToken(context.Background(), refresh)
	require.NoError(t, err)
	assert.NotNil(t, vo)
	assert.NotEmpty(t, vo.AccessToken)
	assert.NotEmpty(t, vo.RefreshToken)
	// 注：JWT 使用秒级 exp/iat，同一秒内签发的同类型 token 字符串可能相同；
	// 服务保证「签发新对 + 旧 refresh 黑名单」即可，不强制要求字符串不同。

	// 旧 refresh 加入黑名单
	blacklistKey := "auth:blacklist:refresh:" + refresh
	exists, _ := cacheMgr.Exists(context.Background(), blacklistKey)
	assert.True(t, exists)

	// tokenStore 替换会话
	assert.Equal(t, 1, store.deleteCalls, "应 Delete 旧 refresh hash")
	assert.Equal(t, 2, store.createCalls, "应 Create 新 access + refresh 两个 hash")
}

func TestUserRefreshToken_InvalidTokenRejected(t *testing.T) {
	svc, _, _, _, _, _ := newTestUserClientService(t)

	_, err := svc.RefreshToken(context.Background(), "invalid.refresh.token")
	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUnauthorized, bizErr.Code)
	assert.Contains(t, bizErr.Message, "刷新令牌无效")
}

func TestUserRefreshToken_BlacklistedRejected(t *testing.T) {
	_ = hashedUserPassword(t)
	svc, repo, cacheMgr, _, _, j := newTestUserClientService(t)
	user := enabledUser("01HTESTUSERREFRESH002", "blacklisted")
	repo.UserByID = user

	claims := j.NewUserClaims(user.ID, "web", jwt.RefreshToken, user.TokenVersion)
	refresh, err := j.GenerateToken(claims)
	require.NoError(t, err)

	require.NoError(t, cacheMgr.Set(context.Background(),
		"auth:blacklist:refresh:"+refresh, "1", time.Hour))

	_, err = svc.RefreshToken(context.Background(), refresh)
	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUnauthorized, bizErr.Code)
	assert.Contains(t, bizErr.Message, "已失效")
}

func TestUserRefreshToken_UserDisabled(t *testing.T) {
	_ = hashedUserPassword(t)
	svc, repo, _, _, _, j := newTestUserClientService(t)
	user := enabledUser("01HTESTUSERREFRESH003", "refresh-disabled")
	user.Status = entity.StatusDisabled
	repo.UserByID = user

	claims := j.NewUserClaims(user.ID, "web", jwt.RefreshToken, user.TokenVersion)
	refresh, err := j.GenerateToken(claims)
	require.NoError(t, err)

	_, err = svc.RefreshToken(context.Background(), refresh)
	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUserDisabled, bizErr.Code)
}

// ============== 验证码场景（轻量覆盖） ==============

// TestUserLogin_CaptchaRequired 验证：开启图形验证码后，未提供 captcha 时返回 CodeCaptchaRequired
func TestUserLogin_CaptchaRequired(t *testing.T) {
	_ = hashedUserPassword(t)
	svc, repo, _, _, _, _ := newTestUserClientService(t)
	repo.ByUsername = enabledUser("01HTESTUSERCAPTCHA01", "cap")

	// 在 svc 内部 watcher 上注入：开启图形验证码
	// 由于 watcher 是 svc.configWatcher 接口类型，需要重新构造一个 watcher
	svc.configWatcher = &mockConfigWatcher{
		configs: map[string]string{
			"captcha_config:user_login_enabled": "true",
		},
	}

	_, err := svc.Login(context.Background(), &clientDto.UserLoginReq{
		Username:    "cap",
		Password:    "User@12345",
		CaptchaKey:  "", // 缺失
		CaptchaCode: "",
	}, "127.0.0.1")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeCaptchaRequired, bizErr.Code)
}

// TestUserLogin_CaptchaInvalid 验证：图形验证码错误时返回 CodeCaptchaInvalid
func TestUserLogin_CaptchaInvalid(t *testing.T) {
	_ = hashedUserPassword(t)
	svc, repo, _, _, _, _ := newTestUserClientService(t)
	repo.ByUsername = enabledUser("01HTESTUSERCAPTCHA02", "cap2")

	// 替换 watcher：开启图形验证码
	svc.configWatcher = &mockConfigWatcher{
		configs: map[string]string{
			"captcha_config:user_login_enabled": "true",
		},
	}
	// captchaStore.Verify 默认返回 false（mockCaptchaStore 初始化时设置）

	_, err := svc.Login(context.Background(), &clientDto.UserLoginReq{
		Username:    "cap2",
		Password:    "User@12345",
		CaptchaKey:  "captcha-key-1",
		CaptchaCode: "wrong-code",
	}, "127.0.0.1")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeCaptchaInvalid, bizErr.Code)
}
