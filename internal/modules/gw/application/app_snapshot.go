// app_snapshot.go：edge 判定数据装载型内存快照（「edge HMAC 热路径
// 应用快照装载化」销项——受控装载面，非通用 L1 缓存：判定数据全量装载进程内，
// 治理面写路径 commit 后 Reload + Bus 广播，各节点原子替换收敛。先例=IPEngine
// / rbac Casbin：安全判定数据不进通用缓存平面，由模块自持快照，
// 不触「安全数据永不 L1」禁令。
//
// 快照内容（一次全量装载、不可变；sync/atomic.Pointer 原子替换）：
//   - gw_apps 全量（含 status/限流/quota/IP 组集 + 装载期解密的 secret）——
//     禁用行同样装载（edge ②③ 步要区分 18101 未找到 / 18102 已停用）；
//   - gw_app_scopes（app→授权 scopes，按 scope 排序对齐原 AppScopes 查询）；
//   - gw_apis（status=1——原 byMethodPath 过滤口径）+ gw_scope_apis×
//     gw_scope_groups（granted=是否存在任一白名单行；required=启用组 code 集）。
//
// 失败语义：查询/扫描/解析失败 → Reload 返回错误且**保留旧快照**（调用方
// 告警）；secret 解密失败不整批拒绝——该 app 记为不可用，验签恒 18103（与
// 原逐请求解密失败语义逐字节一致），避免一行坏数据拖垮全量网关。
package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/gw/domain"
)

// errSnapshotNotLoaded 是快照未装载哨兵（edge 映射既有 18xxx3 fail-closed）。
var errSnapshotNotLoaded = errors.New("gw: app snapshot not loaded")

// apiSnapshotKeySep 是 (method,path) 复合键分隔符（method 不可能含 NUL）。
const apiSnapshotKeySep = "\x00"

// loadedApp 是快照内单个应用（app 值 + 授权 scopes + 解密后 secret）。
type loadedApp struct {
	app       domain.App
	scopes    []string
	secret    []byte
	secretErr string // 非空=装载期解密失败（该 app 验签恒 18103）
}

// loadedAPI 是快照内单个登记 API 的判定数据。
type loadedAPI struct {
	api      domain.Api
	granted  bool     // 是否存在任一白名单行（不论组状态——F8 区分未配置/全禁用）
	required []string // 启用组 code 集（原 JOIN status=1 口径）
}

// appSnapshot 是一次装载完成的不可变判定数据集。
type appSnapshot struct {
	apps map[string]*loadedApp // key=app_key
	apis map[string]*loadedAPI // key=method+NUL+path
}

// AppSnapshot 是 edge 判定快照持有者（Reload 全量替换；读取零 IO）。
type AppSnapshot struct {
	aesKey []byte // App secret 解密钥（Service 构造注入）
	// reloadMu 串行化装载（P2-C：写路径 changed/Bus 订阅/周期对账并发 Reload——
	// 非串行时旧装载可能后写覆盖新装载；装载为低频 5 条 SELECT，串行成本可接受）。
	reloadMu sync.Mutex
	snap     atomic.Pointer[appSnapshot]
}

// NewAppSnapshot 创建（未 Reload 时读取返回 errSnapshotNotLoaded——
// edge fail-closed，与 DB 不可达时既有 18xxx 语义一致）。
func NewAppSnapshot(aesKey []byte) *AppSnapshot { return &AppSnapshot{aesKey: aesKey} }

// Loaded 报告快照是否已装载（诊断/测试面）。
func (s *AppSnapshot) Loaded() bool { return s.snap.Load() != nil }

// Reload 全量装载并原子替换（失败保留旧快照——调用方告警；初始装载失败=
// 快照 nil，edge fail-closed 直至下次变更事件/重试）。
// 装载入口互斥串行（reloadMu）：并发 Reload 的旧装载不得后写覆盖新装载；
// repo=应用行读取面（adapters.Repo 同包 scan 复用——消除与 repo.go 的逐字重复）。
func (s *AppSnapshot) Reload(ctx context.Context, pool *database.Pool, repo domain.Repo) error {
	if pool == nil {
		return errors.New("gw: nil pool")
	}
	if repo == nil {
		return errors.New("gw: nil repo")
	}
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	snap, err := s.load(ctx, pool, repo)
	if err != nil {
		return err
	}
	s.snap.Store(snap)
	return nil
}

// load 执行全量查询并组装快照（任何一步失败整批失败，旧快照不被替换）。
func (s *AppSnapshot) load(ctx context.Context, pool *database.Pool, repo domain.Repo) (*appSnapshot, error) {
	apps, appsByID, err := s.loadApps(ctx, repo)
	if err != nil {
		return nil, err
	}
	if err := loadAppScopes(ctx, pool, appsByID); err != nil {
		return nil, err
	}
	groups, err := loadScopeGroups(ctx, pool)
	if err != nil {
		return nil, err
	}
	granted, required, err := loadScopeGrants(ctx, pool, groups)
	if err != nil {
		return nil, err
	}
	apis, err := loadAPIs(ctx, pool, granted, required)
	if err != nil {
		return nil, err
	}
	return &appSnapshot{apps: apps, apis: apis}, nil
}

// loadApps ① gw_apps 全量（含禁用行）+ 装载期 secret 解密。行读取走
// repo.ListAllForSnapshot（adapters 复用 appCols/scanApp——J-1 消除跨层重复）。
func (s *AppSnapshot) loadApps(ctx context.Context, repo domain.Repo) (
	map[string]*loadedApp, map[string]*loadedApp, error) {

	appsRows, err := repo.ListAllForSnapshot(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("gw: snapshot load apps: %w", err)
	}
	apps := make(map[string]*loadedApp, len(appsRows))
	byID := make(map[string]*loadedApp, len(appsRows))
	for _, a := range appsRows {
		entry := &loadedApp{app: a}
		if secret, derr := primitives.DecryptAESGCM(s.aesKey, a.AppSecretEnc); derr != nil {
			entry.secretErr = derr.Error()
		} else {
			entry.secret = secret
		}
		apps[a.AppKey] = entry
		byID[a.ID] = entry
	}
	return apps, byID, nil
}

// loadAppScopes ② gw_app_scopes（顺序对齐原 AppScopes ORDER BY scope）。
func loadAppScopes(ctx context.Context, pool *database.Pool, appsByID map[string]*loadedApp) error {
	rows, err := pool.Query(ctx, `SELECT app_id, scope FROM gw_app_scopes ORDER BY app_id, scope`)
	if err != nil {
		return fmt.Errorf("gw: snapshot load app scopes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var appID, scope string
		if err := rows.Scan(&appID, &scope); err != nil {
			return fmt.Errorf("gw: snapshot scan app scope: %w", err)
		}
		if e, ok := appsByID[appID]; ok {
			e.scopes = append(e.scopes, scope)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("gw: snapshot iterate app scopes: %w", err)
	}
	return nil
}

// groupMeta 是 required scopes 反查所需的组口径。
type groupMeta struct {
	code    string
	enabled bool
}

// loadScopeGroups ③ scope 组字典（id→code/status）。
func loadScopeGroups(ctx context.Context, pool *database.Pool) (map[string]groupMeta, error) {
	rows, err := pool.Query(ctx, `SELECT id, code, status FROM gw_scope_groups`)
	if err != nil {
		return nil, fmt.Errorf("gw: snapshot load scope groups: %w", err)
	}
	defer rows.Close()
	groups := make(map[string]groupMeta, 16)
	for rows.Next() {
		var id, code string
		var status int16
		if err := rows.Scan(&id, &code, &status); err != nil {
			return nil, fmt.Errorf("gw: snapshot scan scope group: %w", err)
		}
		groups[id] = groupMeta{code: code, enabled: domain.Status(status) == domain.StatusEnabled}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("gw: snapshot iterate scope groups: %w", err)
	}
	return groups, nil
}

// loadScopeGrants ④ scope→API 白名单：granted=原始行数（含禁用组行——
// 区分「未配置白名单」与「白名单存在但组全禁用」）；required=启用组 code 集。
func loadScopeGrants(ctx context.Context, pool *database.Pool, groups map[string]groupMeta) (
	map[string]bool, map[string][]string, error) {

	rows, err := pool.Query(ctx, `SELECT scope_id, api_id FROM gw_scope_apis`)
	if err != nil {
		return nil, nil, fmt.Errorf("gw: snapshot load scope apis: %w", err)
	}
	defer rows.Close()
	granted := make(map[string]bool, 64)
	codeSets := make(map[string]map[string]struct{}, 64)
	for rows.Next() {
		var scopeID, apiID string
		if err := rows.Scan(&scopeID, &apiID); err != nil {
			return nil, nil, fmt.Errorf("gw: snapshot scan scope api: %w", err)
		}
		granted[apiID] = true
		g, ok := groups[scopeID]
		if !ok || !g.enabled {
			continue
		}
		set := codeSets[apiID]
		if set == nil {
			set = make(map[string]struct{}, 2)
			codeSets[apiID] = set
		}
		set[g.code] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("gw: snapshot iterate scope apis: %w", err)
	}
	required := make(map[string][]string, len(codeSets))
	for apiID, set := range codeSets {
		codes := make([]string, 0, len(set))
		for code := range set {
			codes = append(codes, code)
		}
		sort.Strings(codes) // 集合序无关判定——固定序便于断言/排障
		required[apiID] = codes
	}
	return granted, required, nil
}

// loadAPIs ⑤ gw_apis 启用行（原 byMethodPath status=1 口径）。
func loadAPIs(ctx context.Context, pool *database.Pool,
	granted map[string]bool, required map[string][]string) (map[string]*loadedAPI, error) {

	rows, err := pool.Query(ctx, `SELECT id, method, path, name, grp, description, status,
		created_at, updated_at FROM gw_apis WHERE status = 1`)
	if err != nil {
		return nil, fmt.Errorf("gw: snapshot load apis: %w", err)
	}
	defer rows.Close()
	apis := make(map[string]*loadedAPI, 64)
	for rows.Next() {
		var v domain.Api
		var status int16
		if err := rows.Scan(&v.ID, &v.Method, &v.Path, &v.Name, &v.Grp, &v.Description,
			&status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("gw: snapshot scan api: %w", err)
		}
		v.Status = domain.Status(status)
		apis[v.Method+apiSnapshotKeySep+v.Path] = &loadedAPI{
			api:      v,
			granted:  granted[v.ID],
			required: required[v.ID],
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("gw: snapshot iterate apis: %w", err)
	}
	return apis, nil
}

// —— 读取面（零 IO；返回副本——快照不可变，内部切片不外泄）——

// AppByKey 按 appKey 取判定快照（app + 授权 scopes；未找到 → domain.ErrNotFound）。
func (s *AppSnapshot) AppByKey(appKey string) (domain.App, []string, error) {
	snap := s.snap.Load()
	if snap == nil {
		return domain.App{}, nil, errSnapshotNotLoaded
	}
	e, ok := snap.apps[appKey]
	if !ok {
		return domain.App{}, nil, domain.ErrNotFound
	}
	return cloneApp(e.app), append([]string(nil), e.scopes...), nil
}

// App 按 appKey 取应用行（面级访问控制消费——scopes 不需要）。
func (s *AppSnapshot) App(appKey string) (domain.App, error) {
	snap := s.snap.Load()
	if snap == nil {
		return domain.App{}, errSnapshotNotLoaded
	}
	e, ok := snap.apps[appKey]
	if !ok {
		return domain.App{}, domain.ErrNotFound
	}
	return cloneApp(e.app), nil
}

// Secret 按 appKey 取 HMAC 明文密钥（装载期解密失败 → 该 app 恒错误=18103）。
func (s *AppSnapshot) Secret(appKey string) ([]byte, error) {
	snap := s.snap.Load()
	if snap == nil {
		return nil, errSnapshotNotLoaded
	}
	e, ok := snap.apps[appKey]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if e.secret == nil {
		return nil, fmt.Errorf("gw: app %s secret unavailable at snapshot load: %s", appKey, e.secretErr)
	}
	return append([]byte(nil), e.secret...), nil
}

// MatchAPI 按 method+path 匹配登记 API（method 大写归一——原查询 ToUpper 口径；
// 未登记 → domain.ErrNotFound=18401）。
func (s *AppSnapshot) MatchAPI(method, path string) (domain.Api, []string, bool, error) {
	snap := s.snap.Load()
	if snap == nil {
		return domain.Api{}, nil, false, errSnapshotNotLoaded
	}
	e, ok := snap.apis[strings.ToUpper(method)+apiSnapshotKeySep+path]
	if !ok {
		return domain.Api{}, nil, false, domain.ErrNotFound
	}
	return e.api, append([]string(nil), e.required...), e.granted, nil
}

// cloneApp 复制应用行（IPGroupIDs 切片不外泄——防止调用方改写快照内容）。
func cloneApp(a domain.App) domain.App {
	a.IPGroupIDs = append([]string(nil), a.IPGroupIDs...)
	return a
}
