package open_platform

import (
	"context"
	"log/slog"

	"NetyAdmin/internal/domain/entity/open_platform"
	openDto "NetyAdmin/internal/interface/admin/dto/open_platform"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	openRepo "NetyAdmin/internal/repository/open_platform"
)

type OpenApiService interface {
	CreateApi(ctx context.Context, req *openDto.CreateOpenApiReq) error
	UpdateApi(ctx context.Context, req *openDto.UpdateOpenApiReq) error
	DeleteApi(ctx context.Context, id uint64) error
	GetApiByID(ctx context.Context, id uint64) (*open_platform.OpenApi, error)
	ListApis(ctx context.Context, req *openDto.OpenApiQuery) ([]*open_platform.OpenApi, int64, error)
	ListAllApis(ctx context.Context) ([]*open_platform.OpenApi, error)
	ListGroupedApis(ctx context.Context) (interface{}, error)

	GetScopeApis(ctx context.Context, scopeID uint64) ([]*open_platform.OpenApi, error)
	UpdateScopeApis(ctx context.Context, scopeID uint64, apiIDs []uint64) error
	GetApisByScopeIDs(ctx context.Context, scopeIDs []uint64) ([]*open_platform.OpenApi, error)

	GetAppAllowedApis(ctx context.Context, appID string) ([]string, error)
}

type openApiService struct {
	apiRepo  openRepo.OpenApiRepository
	appRepo  openRepo.AppRepository
	cacheMgr cache.LazyCacheManager
	tm       *database.TransactionManager
}

func NewOpenApiService(apiRepo openRepo.OpenApiRepository, appRepo openRepo.AppRepository, cacheMgr cache.LazyCacheManager, tm *database.TransactionManager) OpenApiService {
	return &openApiService{
		apiRepo:  apiRepo,
		appRepo:  appRepo,
		cacheMgr: cacheMgr,
		tm:       tm,
	}
}

func (s *openApiService) CreateApi(ctx context.Context, req *openDto.CreateOpenApiReq) error {
	api := &open_platform.OpenApi{
		Method:      req.Method,
		Path:        req.Path,
		Name:        req.Name,
		Group:       req.Group,
		Description: req.Description,
		Status:      req.Status,
	}
	if api.Group == "" {
		api.Group = "default"
	}
	if api.Status == 0 {
		api.Status = 1
	}
	if err := s.apiRepo.Create(ctx, api); err != nil {
		return err
	}
	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagOpenApi); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagOpenApi, "err", err)
	}
	return nil
}

func (s *openApiService) UpdateApi(ctx context.Context, req *openDto.UpdateOpenApiReq) error {
	api := &open_platform.OpenApi{
		ID:          req.ID,
		Method:      req.Method,
		Path:        req.Path,
		Name:        req.Name,
		Group:       req.Group,
		Description: req.Description,
		Status:      req.Status,
	}
	if err := s.apiRepo.Update(ctx, api); err != nil {
		return err
	}
	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagOpenApi); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagOpenApi, "err", err)
	}
	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagApp); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagApp, "err", err)
	}
	return nil
}

func (s *openApiService) DeleteApi(ctx context.Context, id uint64) error {
	if err := s.apiRepo.Delete(ctx, id); err != nil {
		return err
	}
	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagOpenApi); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagOpenApi, "err", err)
	}
	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagApp); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagApp, "err", err)
	}
	return nil
}

func (s *openApiService) GetApiByID(ctx context.Context, id uint64) (*open_platform.OpenApi, error) {
	return s.apiRepo.GetByID(ctx, id)
}

func (s *openApiService) ListApis(ctx context.Context, req *openDto.OpenApiQuery) ([]*open_platform.OpenApi, int64, error) {
	// service 层接收 admin DTO，内部构造 repository query（spec B10：service 不应依赖 handler 构造的 repo 类型）
	repoQuery := &openRepo.OpenApiRepoQuery{
		Page:     req.Current,
		PageSize: req.Size,
		Method:   req.Method,
		Path:     req.Path,
		Name:     req.Name,
		Group:    req.Group,
		Status:   req.Status,
	}
	return s.apiRepo.List(ctx, repoQuery)
}

func (s *openApiService) ListAllApis(ctx context.Context) ([]*open_platform.OpenApi, error) {
	var list []*open_platform.OpenApi
	key := cache.KeyOpenApiAll()
	err := s.cacheMgr.Fetch(ctx, key, cache.TagOpenApi, []string{cache.TagOpenApi}, 0, &list, func() (interface{}, error) {
		return s.apiRepo.ListAll(ctx)
	})
	return list, err
}

func (s *openApiService) ListGroupedApis(ctx context.Context) (interface{}, error) {
	var result []map[string]interface{}
	key := cache.KeyOpenApiGrouped()
	err := s.cacheMgr.Fetch(ctx, key, cache.TagOpenApi, []string{cache.TagOpenApi}, 0, &result, func() (interface{}, error) {
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
	if _, err := s.appRepo.GetScopeGroupByID(ctx, scopeID); err != nil {
		return nil, err
	}
	return s.apiRepo.GetScopeApis(ctx, scopeID)
}

func (s *openApiService) UpdateScopeApis(ctx context.Context, scopeID uint64, apiIDs []uint64) error {
	if _, err := s.appRepo.GetScopeGroupByID(ctx, scopeID); err != nil {
		return err
	}
	// TM 单事务原子完成「删除旧关联 + 创建新关联」，任一步失败整体回滚（fail-closed）。
	// repo.UpdateScopeApis 内部已移除自管事务，由 service 层负责 TM 包裹。
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.apiRepo.UpdateScopeApis(txCtx, scopeID, apiIDs); err != nil {
		slog.Error("open api update scope apis: update scope apis failed", "scopeID", scopeID, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "Scope API 关联更新失败")
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("open api update scope apis: commit failed", "scopeID", scopeID, "err", err)
		return errorx.New(errorx.CodeInternalError, "Scope API 关联更新失败")
	}
	// 事务后失效缓存
	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagApp); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagApp, "err", err)
	}
	return nil
}

func (s *openApiService) GetApisByScopeIDs(ctx context.Context, scopeIDs []uint64) ([]*open_platform.OpenApi, error) {
	return s.apiRepo.GetApisByScopeIDs(ctx, scopeIDs)
}

func (s *openApiService) GetAppAllowedApis(ctx context.Context, appID string) ([]string, error) {
	var apiKeys []string
	key := cache.KeyAppApis(appID)
	err := s.cacheMgr.FetchFast(ctx, key, cache.TagApp, []string{cache.TagApp, cache.TagAppKey(appID)}, 0, &apiKeys, func() (interface{}, error) {
		scopes, err := s.appRepo.GetAppScopes(ctx, appID)
		if err != nil {
			return nil, err
		}
		if len(scopes) == 0 {
			return []string{}, nil
		}

		var scopeIDs []uint64
		groups, err := s.appRepo.ListScopeGroups(ctx)
		if err != nil {
			return nil, err
		}
		codeToID := make(map[string]uint64, len(groups))
		for _, g := range groups {
			codeToID[g.Code] = g.ID
		}
		for _, code := range scopes {
			if id, ok := codeToID[code]; ok {
				scopeIDs = append(scopeIDs, id)
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
