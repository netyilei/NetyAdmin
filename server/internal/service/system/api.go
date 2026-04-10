package system

import (
	"context"
	"time"

	systemDto "netyadmin/internal/interface/admin/dto/system"
	systemEntity "netyadmin/internal/domain/entity/system"
	"netyadmin/internal/pkg/cache"
	"netyadmin/internal/pkg/errorx"
	systemRepo "netyadmin/internal/repository/system"
)

type APIService interface {
	List(ctx context.Context, req *systemDto.APIQuery) ([]*systemDto.APIVO, int64, error)
	GetByID(ctx context.Context, id uint) (*systemDto.APIVO, error)
	Create(ctx context.Context, req *systemDto.CreateAPIReq) (uint, error)
	Update(ctx context.Context, req *systemDto.UpdateAPIReq) error
	Delete(ctx context.Context, id uint) error
	GetByMenuID(ctx context.Context, menuID uint) ([]*systemDto.APIVO, error)
	GetAll(ctx context.Context) ([]*systemDto.APIVO, error)
	GetByRoleID(ctx context.Context, roleID uint) ([]*systemEntity.API, error)
}

type apiService struct {
	apiRepo  systemRepo.APIRepository
	cacheMgr cache.LazyCacheManager
}

func NewAPIService(apiRepo systemRepo.APIRepository, cacheMgr cache.LazyCacheManager) APIService {
	return &apiService{
		apiRepo:  apiRepo,
		cacheMgr: cacheMgr,
	}
}

func (s *apiService) List(ctx context.Context, req *systemDto.APIQuery) ([]*systemDto.APIVO, int64, error) {
	query := &systemRepo.APIRepoQuery{
		Name:    req.Name,
		Method:  req.Method,
		Path:    req.Path,
		Current: req.Current,
		Size:    req.Size,
	}

	apis, total, err := s.apiRepo.List(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	items := make([]*systemDto.APIVO, 0, len(apis))
	for _, a := range apis {
		item := &systemDto.APIVO{
			ID:          a.ID,
			MenuID:      a.MenuID,
			Name:        a.Name,
			Method:      a.Method,
			Path:        a.Path,
			Description: a.Description,
			Auth:        a.Auth,
			CreatedAt:   a.CreatedAt.Format(time.DateTime),
			UpdatedAt:   a.UpdatedAt.Format(time.DateTime),
		}
		if a.Menu != nil {
			item.MenuName = a.Menu.Name
		}
		items = append(items, item)
	}

	return items, total, nil
}

func (s *apiService) GetByID(ctx context.Context, id uint) (*systemDto.APIVO, error) {
	api, err := s.apiRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errorx.New(errorx.CodeNotFound, "API不存在")
	}

	item := &systemDto.APIVO{
		ID:          api.ID,
		MenuID:      api.MenuID,
		Name:        api.Name,
		Method:      api.Method,
		Path:        api.Path,
		Description: api.Description,
		Auth:        api.Auth,
		CreatedAt:   api.CreatedAt.Format(time.DateTime),
		UpdatedAt:   api.UpdatedAt.Format(time.DateTime),
	}
	if api.Menu != nil {
		item.MenuName = api.Menu.Name
	}

	return item, nil
}

func (s *apiService) Create(ctx context.Context, req *systemDto.CreateAPIReq) (uint, error) {
	exists, err := s.apiRepo.ExistsByMethodAndPath(ctx, req.Method, req.Path)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, errorx.New(errorx.CodeAlreadyExists, "API路径已存在")
	}

	api := &systemEntity.API{
		Name:        req.Name,
		Method:      req.Method,
		Path:        req.Path,
		Description: req.Desc,
	}

	if err := s.apiRepo.Create(ctx, api); err != nil {
		return 0, err
	}

	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACAPI)

	return api.ID, nil
}

func (s *apiService) Update(ctx context.Context, req *systemDto.UpdateAPIReq) error {
	api, err := s.apiRepo.GetByID(ctx, req.ID)
	if err != nil {
		return errorx.New(errorx.CodeNotFound, "API不存在")
	}

	if req.Method != "" && req.Path != "" && (req.Method != api.Method || req.Path != api.Path) {
		exists, err := s.apiRepo.ExistsByMethodAndPath(ctx, req.Method, req.Path, req.ID)
		if err != nil {
			return err
		}
		if exists {
			return errorx.New(errorx.CodeAlreadyExists, "API路径已存在")
		}
	}

	api.Name = req.Name
	api.Method = req.Method
	api.Path = req.Path
	api.Description = req.Desc

	err = s.apiRepo.Update(ctx, api)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACAPI, cache.TagRBACRole)
	}
	return err
}

func (s *apiService) Delete(ctx context.Context, id uint) error {
	err := s.apiRepo.Delete(ctx, id)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACAPI, cache.TagRBACRole)
	}
	return err
}

func (s *apiService) GetByMenuID(ctx context.Context, menuID uint) ([]*systemDto.APIVO, error) {
	apis, err := s.apiRepo.GetByMenuID(ctx, menuID)
	if err != nil {
		return nil, err
	}

	items := make([]*systemDto.APIVO, 0, len(apis))
	for _, a := range apis {
		items = append(items, &systemDto.APIVO{
			ID:          a.ID,
			MenuID:      a.MenuID,
			Name:        a.Name,
			Method:      a.Method,
			Path:        a.Path,
			Description: a.Description,
			Auth:        a.Auth,
		})
	}

	return items, nil
}

func (s *apiService) GetAll(ctx context.Context) ([]*systemDto.APIVO, error) {
	apis, err := s.apiRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*systemDto.APIVO, 0, len(apis))
	for _, a := range apis {
		items = append(items, &systemDto.APIVO{
			ID:          a.ID,
			MenuID:      a.MenuID,
			Name:        a.Name,
			Method:      a.Method,
			Path:        a.Path,
			Description: a.Description,
			Auth:        a.Auth,
		})
	}

	return items, nil
}

func (s *apiService) GetByRoleID(ctx context.Context, roleID uint) ([]*systemEntity.API, error) {
	return s.apiRepo.GetByRoleID(ctx, roleID)
}
