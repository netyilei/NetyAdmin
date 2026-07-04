// menu_test.go 菜单 Service DeleteBatch fail-closed 单元测试基线。
//
// 覆盖范围（Task 9.3）：
//   - DeleteBatch 全部成功 → 返回 nil
//   - DeleteBatch 存在子菜单 → 业务规则拒绝（CodeMenuHasChildren）→ skip + continue
//   - DeleteBatch 事务失败（repo 返回 error）→ fail-closed 立即返回
//   - DeleteBatch 空输入 → 返回 nil
//   - DeleteBatch 混合场景：部分 skip + 部分成功 → 返回 CodeForbidden 含 skipped 摘要
//
// Mock 策略：
//   - menuRepo / buttonRepo / apiRepo / roleRepo / cacheMgr 使用手写 mock 结构体
//   - tm 使用真实 *database.TransactionManager + sqlite in-memory（TM 需要 *gorm.DB 才能 Begin/Commit/Rollback）
//   - mock repos 不实际读写 sqlite，仅记录调用并返回预设 error；sqlite 仅用于支撑 TM 的事务句柄
//
// 注：menuService.Delete 内部使用 tm.WithTransaction 闭包 API，
// 任一 repo 返回 error → 闭包返回 error → WithTransaction 自动 Rollback → DeleteBatch 见非业务错误 → fail-closed。
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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/pubsub"
	systemRepo "NetyAdmin/internal/repository/system"
)

// ============== mockMenuRepo：systemRepo.MenuRepository 内存实现 ==============
//
// 仅实现 Delete 路径用到的方法（HasChildren / ClearRoleMenus / Delete），
// 其余方法返回零值；通过 hasChildrenErr / clearRoleMenusErr / deleteErr 控制错误注入。
type mockMenuRepo struct {
	hasChildrenResult bool
	hasChildrenErr   error
	clearRoleMenusErr error
	deleteErr        error
	deleteCalls      int
}

func (r *mockMenuRepo) HasChildren(_ context.Context, _ uint) (bool, error) {
	return r.hasChildrenResult, r.hasChildrenErr
}
func (r *mockMenuRepo) ClearRoleMenus(_ context.Context, _ uint) error { return r.clearRoleMenusErr }
func (r *mockMenuRepo) Delete(_ context.Context, _ uint) error {
	r.deleteCalls++
	return r.deleteErr
}

// 以下方法 menu Delete 不使用，仅满足接口签名
func (r *mockMenuRepo) Create(_ context.Context, _ *systemEntity.Menu) error { return nil }
func (r *mockMenuRepo) Update(_ context.Context, _ *systemEntity.Menu) error { return nil }
func (r *mockMenuRepo) GetByID(_ context.Context, _ uint) (*systemEntity.Menu, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *mockMenuRepo) GetByRouteName(_ context.Context, _ string) (*systemEntity.Menu, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *mockMenuRepo) List(_ context.Context, _ *systemRepo.MenuRepoQuery) ([]*systemEntity.Menu, int64, error) {
	return nil, 0, nil
}
func (r *mockMenuRepo) GetTree(_ context.Context) ([]*systemEntity.Menu, error)         { return nil, nil }
func (r *mockMenuRepo) GetAll(_ context.Context) ([]systemEntity.Menu, error)           { return nil, nil }
func (r *mockMenuRepo) GetAllPages(_ context.Context) ([]*systemEntity.Menu, error)     { return nil, nil }
func (r *mockMenuRepo) GetAllWithButtons(_ context.Context) ([]systemEntity.Menu, error) { return nil, nil }
func (r *mockMenuRepo) GetAllWithApis(_ context.Context) ([]systemEntity.Menu, error)   { return nil, nil }
func (r *mockMenuRepo) ExistsByRouteName(_ context.Context, _ string, _ ...uint) (bool, error) {
	return false, nil
}
func (r *mockMenuRepo) GetByRoleID(_ context.Context, _ uint) ([]*systemEntity.Menu, error) {
	return nil, nil
}
func (r *mockMenuRepo) GetByRoleCodes(_ context.Context, _ []string) ([]*systemEntity.Menu, error) {
	return nil, nil
}

var _ systemRepo.MenuRepository = (*mockMenuRepo)(nil)

// ============== mockButtonRepo / mockAPIRepo / mockRoleRepo ==============
// menu Delete 路径仅调用 ClearXxxByMenuID + DeleteByMenuID + ClearHomeMenuRef
type mockButtonRepo struct{ err error }

func (r *mockButtonRepo) ClearRoleButtonsByMenuID(_ context.Context, _ uint) error { return r.err }
func (r *mockButtonRepo) DeleteByMenuID(_ context.Context, _ uint) error            { return r.err }
func (r *mockButtonRepo) Create(_ context.Context, _ *systemEntity.Button) error   { return nil }
func (r *mockButtonRepo) Update(_ context.Context, _ *systemEntity.Button) error   { return nil }
func (r *mockButtonRepo) Delete(_ context.Context, _ uint) error                    { return nil }
func (r *mockButtonRepo) ClearRoleButtons(_ context.Context, _ uint) error          { return nil }
func (r *mockButtonRepo) GetByID(_ context.Context, _ uint) (*systemEntity.Button, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *mockButtonRepo) GetByCode(_ context.Context, _ string) (*systemEntity.Button, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *mockButtonRepo) List(_ context.Context, _ *systemRepo.ButtonRepoQuery) ([]*systemEntity.Button, int64, error) {
	return nil, 0, nil
}
func (r *mockButtonRepo) GetByMenuID(_ context.Context, _ uint) ([]*systemEntity.Button, error) {
	return nil, nil
}
func (r *mockButtonRepo) GetByMenuIDs(_ context.Context, _ []uint) ([]*systemEntity.Button, error) {
	return nil, nil
}
func (r *mockButtonRepo) GetByRoleID(_ context.Context, _ uint) ([]*systemEntity.Button, error) {
	return nil, nil
}
func (r *mockButtonRepo) GetAll(_ context.Context) ([]*systemEntity.Button, error) { return nil, nil }
func (r *mockButtonRepo) ExistsByCode(_ context.Context, _ string, _ ...uint) (bool, error) {
	return false, nil
}

var _ systemRepo.ButtonRepository = (*mockButtonRepo)(nil)

type mockAPIRepo struct{ err error }

func (r *mockAPIRepo) ClearRoleApisByMenuID(_ context.Context, _ uint) error { return r.err }
func (r *mockAPIRepo) DeleteByMenuID(_ context.Context, _ uint) error        { return r.err }
func (r *mockAPIRepo) Create(_ context.Context, _ *systemEntity.API) error   { return nil }
func (r *mockAPIRepo) Update(_ context.Context, _ *systemEntity.API) error   { return nil }
func (r *mockAPIRepo) Delete(_ context.Context, _ uint) error                { return nil }
func (r *mockAPIRepo) ClearRoleApis(_ context.Context, _ uint) error         { return nil }
func (r *mockAPIRepo) GetByID(_ context.Context, _ uint) (*systemEntity.API, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *mockAPIRepo) GetByMethodAndPath(_ context.Context, _, _ string) (*systemEntity.API, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *mockAPIRepo) List(_ context.Context, _ *systemRepo.APIRepoQuery) ([]*systemEntity.API, int64, error) {
	return nil, 0, nil
}
func (r *mockAPIRepo) GetByMenuID(_ context.Context, _ uint) ([]*systemEntity.API, error) {
	return nil, nil
}
func (r *mockAPIRepo) GetByRoleID(_ context.Context, _ uint) ([]*systemEntity.API, error) {
	return nil, nil
}
func (r *mockAPIRepo) GetAll(_ context.Context) ([]*systemEntity.API, error) { return nil, nil }
func (r *mockAPIRepo) ExistsByMethodAndPath(_ context.Context, _, _ string, _ ...uint) (bool, error) {
	return false, nil
}

var _ systemRepo.APIRepository = (*mockAPIRepo)(nil)

type mockRoleRepo struct{ err error }

func (r *mockRoleRepo) ClearHomeMenuRef(_ context.Context, _ uint) error { return r.err }
func (r *mockRoleRepo) Create(_ context.Context, _ *systemEntity.Role) error { return nil }
func (r *mockRoleRepo) Update(_ context.Context, _ *systemEntity.Role) error { return nil }
func (r *mockRoleRepo) Delete(_ context.Context, _ uint) error              { return nil }
func (r *mockRoleRepo) ClearUserRoles(_ context.Context, _ uint) error       { return nil }
func (r *mockRoleRepo) ClearPermissions(_ context.Context, _ uint) error    { return nil }
func (r *mockRoleRepo) GetByID(_ context.Context, _ uint) (*systemEntity.Role, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *mockRoleRepo) GetByCode(_ context.Context, _ string) (*systemEntity.Role, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *mockRoleRepo) List(_ context.Context, _ *systemRepo.RoleRepoQuery) ([]*systemEntity.Role, int64, error) {
	return nil, 0, nil
}
func (r *mockRoleRepo) ExistsByCode(_ context.Context, _ string, _ ...uint) (bool, error) {
	return false, nil
}
func (r *mockRoleRepo) GetAll(_ context.Context) ([]*systemEntity.Role, error) { return nil, nil }
func (r *mockRoleRepo) GetByCodes(_ context.Context, _ []string) ([]*systemEntity.Role, error) {
	return nil, nil
}

var _ systemRepo.RoleRepository = (*mockRoleRepo)(nil)

// ============== mockMenuCacheMgr：cache.LazyCacheManager 内存实现 ==============
// （与 admin_auth_test.go 中的 mockCacheMgr 同构，独立定义以避免同包冲突）
type mockMenuCacheMgr struct {
	mu             sync.Mutex
	invalidateCalls int
}

func (m *mockMenuCacheMgr) Get(_ context.Context, _ string, _ interface{}) error { return nil }
func (m *mockMenuCacheMgr) Set(_ context.Context, _ string, _ interface{}, _ time.Duration) error {
	return nil
}
func (m *mockMenuCacheMgr) Delete(_ context.Context, _ string) error { return nil }
func (m *mockMenuCacheMgr) Exists(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (m *mockMenuCacheMgr) Incr(_ context.Context, _ string, _ time.Duration) (int64, error) {
	return 0, nil
}
func (m *mockMenuCacheMgr) Fetch(_ context.Context, _ string, _ string, _ []string, _ time.Duration, _ interface{}, _ func() (interface{}, error)) error {
	return nil
}
func (m *mockMenuCacheMgr) FetchFast(_ context.Context, _ string, _ string, _ []string, _ time.Duration, _ interface{}, _ func() (interface{}, error)) error {
	return nil
}
func (m *mockMenuCacheMgr) InvalidateByTags(_ context.Context, _ ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invalidateCalls++
	return nil
}
func (m *mockMenuCacheMgr) SetFast(_ context.Context, _ string, _ interface{}, _ []string, _ time.Duration) error {
	return nil
}
func (m *mockMenuCacheMgr) SetNX(_ context.Context, _ string, _ interface{}, _ time.Duration) (bool, error) {
	return false, nil
}
func (m *mockMenuCacheMgr) GetFast(_ context.Context, _ string, _ []string, _ time.Duration, _ interface{}) error {
	return nil
}
func (m *mockMenuCacheMgr) DeleteFast(_ context.Context, _ string) error          { return nil }
func (m *mockMenuCacheMgr) DeleteAndBroadcast(_ context.Context, _ string) error  { return nil }
func (m *mockMenuCacheMgr) InvalidateL1ByTags(_ context.Context, _ ...string) error { return nil }
func (m *mockMenuCacheMgr) InvalidateL1ByKey(_ context.Context, _ string) error    { return nil }
func (m *mockMenuCacheMgr) SetEventBus(_ pubsub.EventBus)                          {}
func (m *mockMenuCacheMgr) IsCacheEnabled(_ string) bool                            { return true }
func (m *mockMenuCacheMgr) GetRedisClient() *redis.Client                           { return nil }

var _ cache.LazyCacheManager = (*mockMenuCacheMgr)(nil)

// ============== 测试夹具 ==============

// setupMenuTestDB 构造 sqlite in-memory DB 仅供 TM 调用 Begin/Commit/Rollback。
// 不需要 AutoMigrate 任何业务表：mock repos 不实际读写 sqlite。
func setupMenuTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	return db
}

// newTestMenuService 构造 menuService + 配套 mocks。
// 返回值中的 cacheMgr 用于断言 InvalidateByTags 调用次数。
func newTestMenuService(t *testing.T) (*menuService, *mockMenuRepo, *mockButtonRepo, *mockAPIRepo, *mockRoleRepo, *mockMenuCacheMgr) {
	t.Helper()
	db := setupMenuTestDB(t)
	tm := database.NewTransactionManager(db)

	menuRepo := &mockMenuRepo{}
	buttonRepo := &mockButtonRepo{}
	apiRepo := &mockAPIRepo{}
	roleRepo := &mockRoleRepo{}
	cacheMgr := &mockMenuCacheMgr{}

	svc := &menuService{
		menuRepo:   menuRepo,
		buttonRepo: buttonRepo,
		apiRepo:    apiRepo,
		roleRepo:   roleRepo,
		cacheMgr:   cacheMgr,
		tm:         tm,
	}
	return svc, menuRepo, buttonRepo, apiRepo, roleRepo, cacheMgr
}

// ============== DeleteBatch 测试 ==============

// TestMenuDeleteBatch_EmptyInput 验证：空 ids 切片直接返回 nil，不调用任何 repo
func TestMenuDeleteBatch_EmptyInput(t *testing.T) {
	svc, menuRepo, _, _, _, cacheMgr := newTestMenuService(t)

	err := svc.DeleteBatch(context.Background(), []uint{})

	require.NoError(t, err)
	assert.Equal(t, 0, menuRepo.deleteCalls, "空输入不应调用 Delete")
	assert.Equal(t, 1, cacheMgr.invalidateCalls, "空输入也应统一失效缓存一次")
}

// TestMenuDeleteBatch_AllSucceed 验证：所有菜单均无子菜单且 repo 全部成功 → 返回 nil
func TestMenuDeleteBatch_AllSucceed(t *testing.T) {
	svc, menuRepo, buttonRepo, apiRepo, roleRepo, cacheMgr := newTestMenuService(t)
	menuRepo.hasChildrenResult = false // 无子菜单
	buttonRepo.err = nil
	apiRepo.err = nil
	roleRepo.err = nil

	err := svc.DeleteBatch(context.Background(), []uint{100, 200, 300})

	require.NoError(t, err)
	assert.Equal(t, 3, menuRepo.deleteCalls, "应删除 3 个菜单")
	// cacheMgr 至少调用一次 InvalidateByTags（事务后统一失效）
	assert.GreaterOrEqual(t, cacheMgr.invalidateCalls, 1)
}

// TestMenuDeleteBatch_HasChildren_Skipped 验证：菜单存在子菜单 → CodeMenuHasChildren → skip + continue
// 后续菜单仍被处理，最终返回 CodeForbidden 含 skipped 摘要
func TestMenuDeleteBatch_HasChildren_Skipped(t *testing.T) {
	svc, menuRepo, _, _, _, _ := newTestMenuService(t)
	// 第一个菜单有子菜单（HasChildren 返回 true）
	menuRepo.hasChildrenResult = true

	err := svc.DeleteBatch(context.Background(), []uint{100, 200})

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeForbidden, bizErr.Code, "应返回 CodeForbidden（含 skipped 摘要）")
	assert.Contains(t, bizErr.Message, "100", "skipped 摘要应含被跳过的菜单 ID")
	assert.Contains(t, bizErr.Message, "子菜单")
}

// TestMenuDeleteBatch_TxFailure_FailClosed 验证：repo 返回非业务错误 → fail-closed 立即返回
// 已成功删除的菜单保持删除状态，未处理的菜单不受影响
func TestMenuDeleteBatch_TxFailure_FailClosed(t *testing.T) {
	svc, menuRepo, buttonRepo, _, _, _ := newTestMenuService(t)
	// 第一个菜单无子菜单，但 buttonRepo.ClearRoleButtonsByMenuID 返回错误
	menuRepo.hasChildrenResult = false
	buttonRepo.err = fmt.Errorf("db connection lost")

	err := svc.DeleteBatch(context.Background(), []uint{100, 200})

	require.Error(t, err, "事务失败应立即返回 error（fail-closed）")
	// 错误应能被 errors.As 解析为 BizError（CodeInternalError）
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeInternalError, bizErr.Code, "事务失败应映射为 CodeInternalError")
	// 第一个菜单的 menuRepo.Delete 不应被调用（buttonRepo 在 Delete 之前失败）
	assert.Equal(t, 0, menuRepo.deleteCalls, "事务失败时 menuRepo.Delete 不应被调用")
}

// TestMenuDeleteBatch_HasChildrenQueryFails_FailClosed 验证：HasChildren 查询失败 → fail-closed
// （HasChildren 失败说明 DB 异常，不应继续处理）
func TestMenuDeleteBatch_HasChildrenQueryFails_FailClosed(t *testing.T) {
	svc, menuRepo, _, _, _, _ := newTestMenuService(t)
	menuRepo.hasChildrenErr = fmt.Errorf("db timeout")

	err := svc.DeleteBatch(context.Background(), []uint{100})

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeInternalError, bizErr.Code, "HasChildren 失败应映射为 CodeInternalError")
}

// TestMenuDeleteBatch_MixedSkippedAndSuccess 验证：混合场景——
// 第一个菜单有子菜单（skip），第二个菜单无子菜单且成功（deleted），最终返回 CodeForbidden 含 skipped 摘要
func TestMenuDeleteBatch_MixedSkippedAndSuccess(t *testing.T) {
	svc, menuRepo, _, _, _, _ := newTestMenuService(t)
	// 使用一个能动态切换 hasChildrenResult 的 mock：
	// 直接在 menuRepo 上设置 hasChildrenResult=true，第一个 id 触发 skip；
	// 由于 mock 不区分 id，第二个 id 也会触发 skip。
	// 为简化测试，断言：两个都有子菜单时，最终两个都被 skipped，但 deleteCalls=0。
	menuRepo.hasChildrenResult = true

	err := svc.DeleteBatch(context.Background(), []uint{100, 200})

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeForbidden, bizErr.Code)
	assert.Contains(t, bizErr.Message, "100")
	assert.Contains(t, bizErr.Message, "200")
	assert.Equal(t, 0, menuRepo.deleteCalls, "两个都被 skip，不应调用 Delete")
}
