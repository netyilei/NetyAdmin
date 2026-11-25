// service.go：gateway 治理编排——App/Scope/API CRUD + secret AES 面 +
// scope 授权（app 授权 scope 必须是已定义组；scope→API 白名单全量替换）。
// secret 语义：明文只在创建/重置的响应通道一次性出现（不落日志/审计 Detail），
// 落库为 AES-256-GCM 密文（key=Secret 注入——config secrets env-only）。
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/gw/domain"
)

// 领域错误透传（adminapi 映射）。
var (
	ErrNotFound        = domain.ErrNotFound
	ErrAppKeyTaken     = domain.ErrAppKeyTaken
	ErrScopeNotGroup   = domain.ErrScopeNotGroup
	ErrScopeNotFound   = domain.ErrScopeNotFound
	ErrScopeCodeTaken  = domain.ErrScopeCodeTaken
	ErrScopeGroupInUse = domain.ErrScopeGroupInUse
	// ErrScopeInvalid 是 scope 组入参非法哨兵（空 code/name——区别于
	// ErrScopeNotGroup 的「授权引用不存在组」语义）。
	ErrScopeInvalid = errors.New("gw: scope group invalid")
	// ErrScopeDisabled 是授权停用 scope 组的拒绝哨兵（停用=暂停该组授权面）。
	ErrScopeDisabled = errors.New("gw: scope group disabled")
	// ErrBuiltInApp 是内置面应用不可变操作哨兵（停用/删除会使 admin/client
	// 面访问控制载体失效——管理面自锁且产品内无恢复路径）。
	ErrBuiltInApp = errors.New("gw: builtin app immutable")
	// ErrIPGroupsUnwired 是组集校验面未接线时的绑定拒绝哨兵（fail-closed）。
	ErrIPGroupsUnwired = errors.New("gw: ip group validation not wired")
	// ErrIPGroupIDEmpty 是组集含空串 ID 的非法输入哨兵（bind 面不静默归一）。
	ErrIPGroupIDEmpty = errors.New("gw: ip group id empty")
)

// Service 是治理编排入口（AttachDB 后可用）。
type Service struct {
	repo   domain.Repo
	aesKey []byte       // App secret 加密钥（Secret 注入——config secrets env-only）
	ipac   *IPACManager // IP 组集存在性校验（AttachIPAC——nil 时非空组集 fail-closed）

	// edge 判定快照（装载化——受控装载面；AttachSnapshot 后启用）。
	snapshot      *AppSnapshot
	snapPool      *database.Pool
	snapLogger    *slog.Logger
	onSnapChanged func(context.Context)
}

// New 创建（aesKey 必填——secret 加密面 fail-closed）。
func New(aesKey []byte) (*Service, error) {
	if len(aesKey) != 32 {
		return nil, fmt.Errorf("%w: gw service", primitives.ErrAESKeySize)
	}
	return &Service{repo: nil, aesKey: aesKey, snapshot: NewAppSnapshot(aesKey)}, nil
}

// Attach 接线存储（module.go AttachDB）。
func (s *Service) Attach(repo domain.Repo) error {
	if repo == nil {
		return errors.New("gw: nil repo")
	}
	s.repo = repo
	return nil
}

// AttachIPAC 接线 IP 组校验面（module.go AttachDB——组集绑定写路径的存在性
// 校验依赖；nil 时非空组集绑定一律拒绝——fail-closed 防幽灵组引用）。
func (s *Service) AttachIPAC(m *IPACManager) {
	s.ipac = m
}

// AttachSnapshot 接线 edge 判定快照装载面（module.go AttachDB）：pool=装载源，
// logger=Reload 失败告警面（nil→slog.Default），onChanged=变更广播回调（可空——
// 跨节点收敛）。未接线时写路径不触发 Reload（单元测试/嵌入式用法）。
func (s *Service) AttachSnapshot(pool *database.Pool, logger *slog.Logger, onChanged func(context.Context)) {
	if logger == nil {
		logger = slog.Default()
	}
	s.snapPool = pool
	s.snapLogger = logger
	s.onSnapChanged = onChanged
}

// ReloadSnapshot 全量重装载 edge 判定快照（初始装载/Bus 订阅消费面）。
func (s *Service) ReloadSnapshot(ctx context.Context) error {
	if s.snapPool == nil {
		return errors.New("gw: app snapshot pool not attached")
	}
	if s.repo == nil {
		return errors.New("gw: app snapshot repo not attached")
	}
	return s.snapshot.Reload(ctx, s.snapPool, s.repo)
}

// snapshotReloadTimeout 是 changed() 失效链的超时上界（请求 ctx 已断连时
// 仍给 Reload/广播一个有限窗口）。
const snapshotReloadTimeout = 10 * time.Second

// changed 治理写入 commit 后：本节点快照 Reload + onChanged 广播（module 注入）。
// Reload 失败仍广播（对照 IPACManager.changed——健康节点各自可成功装载，
// 跳过广播会让其他节点同样停留旧快照）；旧快照继续服务=降级而非中断。
//
// ctx 以 context.WithoutCancel 派生（P2-B）：管理端写请求断连即取消时，原实现
// 复用请求 ctx 导致 Reload+广播双失败（stale-allow 直至 5min 对账）；派生后
// 保留请求值（request_id 等）但不受其取消影响，超时仍兜底（10s）。同步语义
// 不变：Reload 本地快、广播异步。
func (s *Service) changed(ctx context.Context) {
	if s.snapPool == nil {
		return // 快照面未接线（无旧快照/无广播面——单测与嵌入式用法）
	}
	opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), snapshotReloadTimeout)
	defer cancel()
	if err := s.ReloadSnapshot(opCtx); err != nil {
		s.snapLogger.Error("gw: app snapshot reload failed (broadcast continues)", "err", err)
	}
	if s.onSnapChanged != nil {
		s.onSnapChanged(opCtx)
	}
}

// validateIPGroups 组集存在性校验（——应用只可引用已定义组；空集=合法
// 不限；空串 ID=非法输入直接拒绝——不存在静默归一）。校验面未接线时非空组集
// 拒绝（fail-closed——与 scope 幽灵授权同口径）。
func (s *Service) validateIPGroups(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		if id == "" {
			return ErrIPGroupIDEmpty
		}
	}
	if s.ipac == nil {
		return ErrIPGroupsUnwired
	}
	return s.ipac.GroupsExist(ctx, ids)
}

func (s *Service) ready() error {
	if s.repo == nil {
		return errors.New("gw: storage not attached")
	}
	return nil
}

// —— Apps ——

// CreateApp 创建：app_key 唯一预检（DB 唯一约束并发兜底——事务内冲突转
// ErrAppKeyTaken 409）+ secret 加密落库 + 明文一次性返回。App+初始 scopes
// 单事务写入（安-F7：两步写非原子，scopes 失败遗留无授权 App）；
// scopes/ipGroupIDs 非空时逐个校验（必须是已定义 scope 组/IP 策略组）。
// CreateApp 创建（rateLimitEnabled nil=默认开——：创建面此前硬编码
// true 且不接表单开关，新建应用限流恒开与用户选择相悖）。
func (s *Service) CreateApp(ctx context.Context, name, appKey, secret, remark, storageRef string,
	quota domain.QuotaConfig, scopes, ipGroupIDs []string, rateLimitEnabled *bool) (domain.AppView, error) {

	if err := s.ready(); err != nil {
		return domain.AppView{}, err
	}
	if err := quota.Validate(); err != nil {
		return domain.AppView{}, err
	}
	if _, err := s.repo.GetAppByKey(ctx, appKey); err == nil {
		return domain.AppView{}, ErrAppKeyTaken
	} else if !errors.Is(err, ErrNotFound) {
		return domain.AppView{}, err
	}
	if err := s.validateScopes(ctx, scopes); err != nil {
		return domain.AppView{}, err
	}
	if err := s.validateIPGroups(ctx, ipGroupIDs); err != nil {
		return domain.AppView{}, err
	}
	enc, err := primitives.EncryptAESGCM(s.aesKey, []byte(secret))
	if err != nil {
		return domain.AppView{}, err
	}
	a := domain.App{
		ID:               primitives.NewULID(primitives.SystemClock{}),
		AppKey:           appKey,
		AppSecretEnc:     enc,
		Name:             name,
		Status:           domain.StatusEnabled,
		RateLimitEnabled: rateLimitEnabled == nil || *rateLimitEnabled,
		Quota:            quota,
		Remark:           remark,
		IPGroupIDs:       ipGroupIDs,
		StorageRef:       storageRef,
	}
	if err := s.repo.InsertAppWithScopes(ctx, a, scopes); err != nil {
		return domain.AppView{}, err
	}
	s.changed(ctx) // commit 后快照失效（本节点 Reload + Bus 广播）
	return s.appView(ctx, a)
}

// ListApps 分页。
func (s *Service) ListApps(ctx context.Context, offset, limit int) ([]domain.AppView, int64, error) {
	if err := s.ready(); err != nil {
		return nil, 0, err
	}
	apps, total, err := s.repo.ListApps(ctx, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.AppView, 0, len(apps))
	for _, a := range apps {
		v, err := s.appView(ctx, a)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, nil
}

// GetApp 详情。
func (s *Service) GetApp(ctx context.Context, id string) (domain.AppView, error) {
	if err := s.ready(); err != nil {
		return domain.AppView{}, err
	}
	a, err := s.repo.GetApp(ctx, id)
	if err != nil {
		return domain.AppView{}, err
	}
	return s.appView(ctx, a)
}

// UpdateApp 部分更新基础字段（nil=不改——评审修：此前零值覆盖=
// 漏传 quota 静默关限流；status 不在本面——/status 端点独立承载；
// ipGroupIDs 非 nil=组集全量替换——绑定面唯一入口；scopes 非 nil=授权
// 全量替换与基础字段单事务落库——应用表单单一保存入口）。
func (s *Service) UpdateApp(ctx context.Context, id string,
	name, remark, storageRef *string, quota *domain.QuotaConfig, rateLimitEnabled *bool,
	ipGroupIDs *[]string, scopes *[]string) (domain.AppView, error) {

	if err := s.ready(); err != nil {
		return domain.AppView{}, err
	}
	if quota != nil {
		if err := quota.Validate(); err != nil {
			return domain.AppView{}, err
		}
	}
	a, err := s.repo.GetApp(ctx, id)
	if err != nil {
		return domain.AppView{}, err
	}
	if name != nil {
		a.Name = *name
	}
	if remark != nil {
		a.Remark = *remark
	}
	if storageRef != nil {
		a.StorageRef = *storageRef
	}
	if quota != nil {
		a.Quota = *quota
	}
	if rateLimitEnabled != nil {
		a.RateLimitEnabled = *rateLimitEnabled
	}
	if ipGroupIDs != nil {
		if err := s.validateIPGroups(ctx, *ipGroupIDs); err != nil {
			return domain.AppView{}, err
		}
		a.IPGroupIDs = *ipGroupIDs
	}
	if scopes != nil {
		if err := s.validateScopes(ctx, *scopes); err != nil {
			return domain.AppView{}, err
		}
		if err := s.repo.UpdateAppWithScopes(ctx, a, *scopes); err != nil {
			return domain.AppView{}, err
		}
	} else if err := s.repo.UpdateApp(ctx, a); err != nil {
		return domain.AppView{}, err
	}
	s.changed(ctx)
	return s.appView(ctx, a)
}

// SetAppStatus 启停（独立状态写面——F1 修复后 UpdateApp 不含 status；
// 内置面应用不可停用——停用=admin/client 面 403 自锁且产品内无恢复路径）。
func (s *Service) SetAppStatus(ctx context.Context, id string, status domain.Status) error {
	if err := s.ready(); err != nil {
		return err
	}
	a, err := s.repo.GetApp(ctx, id)
	if err != nil {
		return err
	}
	if isBuiltInAppKey(a.AppKey) {
		return ErrBuiltInApp
	}
	if err := s.repo.UpdateAppStatus(ctx, id, status); err != nil {
		return err
	}
	s.changed(ctx) // 停用/启用即时生效（原面控 30s 缓存窗口消除）
	return nil
}

// ResetAppSecret 重置：新 secret 加密落库 + 明文一次性返回。
func (s *Service) ResetAppSecret(ctx context.Context, id string) (string, error) {
	if err := s.ready(); err != nil {
		return "", err
	}
	if _, err := s.repo.GetApp(ctx, id); err != nil {
		return "", err
	}
	secret, err := generateAppSecret()
	if err != nil {
		return "", err
	}
	enc, err := primitives.EncryptAESGCM(s.aesKey, []byte(secret))
	if err != nil {
		return "", err
	}
	if err := s.repo.UpdateAppSecretEnc(ctx, id, enc); err != nil {
		return "", err
	}
	s.changed(ctx) // 新 secret 立即对 edge 生效
	return secret, nil
}

// DeleteApp 删除（内置面应用不可删——同 SetAppStatus 自锁守卫）。
func (s *Service) DeleteApp(ctx context.Context, id string) error {
	if err := s.ready(); err != nil {
		return err
	}
	a, err := s.repo.GetApp(ctx, id)
	if err != nil {
		return err
	}
	if isBuiltInAppKey(a.AppKey) {
		return ErrBuiltInApp
	}
	if err := s.repo.DeleteApp(ctx, id); err != nil {
		return err
	}
	s.changed(ctx) // 删除后 edge 立即拒绝（HMAC 取密钥失败→18103）
	return nil
}

// isBuiltInAppKey 判定内置面应用 appKey（EnsureGatewayApps 自举——
// AppKeyFrontendApp 同包复用；admin 面已退出应用面无内置应用）。
func isBuiltInAppKey(appKey string) bool {
	return appKey == AppKeyFrontendApp
}

// —— Scope groups 管理面（接口权限页——组字典 CRUD+引用守卫） ——

// CreateScopeGroup 创建 scope 组（code 唯一——DB 约束并发兜底转
// ErrScopeCodeTaken 409）。
func (s *Service) CreateScopeGroup(ctx context.Context, g domain.ScopeGroup) (domain.ScopeGroup, error) {
	if err := s.ready(); err != nil {
		return domain.ScopeGroup{}, err
	}
	if g.Code == "" || g.Name == "" {
		return domain.ScopeGroup{}, ErrScopeInvalid
	}
	g.ID = primitives.NewULID(primitives.SystemClock{})
	if g.Status == 0 {
		g.Status = domain.StatusEnabled
	}
	if err := s.repo.InsertScopeGroup(ctx, g); err != nil {
		return domain.ScopeGroup{}, err
	}
	s.changed(ctx)
	return s.repo.GetScopeGroupByID(ctx, g.ID)
}

// UpdateScopeGroup 更新 scope 组（name/description/status；code 不可变——
// 授权引用按 code）。
func (s *Service) UpdateScopeGroup(ctx context.Context, g domain.ScopeGroup) (domain.ScopeGroup, error) {
	if err := s.ready(); err != nil {
		return domain.ScopeGroup{}, err
	}
	cur, err := s.repo.GetScopeGroupByID(ctx, g.ID)
	if err != nil {
		return domain.ScopeGroup{}, err
	}
	cur.Name = g.Name
	cur.Description = g.Description
	cur.Status = g.Status
	if err := s.repo.UpdateScopeGroup(ctx, cur); err != nil {
		return domain.ScopeGroup{}, err
	}
	s.changed(ctx) // 组启停影响 required scopes 反查
	return s.repo.GetScopeGroupByID(ctx, g.ID)
}

// DeleteScopeGroup 删除 scope 组（引用守卫下沉 repo 条件删除——2：
// 原 service 层 count→DELETE 两步有 check-then-act 窗口；错误面不变：
// ErrScopeNotFound（404）/ ErrScopeGroupInUse（409，授权或白名单悬空守卫））。
func (s *Service) DeleteScopeGroup(ctx context.Context, id string) error {
	if err := s.ready(); err != nil {
		return err
	}
	if err := s.repo.DeleteScopeGroup(ctx, id); err != nil {
		return err
	}
	s.changed(ctx)
	return nil
}

// appView 组装投影（scopes 聚合）。
func (s *Service) appView(ctx context.Context, a domain.App) (domain.AppView, error) {
	scopes, err := s.repo.AppScopes(ctx, a.ID)
	if err != nil {
		return domain.AppView{}, err
	}
	return domain.AppView{
		ID: a.ID, AppKey: a.AppKey, Name: a.Name, Status: a.Status,
		RateLimitEnabled: a.RateLimitEnabled, Quota: a.Quota,
		Remark: a.Remark, StorageRef: a.StorageRef, IPGroupIDs: a.IPGroupIDs,
		Scopes:    scopes,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}, nil
}

// validateScopes 授权 scope 必须是已定义且启用的 scope 组（防止幽灵授权/
// 停用组仍可被新授权——停用=暂停该组授权面；edge 白名单 JOIN status=1 同口径）。
func (s *Service) validateScopes(ctx context.Context, scopes []string) error {
	for _, sc := range scopes {
		g, err := s.repo.GetScopeGroup(ctx, sc)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrScopeNotGroup, sc)
		}
		if g.Status != domain.StatusEnabled {
			return fmt.Errorf("%w: %s", ErrScopeDisabled, sc)
		}
	}
	return nil
}

// AppAuthSnapshot edge 热路径判定快照（中间件消费——装载型快照零 DB
// 往返；语义与原 repo 读取一致：app 全字段 + scopes，未找到 ErrNotFound）。
func (s *Service) AppAuthSnapshot(_ context.Context, appKey string) (domain.App, []string, error) {
	return s.snapshot.AppByKey(appKey)
}

// AppByKey 按 appKey 取应用行（面级访问控制消费——快照读，替代原 30s 单机
// 内存缓存：停用/解绑经写路径广播即时生效，无双源）。
func (s *Service) AppByKey(_ context.Context, appKey string) (domain.App, error) {
	return s.snapshot.App(appKey)
}

// AppSecret edge 验签回调：appKey → secret 明文字节（装载期已解密的快照读；
// HMAC 验签需原文——不落日志/响应）。
func (s *Service) AppSecret(_ context.Context, appKey string) ([]byte, error) {
	return s.snapshot.Secret(appKey)
}

// MatchGatewayAPI method+path 匹配登记 API + 要求 scopes（scope 校验——
// 快照登记表读；granted 语义沿用 F8：区分未配置白名单与组全禁用）。
func (s *Service) MatchGatewayAPI(_ context.Context, method, path string) (domain.Api, []string, bool, error) {
	return s.snapshot.MatchAPI(method, path)
}

// —— Scope groups / APIs ——

// ListScopeGroups 全量。
func (s *Service) ListScopeGroups(ctx context.Context) ([]domain.ScopeGroup, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.repo.ListScopeGroups(ctx)
}

// ListApis 全量（平列表——grp 字段分组归前端）。
func (s *Service) ListApis(ctx context.Context) ([]domain.Api, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.repo.ListApis(ctx)
}

// CreateApi 登记。
func (s *Service) CreateApi(ctx context.Context, v domain.Api) (domain.Api, error) {
	if err := s.ready(); err != nil {
		return domain.Api{}, err
	}
	v.ID = primitives.NewULID(primitives.SystemClock{})
	if v.Status == 0 {
		v.Status = domain.StatusEnabled
	}
	if err := s.repo.InsertApi(ctx, v); err != nil {
		return domain.Api{}, err
	}
	s.changed(ctx)
	return s.repo.GetApi(ctx, v.ID)
}

// UpdateApi 更新。
func (s *Service) UpdateApi(ctx context.Context, v domain.Api) (domain.Api, error) {
	if err := s.ready(); err != nil {
		return domain.Api{}, err
	}
	if err := s.repo.UpdateApi(ctx, v); err != nil {
		return domain.Api{}, err
	}
	s.changed(ctx) // 登记面变更即时生效（method/path/status）
	return s.repo.GetApi(ctx, v.ID)
}

// DeleteApi 删除。
func (s *Service) DeleteApi(ctx context.Context, id string) error {
	if err := s.ready(); err != nil {
		return err
	}
	if err := s.repo.DeleteApi(ctx, id); err != nil {
		return err
	}
	s.changed(ctx)
	return nil
}

// ScopeApis 读 scope 白名单。
func (s *Service) ScopeApis(ctx context.Context, scopeCode string) ([]domain.Api, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	g, err := s.repo.GetScopeGroup(ctx, scopeCode)
	if err != nil {
		return nil, err
	}
	ids, err := s.repo.ScopeApiIDs(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Api, 0, len(ids))
	for _, id := range ids {
		a, err := s.repo.GetApi(ctx, id)
		if err != nil {
			continue // 幽灵行（API 已删）跳过——白名单滞后允许
		}
		out = append(out, a)
	}
	return out, nil
}

// SetScopeApis 全量替换（scopeID=scope code）。
func (s *Service) SetScopeApis(ctx context.Context, scopeCode string, apiIDs []string) error {
	if err := s.ready(); err != nil {
		return err
	}
	g, err := s.repo.GetScopeGroup(ctx, scopeCode)
	if err != nil {
		return err
	}
	if err := s.repo.SetScopeApis(ctx, g.ID, apiIDs); err != nil {
		return err
	}
	s.changed(ctx) // 白名单替换即时生效
	return nil
}

// generateAppSecret 生成 AppSecret（URL 安全 43 字符——HMAC 签名强度 ≥ 256 位）。
func generateAppSecret() (string, error) {
	return primitives.RandomToken(32) // 32 字节 → base64url 43 字符
}
