package open_platform

import (
	"context"
	"time"

	"NetyAdmin/internal/domain/entity/open_platform"
	"NetyAdmin/internal/pkg/cache"
	openRepo "NetyAdmin/internal/repository/open_platform"
)

type OpenApiService interface {
	CreateApi(ctx context.Context, api *open_platform.OpenApi) error
	UpdateApi(ctx context.Context, api *open_platform.OpenApi) error
	DeleteApi(ctx context.Context, id uint64) error
	GetApiByID(ctx context.Context, id uint64) (*open_platform.OpenApi, error)
	ListApis(ctx context.Context, query *openRepo.OpenApiRepoQuery) ([]*open_platform.OpenApi, int64, error)
	ListAllApis(ctx context.Context) ([]*open_platform.OpenApi, error)
	ListGroupedApis(ctx context.Context) (interface{}, error)

	GetScopeApis(ctx context.Context, scopeID uint64) ([]*open_platform.OpenApi, error)
	UpdateScopeApis(ctx context.Context, scopeID uint64, apiIDs []uint64) error
	GetApisByScopeIDs(ctx context.Context, scopeIDs []uint64) ([]*open_platform.OpenApi, error)

	GetAppAllowedApis(ctx context.Context, appID string) ([]string, error)
}

type openApiService struct {
	apiRepo        openRepo.OpenApiRepository
	appRepo        openRepo.AppRepository
	scopeGroupRepo openRepo.AppRepository
	cacheMgr       cache.LazyCacheManager
}

func NewOpenApiService(apiRepo openRepo.OpenApiRepository, appRepo openRepo.AppRepository, scopeGroupRepo openRepo.AppRepository, cacheMgr cache.LazyCacheManager) OpenApiService {
	return &openApiService{
		apiRepo:        apiRepo,
		appRepo:        appRepo,
		scopeGroupRepo: scopeGroupRepo,
		cacheMgr:       cacheMgr,
	}
}

func (s *openApiService) CreateApi(ctx context.Context, api *open_platform.OpenApi) error {
	if api.Group == "" {
		api.Group = "default"
	}
	if api.Status == 0 {
		api.Status = 1
	}
	if err := s.apiRepo.Create(ctx, api); err != nil {
		return err
	}
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagOpenApi)
	return nil
}

func (s *openApiService) UpdateApi(ctx context.Context, api *open_platform.OpenApi) error {
	if err := s.apiRepo.Update(ctx, api); err != nil {
		return err
	}
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagOpenApi)
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagApp)
	return nil
}

func (s *openApiService) DeleteApi(ctx context.Context, id uint64) error {
	if err := s.apiRepo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagOpenApi)
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagApp)
	return nil
}

func (s *openApiService) GetApiByID(ctx context.Context, id uint64) (*open_platform.OpenApi, error) {
	return s.apiRepo.GetByID(ctx, id)
}

func (s *openApiService) ListApis(ctx context.Context, query *openRepo.OpenApiRepoQuery) ([]*open_platform.OpenApi, int64, error) {
	return s.apiRepo.List(ctx, query)
}

func (s *openApiService) ListAllApis(ctx context.Context) ([]*open_platform.OpenApi, error) {
	var list []*open_platform.OpenApi
	key := cache.KeyOpenApiAll()
	err := s.cacheMgr.Fetch(ctx, key, cache.TagOpenApi, []string{cache.TagOpenApi}, 3600*time.Second, &list, func() (interface{}, error) {
		return s.apiRepo.ListAll(ctx)
	})
	return list, err
}

func (s *openApiService) ListGroupedApis(ctx context.Context) (interface{}, error) {
	var result []map[string]interface{}
	key := cache.KeyOpenApiGrouped()
	err := s.cacheMgr.Fetch(ctx, key, cache.TagOpenApi, []string{cache.TagOpenApi}, 3600*time.Second, &result, func() (interface{}, error) {
		apis, err := s.apiRepo.ListAll(ctx)
		if err != nil {
			return nil, err
		}

		groups := make(map[string][]map[string]interface{})
		for _, api := range apis {
			item := map[string]interface{}{
				"id":     api.ID,
				"name":   api.Name,
				"method": api.Method,
				"path":   api.Path,
			}
			groups[api.Group] = append(groups[api.Group], item)
		}

		var grouped []map[string]interface{}
		for group, list := range groups {
			grouped = append(grouped, map[string]interface{}{
				"group": group,
				"apis":  list,
			})
		}
		return grouped, nil
	})
	return result, err
}

func (s *openApiService) GetScopeApis(ctx context.Context, scopeID uint64) ([]*open_platform.OpenApi, error) {
	if _, err := s.scopeGroupRepo.GetScopeGroupByID(ctx, scopeID); err != nil {
		return nil, err
	}
	return s.apiRepo.GetScopeApis(ctx, scopeID)
}

func (s *openApiService) UpdateScopeApis(ctx context.Context, scopeID uint64, apiIDs []uint64) error {
	if _, err := s.scopeGroupRepo.GetScopeGroupByID(ctx, scopeID); err != nil {
		return err
	}
	if err := s.apiRepo.UpdateScopeApis(ctx, scopeID, apiIDs); err != nil {
		return err
	}
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagApp)
	return nil
}

func (s *openApiService) GetApisByScopeIDs(ctx context.Context, scopeIDs []uint64) ([]*open_platform.OpenApi, error) {
	return s.apiRepo.GetApisByScopeIDs(ctx, scopeIDs)
}

func (s *openApiService) GetAppAllowedApis(ctx context.Context, appID string) ([]string, error) {
	var apiKeys []string
	key := cache.KeyAppApis(appID)
	err := s.cacheMgr.Fetch(ctx, key, cache.TagApp, []string{cache.TagApp, cache.TagAppID(appID)}, 3600*time.Second, &apiKeys, func() (interface{}, error) {
		scopes, err := s.appRepo.GetAppScopes(ctx, appID)
		if err != nil {
			return nil, err
		}
		if len(scopes) == 0 {
			return []string{}, nil
		}

		var scopeIDs []uint64
		for _, code := range scopes {
			groups, err := s.appRepo.ListScopeGroups(ctx)
			if err != nil {
				return nil, err
			}
			for _, g := range groups {
				if g.Code == code {
					scopeIDs = append(scopeIDs, g.ID)
				}
			}
		}

		apis, err := s.apiRepo.GetApisByScopeIDs(ctx, scopeIDs)
		if err != nil {
			return nil, err
		}

		result := make([]string, 0, len(apis))
		for _, a := range apis {
			result = append(result, a.Method+":"+a.Path)
		}
		return result, nil
	})
	return apiKeys, err
}
