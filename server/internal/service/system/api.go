package system

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	systemRepo "NetyAdmin/internal/repository/system"
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
	tm       *database.TransactionManager
}

func NewAPIService(apiRepo systemRepo.APIRepository, cacheMgr cache.LazyCacheManager, tm *database.TransactionManager) APIService {
	return &apiService{
		apiRepo:  apiRepo,
		cacheMgr: cacheMgr,
		tm:       tm,
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeNotFound, "API不存在")
		}
		slog.Error("apiRepo.GetByID failed", "apiID", id, "err", err)
		return nil, fmt.Errorf("apiRepo.GetByID: %w", err)
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
		MenuID:      req.MenuID,
		Name:        req.Name,
		Method:      req.Method,
		Path:        req.Path,
		Description: req.Desc,
	}

	if err := s.apiRepo.Create(ctx, api); err != nil {
		return 0, err
	}

	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACAPI); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagRBACAPI, "err", err)
	}

	return api.ID, nil
}

func (s *apiService) Update(ctx context.Context, req *systemDto.UpdateAPIReq) error {
	api, err := s.apiRepo.GetByID(ctx, req.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, "API不存在")
		}
		slog.Error("apiRepo.GetByID failed", "apiID", req.ID, "err", err)
		return fmt.Errorf("apiRepo.GetByID: %w", err)
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
	api.MenuID = req.MenuID
	api.Description = req.Desc

	err = s.apiRepo.Update(ctx, api)
	if err == nil {
		if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACAPI, cache.TagRBACRole); cErr != nil {
			slog.Error("invalidate cache failed", "tag", cache.TagRBACAPI, "err", cErr)
		}
	}
	return err
}

func (s *apiService) Delete(ctx context.Context, id uint) error {
	// TM 单事务原子完成「清理 admin_role_apis 关联 + 硬删除 api」。
	// 任一步失败整体回滚，避免「关联已清但 api 未删」或「api 已删但关联未清」的中间态。
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.apiRepo.ClearRoleApis(txCtx, id); err != nil {
		slog.Error("api delete: clear role apis failed", "apiID", id, "err", err)
		s.tm.Rollback(tx)
		return err
	}
	if err := s.apiRepo.Delete(txCtx, id); err != nil {
		slog.Error("api delete: delete api failed", "apiID", id, "err", err)
		s.tm.Rollback(tx)
		return err
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("api delete: commit failed", "apiID", id, "err", err)
		return err
	}
	// 事务后失效缓存：
	//   - TagRBACAPI：KeyAllApis（api 实体被硬删除，全局 api 列表缓存过期）
	//   - TagRBACRole：KeyRoleApis / KeyRoleApiIDs（admin_role_apis 关联被清理，角色权限缓存过期）
	//   - TagRBACMenu：KeyMenuApiTree（api 挂在 menu 下，menu 的 Apis 关联变化，menu-api 树缓存过期）
	if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACAPI, cache.TagRBACRole, cache.TagRBACMenu); cErr != nil {
		slog.Error("invalidate cache failed", "tags", []string{cache.TagRBACAPI, cache.TagRBACRole, cache.TagRBACMenu}, "err", cErr)
	}
	return nil
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
