package open_platform

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"NetyAdmin/internal/domain/entity/open_platform"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/errorx"
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
	GetCacheMgr() cache.LazyCacheManager
	GetAppStorageDriver(ctx context.Context, app *open_platform.App) (storage.Driver, *storage.Config, error)

	// Admin operations
	CreateApp(ctx context.Context, app *open_platform.App, scopes []string) error
	UpdateApp(ctx context.Context, app *open_platform.App, scopes []string) error
	ResetAppSecret(ctx context.Context, id string) (string, error)
	ListApps(ctx context.Context, query *openRepo.AppRepoQuery) ([]*open_platform.App, int64, error)
	DeleteApp(ctx context.Context, id string) error
	GetAppScopes(ctx context.Context, appID string) ([]string, error)
	ListAvailableScopes(ctx context.Context) ([]map[string]string, error)
	LinkIPRules(ctx context.Context, appID string, ruleIDs []uint) error

	// Scope Group Admin
	ListScopeGroups(ctx context.Context) ([]*open_platform.AppScopeGroup, error)
	CreateScopeGroup(ctx context.Context, group *open_platform.AppScopeGroup) error
	UpdateScopeGroup(ctx context.Context, group *open_platform.AppScopeGroup) error
	DeleteScopeGroup(ctx context.Context, id uint64) error
}

type appService struct {
	repo          openRepo.AppRepository
	cacheMgr      cache.LazyCacheManager
	aesKey        string
	ipacSvc       ipacSvcPkg.IPACService
	ipacRepo      ipacRepoPkg.IPACRepository
	storageMgr    *storage.Manager
	configWatcher configsync.ConfigWatcher
}

func NewAppService(repo openRepo.AppRepository, cacheMgr cache.LazyCacheManager, aesKey string, ipacSvc ipacSvcPkg.IPACService, ipacRepo ipacRepoPkg.IPACRepository, storageMgr *storage.Manager, configWatcher configsync.ConfigWatcher) AppService {
	return &appService{
		repo:          repo,
		cacheMgr:      cacheMgr,
		aesKey:        aesKey,
		ipacSvc:       ipacSvc,
		ipacRepo:      ipacRepo,
		storageMgr:    storageMgr,
		configWatcher: configWatcher,
	}
}

func (s *appService) GetAppByKey(ctx context.Context, appKey string) (*open_platform.App, error) {
	key := cache.KeyAppInfo(appKey)
	tags := []string{cache.TagApp, cache.TagAppKey(appKey)}

	if !s.cacheMgr.IsCacheEnabled(cache.TagApp) {
		return s.repo.GetByKey(ctx, appKey)
	}

	var app open_platform.App
	if err := s.cacheMgr.GetFast(ctx, key, tags, 0, &app); err == nil {
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

	_ = s.cacheMgr.SetFast(ctx, key, a, tags, ttl)

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
	err := s.cacheMgr.FetchFast(ctx, key, cache.TagApp, []string{cache.TagApp, cache.TagAppID(appID)}, 0, &scopes, func() (interface{}, error) {
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

	key := cache.KeyAppRateLimit(app.AppKey)
	return s.cacheMgr.RateLimit(ctx, key, rate, capacity)
}

func (s *appService) getDefaultRate() int {
	val, exists := s.configWatcher.GetConfig("open_platform_config", "default_rate")
	if !exists {
		return 100
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return 100
	}
	return n
}

func (s *appService) getDefaultCapacity() int {
	val, exists := s.configWatcher.GetConfig("open_platform_config", "default_capacity")
	if !exists {
		return 200
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return 200
	}
	return n
}

func (s *appService) GetCacheMgr() cache.LazyCacheManager {
	return s.cacheMgr
}

func (s *appService) CreateApp(ctx context.Context, app *open_platform.App, scopes []string) error {
	// 生成 AppKey 和 AppSecret
	app.ID = utils.NewULID()
	app.AppKey = app.ID

	rawSecret := utils.NewULID() + utils.NewULID() // 简单生成一个较长的随机字符串
	encryptedSecret, err := utils.Encrypt(rawSecret, s.aesKey)
	if err != nil {
		return err
	}
	app.AppSecret = encryptedSecret

	if err := s.repo.Create(ctx, app); err != nil {
		return err
	}

	return s.repo.UpdateAppScopes(ctx, app.ID, scopes)
}

func (s *appService) UpdateApp(ctx context.Context, app *open_platform.App, scopes []string) error {
	oldApp, err := s.repo.GetByID(ctx, app.ID)
	if err != nil {
		return err
	}

	if app.AppSecret != "" {
		encryptedSecret, err := utils.Encrypt(app.AppSecret, s.aesKey)
		if err != nil {
			return err
		}
		app.AppSecret = encryptedSecret
	} else {
		app.AppSecret = oldApp.AppSecret
	}

	if err := s.repo.Update(ctx, app); err != nil {
		return err
	}

	if err := s.repo.UpdateAppScopes(ctx, app.ID, scopes); err != nil {
		return err
	}

	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAppID(app.ID))
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAppKey(oldApp.AppKey))

	_ = s.ipacSvc.ReloadCache(ctx)
	return nil
}

func (s *appService) ResetAppSecret(ctx context.Context, id string) (string, error) {
	app, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errorx.New(errorx.CodeNotFound)
		}
		return "", err
	}
	rawSecret := utils.NewULID() + utils.NewULID()

	encryptedSecret, err := utils.Encrypt(rawSecret, s.aesKey)
	if err != nil {
		return "", err
	}

	if err := s.repo.UpdateSecret(ctx, app.ID, encryptedSecret); err != nil {
		return "", err
	}

	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAppKey(app.AppKey))
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAppID(app.ID))
	return rawSecret, nil
}

func (s *appService) ListApps(ctx context.Context, query *openRepo.AppRepoQuery) ([]*open_platform.App, int64, error) {
	apps, total, err := s.repo.List(ctx, query)
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
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAppKey(app.AppKey))
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagAppID(id))

	// 应用删除后，IPAC缓存中的应用规则也应清除
	_ = s.ipacSvc.ReloadCache(ctx)
	return nil
}

func (s *appService) GetAppScopes(ctx context.Context, appID string) ([]string, error) {
	return s.repo.GetAppScopes(ctx, appID)
}

func (s *appService) ListAvailableScopes(ctx context.Context) ([]map[string]string, error) {
	// 从数据库动态加载，支持 i18n key，结合缓存模块
	var groups []*open_platform.AppScopeGroup
	key := cache.KeyAppAvailableScopes()
	err := s.cacheMgr.Fetch(ctx, key, cache.TagApp, []string{cache.TagApp, "app_scopes"}, 0, &groups, func() (interface{}, error) {
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

func (s *appService) CreateScopeGroup(ctx context.Context, group *open_platform.AppScopeGroup) error {
	if err := s.repo.CreateScopeGroup(ctx, group); err != nil {
		return err
	}
	_ = s.cacheMgr.Delete(ctx, cache.KeyAppAvailableScopes())
	return nil
}

func (s *appService) UpdateScopeGroup(ctx context.Context, group *open_platform.AppScopeGroup) error {
	if err := s.repo.UpdateScopeGroup(ctx, group); err != nil {
		return err
	}
	_ = s.cacheMgr.Delete(ctx, cache.KeyAppAvailableScopes())
	return nil
}

func (s *appService) DeleteScopeGroup(ctx context.Context, id uint64) error {
	if err := s.repo.DeleteScopeGroup(ctx, id); err != nil {
		return err
	}
	_ = s.cacheMgr.Delete(ctx, cache.KeyAppAvailableScopes())
	return nil
}

func (s *appService) LinkIPRules(ctx context.Context, appID string, ruleIDs []uint) error {
	if err := s.ipacRepo.LinkRulesToApp(ctx, appID, ruleIDs); err != nil {
		return err
	}
	_ = s.ipacSvc.ReloadCache(ctx)
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
