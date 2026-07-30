package open_platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"NetyAdmin/internal/domain/entity/open_platform"
	openDto "NetyAdmin/internal/interface/admin/dto/open_platform"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/ratelimit"
	"NetyAdmin/internal/pkg/storage"
	"NetyAdmin/internal/pkg/utils"
	ipacRepoPkg "NetyAdmin/internal/repository/ipac"
	openRepo "NetyAdmin/internal/repository/open_platform"
	ipacSvcPkg "NetyAdmin/internal/service/ipac"

	"gorm.io/gorm"
)

type AppService interface {
	GetAppByKey(ctx context.Context, appKey string) (*open_platform.App, error)
	GetAppSecret(ctx context.Context, app *open_platform.App) (string, error)
	VerifyAppScope(ctx context.Context, appID string, requiredScope string) (bool, error)
	AllowRequest(ctx context.Context, app *open_platform.App) (bool, error)
	// TryConsumeNonce 尝试消费 Nonce 用于防重放保护。
	// 返回 (true, nil) 表示 Nonce 首次出现（占用缓存槽位 ttl 时长）；
	// 返回 (false, nil) 表示 Nonce 已存在（重复请求）；
	// 返回 (false, err) 表示缓存服务异常。
	// 内部调用 cacheSlow.SetNX 实现，封装缓存接口不暴露给 middleware/handler 层（BFF 隔离）。
	TryConsumeNonce(ctx context.Context, appKey, nonce string, ttl time.Duration) (bool, error)
	GetAppStorageDriver(ctx context.Context, app *open_platform.App) (storage.Driver, *storage.Config, error)

	// Admin operations
	CreateApp(ctx context.Context, req *openDto.CreateAppReq) error
	UpdateApp(ctx context.Context, req *openDto.UpdateAppReq) error
	ResetAppSecret(ctx context.Context, id string) (string, error)
	ListApps(ctx context.Context, req *openDto.AppQuery) ([]*open_platform.App, int64, error)
	DeleteApp(ctx context.Context, id string) error
	GetAppScopes(ctx context.Context, appID string) ([]string, error)
	ListAvailableScopes(ctx context.Context) ([]map[string]string, error)
	LinkIPRules(ctx context.Context, appID string, ruleIDs []uint) error

	// Scope Group Admin
	ListScopeGroups(ctx context.Context) ([]*open_platform.AppScopeGroup, error)
	CreateScopeGroup(ctx context.Context, req *openDto.CreateScopeGroupReq) error
	UpdateScopeGroup(ctx context.Context, req *openDto.UpdateScopeGroupReq) error
	DeleteScopeGroup(ctx context.Context, id uint64) error
}

type appService struct {
	repo          openRepo.AppRepository
	cacheFast     cache.ConfigCache   // 配置类：app info / scopes（Fast 系列：L1+L2 chain）
	cacheSlow     cache.SecurityCache // 安全类：Nonce 防重放（非 Fast：SetNX，L2 only）
	aesKey        string
	ipacSvc       ipacSvcPkg.IPACService
	ipacRepo      ipacRepoPkg.IPACRepository
	storageMgr    *storage.Manager
	configWatcher configsync.ConfigWatcher
	rateLimiter   *ratelimit.Limiter
	tm            database.TxManager
}

func NewAppService(repo openRepo.AppRepository, cacheFast cache.ConfigCache, cacheSlow cache.SecurityCache, aesKey string, ipacSvc ipacSvcPkg.IPACService, ipacRepo ipacRepoPkg.IPACRepository, storageMgr *storage.Manager, configWatcher configsync.ConfigWatcher, rateLimiter *ratelimit.Limiter, tm database.TxManager) AppService {
	return &appService{
		repo:          repo,
		cacheFast:     cacheFast,
		cacheSlow:     cacheSlow,
		aesKey:        aesKey,
		ipacSvc:       ipacSvc,
		ipacRepo:      ipacRepo,
		storageMgr:    storageMgr,
		configWatcher: configWatcher,
		rateLimiter:   rateLimiter,
		tm:            tm,
	}
}

func (s *appService) GetAppByKey(ctx context.Context, appKey string) (*open_platform.App, error) {
	key := cache.KeyAppInfo(appKey)
	tags := []string{cache.TagApp, cache.TagAppKey(appKey)}

	if !s.cacheFast.IsCacheEnabled(cache.TagApp) {
		return s.repo.GetByKey(ctx, appKey)
	}

	var app open_platform.App
	if err := s.cacheFast.GetFast(ctx, key, tags, 0, &app); err == nil {
		return &app, nil
	}

	a, err := s.repo.GetByKey(ctx, appKey)
	if err != nil {
		return nil, err
	}

	ttl := time.Duration(0)
	if a.CacheTTL > 0 {
		ttl = time.Duration(a.CacheTTL) * time.Second
	}

	if err := s.cacheFast.SetFast(ctx, key, a, tags, ttl); err != nil {
		slog.Warn("set fast cache failed", "key", key, "err", err)
	}

	return a, nil
}

func (s *appService) GetAppSecret(ctx context.Context, app *open_platform.App) (string, error) {
	if app.AppSecret == "" {
		return "", errors.New("app secret is empty")
	}
	// 解密 AppSecret
	return utils.Decrypt(app.AppSecret, s.aesKey)
}

func (s *appService) VerifyAppScope(ctx context.Context, appID string, requiredScope string) (bool, error) {
	if requiredScope == "" {
		return true, nil
	}

	var scopes []string
	key := cache.KeyAppScopes(appID)
	err := s.cacheFast.FetchFast(ctx, key, cache.TagApp, []string{cache.TagApp, cache.TagAppKey(appID)}, 0, &scopes, func() (interface{}, error) {
		return s.repo.GetAppScopes(ctx, appID)
	})

	if err != nil {
		return false, err
	}

	for _, scope := range scopes {
		if scope == requiredScope {
			return true, nil
		}
	}
	return false, nil
}

func (s *appService) AllowRequest(ctx context.Context, app *open_platform.App) (bool, error) {
	if !app.RateLimitEnabled {
		return true, nil
	}

	rate := s.getDefaultRate()
	capacity := s.getDefaultCapacity()

	if app.QuotaConfig != "" {
		var quota open_platform.AppQuotaConfig
		if err := json.Unmarshal([]byte(app.QuotaConfig), &quota); err == nil {
			if quota.Rate > 0 {
				rate = quota.Rate
			}
			if quota.Capacity > 0 {
				capacity = quota.Capacity
			}
		}
	}

	return s.rateLimiter.Allow(ctx, app.AppKey, rate, capacity)
}

func (s *appService) getDefaultRate() int {
	return utils.GetIntWithDefault(s.configWatcher, "open_platform_config", "default_rate", 100)
}

func (s *appService) getDefaultCapacity() int {
	return utils.GetIntWithDefault(s.configWatcher, "open_platform_config", "default_capacity", 200)
}

func (s *appService) TryConsumeNonce(ctx context.Context, appKey, nonce string, ttl time.Duration) (bool, error) {
	nonceKey := cache.KeyAppNonce(appKey, nonce)
	set, err := s.cacheSlow.SetNX(ctx, nonceKey, "1", ttl)
	if err != nil {
		return false, fmt.Errorf("appService.TryConsumeNonce: SetNX failed: %w", err)
	}
	return set, nil
}

func (s *appService) CreateApp(ctx context.Context, req *openDto.CreateAppReq) error {
	app := &open_platform.App{
		Name:             req.Name,
		Status:           req.Status,
		IPFilterEnabled:  req.IPFilterEnabled,
		RateLimitEnabled: req.RateLimitEnabled,
		Remark:           req.Remark,
		QuotaConfig:      normalizeQuotaConfig(req.QuotaConfig),
		CacheTTL:         req.CacheTTL,
		StorageID:        req.StorageID,
	}
	// 生成 AppKey 和 AppSecret
	app.ID = utils.NewULID()
	app.AppKey = app.ID

	rawSecret := utils.NewSecretToken()
	encryptedSecret, err := utils.Encrypt(rawSecret, s.aesKey)
	if err != nil {
		return fmt.Errorf("utils.Encrypt: %w", err)
	}
	app.AppSecret = encryptedSecret

	// TM 单事务原子完成「写入 app 主数据 + 写入 app scopes 关联」，任一步失败整体回滚（fail-closed）。
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.repo.Create(txCtx, app); err != nil {
		slog.Error("app create: create app failed", "appID", app.ID, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "应用创建失败")
	}
	if len(req.Scopes) > 0 {
		if err := s.repo.UpdateAppScopes(txCtx, app.ID, req.Scopes); err != nil {
			slog.Error("app create: update app scopes failed", "appID", app.ID, "err", err)
			s.tm.Rollback(tx)
			return errorx.New(errorx.CodeInternalError, "应用创建失败")
		}
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("app create: commit failed", "appID", app.ID, "err", err)
		return errorx.New(errorx.CodeInternalError, "应用创建失败")
	}
	// 事务后失效缓存（避免「缓存已清但 DB 回滚」中间态）
	if err := s.cacheFast.InvalidateByTags(ctx, cache.TagApp); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagApp, "err", err)
	}
	return nil
}

func (s *appService) UpdateApp(ctx context.Context, req *openDto.UpdateAppReq) error {
	// 复用 old 实例做 patch 后 Update，避免构造新 entity 导致零值字段意外覆盖数据库已有值（与 dict UpdateType 实现模式一致）。
	// AppKey 为业务唯一标识，创建后不可变更（基座设计原则），Update 不修改 AppKey。
	// AppSecret 同样不在此接口修改，由独立的 ResetAppSecret 方法处理轮换。
	txCtx, tx := s.tm.Begin(ctx)
	old, err := s.repo.GetByID(txCtx, req.ID)
	if err != nil {
		s.tm.Rollback(tx)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, "应用不存在")
		}
		slog.Error("repo.GetByID failed", "appID", req.ID, "err", err)
		return fmt.Errorf("repo.GetByID: %w", err)
	}

	old.Name = req.Name
	old.Status = req.Status
	old.IPFilterEnabled = req.IPFilterEnabled
	old.RateLimitEnabled = req.RateLimitEnabled
	old.Remark = req.Remark
	old.QuotaConfig = normalizeQuotaConfig(req.QuotaConfig)
	old.CacheTTL = req.CacheTTL
	old.StorageID = req.StorageID
	// AppKey 不修改（业务唯一标识，创建后不可变更）
	// AppSecret 不修改（由独立的 ResetAppSecret 方法处理轮换）

	if err := s.repo.Update(txCtx, old); err != nil {
		slog.Error("app update: update app failed", "appID", old.ID, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "应用更新失败")
	}

	if err := s.repo.UpdateAppScopes(txCtx, old.ID, req.Scopes); err != nil {
		slog.Error("app update: update app scopes failed", "appID", old.ID, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "应用更新失败")
	}

	if err := s.tm.Commit(tx); err != nil {
		slog.Error("app update: commit failed", "appID", old.ID, "err", err)
		return errorx.New(errorx.CodeInternalError, "应用更新失败")
	}

	// 事务后失效缓存 + ipac reload。
	// AppKey 未变更（创建后不可变更），使用 TagAppKey 失效应用相关缓存即可。
	tag := cache.TagAppKey(old.AppKey)
	if err := s.cacheFast.InvalidateByTags(ctx, tag); err != nil {
		slog.Error("invalidate cache failed", "tag", tag, "err", err)
	}
	if err := s.ipacSvc.NotifyAndReload(ctx); err != nil {
		slog.Warn("app update: notify and reload failed", "err", err)
	}
	return nil
}

func (s *appService) ResetAppSecret(ctx context.Context, id string) (string, error) {
	// TM 单事务原子完成「查询 app + 更新 secret」，任一步失败整体回滚（fail-closed）。
	// 缓存失效在 tm.Commit 之后执行（避免「缓存已清但 DB 回滚」中间态）。
	txCtx, tx := s.tm.Begin(ctx)
	app, err := s.repo.GetByID(txCtx, id)
	if err != nil {
		s.tm.Rollback(tx)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errorx.New(errorx.CodeNotFound)
		}
		return "", err
	}
	rawSecret := utils.NewSecretToken()
	encryptedSecret, err := utils.Encrypt(rawSecret, s.aesKey)
	if err != nil {
		s.tm.Rollback(tx)
		return "", err
	}
	if err := s.repo.UpdateSecret(txCtx, app.ID, encryptedSecret); err != nil {
		slog.Error("app reset secret: update secret failed", "appID", app.ID, "err", err)
		s.tm.Rollback(tx)
		return "", errorx.New(errorx.CodeInternalError, "密钥重置失败")
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("app reset secret: commit failed", "appID", app.ID, "err", err)
		return "", errorx.New(errorx.CodeInternalError, "密钥重置失败")
	}
	// 事务后失效缓存（AppKey == ID，tag 字符串相同，失效一次即可）
	tag := cache.TagAppKey(app.AppKey)
	if err := s.cacheFast.InvalidateByTags(ctx, tag); err != nil {
		slog.Error("invalidate cache failed", "tag", tag, "err", err)
	}
	return rawSecret, nil
}

func (s *appService) ListApps(ctx context.Context, req *openDto.AppQuery) ([]*open_platform.App, int64, error) {
	// service 层接收 admin DTO，内部构造 repository query（spec B10：service 不应依赖 handler 构造的 repo 类型）
	repoQuery := &openRepo.AppRepoQuery{
		Page:     req.Current,
		PageSize: req.Size,
		Name:     req.Name,
		AppKey:   req.AppKey,
		Status:   req.Status,
	}
	apps, total, err := s.repo.List(ctx, repoQuery)
	if err != nil {
		return nil, 0, err
	}

	if len(apps) > 0 {
		appIDs := make([]string, 0, len(apps))
		for _, app := range apps {
			appIDs = append(appIDs, app.ID)
		}
		scopesMap, _ := s.repo.GetAppScopesByAppIDs(ctx, appIDs)
		for _, app := range apps {
			app.Scopes = scopesMap[app.ID]
		}
	}
	return apps, total, nil
}

func (s *appService) DeleteApp(ctx context.Context, id string) error {
	app, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("repo.GetByID: %w", err)
	}
	// TM 单事务级联删除（fail-closed）：app 主表 + sys_app_scopes + sys_open_platform_logs
	// 任一步失败整体回滚，避免产生孤儿行（原 Delete 仅删主表，关联表残留）。
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.repo.DeleteWithCascade(txCtx, id); err != nil {
		slog.Error("app delete: cascade delete failed", "appID", id, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "应用删除失败")
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("app delete: commit failed", "appID", id, "err", err)
		return errorx.New(errorx.CodeInternalError, "应用删除失败")
	}
	// 事务后失效缓存（避免「缓存已清但 DB 回滚」中间态，RULES.md §二）
	if err := s.cacheFast.InvalidateByTags(ctx, cache.TagAppKey(app.AppKey)); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagAppKey(app.AppKey), "err", err)
	}
	if err := s.cacheFast.InvalidateByTags(ctx, cache.TagAppKey(id)); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagAppKey(id), "err", err)
	}

	// DB 已删，NotifyAndReload 失败仅记录日志，不阻断返回（最终一致性可接受）
	if err := s.ipacSvc.NotifyAndReload(ctx); err != nil {
		slog.Warn("app delete: notify and reload failed", "err", err)
	}
	return nil
}

func (s *appService) GetAppScopes(ctx context.Context, appID string) ([]string, error) {
	return s.repo.GetAppScopes(ctx, appID)
}

func (s *appService) ListAvailableScopes(ctx context.Context) ([]map[string]string, error) {
	// 从数据库动态加载，支持 i18n key，结合缓存模块
	var groups []*open_platform.AppScopeGroup
	key := cache.KeyAppAvailableScopes()
	err := s.cacheFast.FetchFast(ctx, key, cache.TagApp, []string{cache.TagApp, "app_scopes"}, 0, &groups, func() (interface{}, error) {
		// 仅返回启用的分组
		allGroups, err := s.repo.ListScopeGroups(ctx)
		if err != nil {
			return nil, err
		}
		enabledGroups := make([]*open_platform.AppScopeGroup, 0)
		for _, g := range allGroups {
			if g.Status == open_platform.AppStatusEnabled {
				enabledGroups = append(enabledGroups, g)
			}
		}
		return enabledGroups, nil
	})

	if err != nil {
		return nil, err
	}

	res := make([]map[string]string, 0, len(groups))
	for _, g := range groups {
		res = append(res, map[string]string{
			"name": g.Name,
			"code": g.Code,
		})
	}
	return res, nil
}

func (s *appService) ListScopeGroups(ctx context.Context) ([]*open_platform.AppScopeGroup, error) {
	return s.repo.ListScopeGroups(ctx)
}

func (s *appService) CreateScopeGroup(ctx context.Context, req *openDto.CreateScopeGroupReq) error {
	group := &open_platform.AppScopeGroup{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
	}
	if err := s.repo.CreateScopeGroup(ctx, group); err != nil {
		return fmt.Errorf("repo.CreateScopeGroup: %w", err)
	}
	if err := s.cacheFast.DeleteFast(ctx, cache.KeyAppAvailableScopes()); err != nil {
		slog.Warn("delete cache failed", "key", cache.KeyAppAvailableScopes(), "err", err)
	}
	return nil
}

func (s *appService) UpdateScopeGroup(ctx context.Context, req *openDto.UpdateScopeGroupReq) error {
	group := &open_platform.AppScopeGroup{
		ID:          req.ID,
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
	}
	if err := s.repo.UpdateScopeGroup(ctx, group); err != nil {
		return fmt.Errorf("repo.UpdateScopeGroup: %w", err)
	}
	if err := s.cacheFast.DeleteFast(ctx, cache.KeyAppAvailableScopes()); err != nil {
		slog.Warn("delete cache failed", "key", cache.KeyAppAvailableScopes(), "err", err)
	}
	return nil
}

// normalizeQuotaConfig 规范化 quota config，空值返回 "{}"
func normalizeQuotaConfig(s string) string {
	if s == "" {
		return "{}"
	}
	return s
}

func (s *appService) DeleteScopeGroup(ctx context.Context, id uint64) error {
	if err := s.repo.DeleteScopeGroup(ctx, id); err != nil {
		return fmt.Errorf("repo.DeleteScopeGroup: %w", err)
	}
	if err := s.cacheFast.DeleteFast(ctx, cache.KeyAppAvailableScopes()); err != nil {
		slog.Warn("delete cache failed", "key", cache.KeyAppAvailableScopes(), "err", err)
	}
	return nil
}

func (s *appService) LinkIPRules(ctx context.Context, appID string, ruleIDs []uint) error {
	// TM 单事务原子完成「清空旧关联 + 写入新关联」，任一步失败整体回滚（fail-closed）。
	// repo.LinkRulesToApp 内部已移除自管事务，由 service 层负责 TM 包裹以满足 RULES.md §二事务管理红线。
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.ipacRepo.LinkRulesToApp(txCtx, appID, ruleIDs); err != nil {
		slog.Error("app link ip rules: link rules to app failed", "appID", appID, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "应用 IP 规则关联失败")
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("app link ip rules: commit failed", "appID", appID, "err", err)
		return errorx.New(errorx.CodeInternalError, "应用 IP 规则关联失败")
	}
	if err := s.ipacSvc.NotifyAndReload(ctx); err != nil {
		return fmt.Errorf("notify ipac reload after link ip rules: %w", err)
	}
	return nil
}

func (s *appService) GetAppStorageDriver(ctx context.Context, app *open_platform.App) (storage.Driver, *storage.Config, error) {
	if app.StorageID > 0 {
		driver, err := s.storageMgr.GetDriver(app.StorageID)
		if err != nil {
			return nil, nil, err
		}
		config, err := s.storageMgr.GetConfig(app.StorageID)
		if err != nil {
			return nil, nil, err
		}
		return driver, config, nil
	}
	driver, config, err := s.storageMgr.GetDefaultDriver()
	if err != nil {
		return nil, nil, err
	}
	return driver, config, nil
}
