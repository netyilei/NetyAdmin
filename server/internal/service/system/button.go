package system

import (
	"context"
	"time"

	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	systemRepo "NetyAdmin/internal/repository/system"
)

type ButtonService interface {
	List(ctx context.Context, req *systemDto.ButtonQuery) ([]*systemDto.ButtonVO, int64, error)
	GetByID(ctx context.Context, id uint) (*systemDto.ButtonVO, error)
	Create(ctx context.Context, req *systemDto.CreateButtonReq) (uint, error)
	Update(ctx context.Context, req *systemDto.UpdateButtonReq) error
	Delete(ctx context.Context, id uint) error
	GetByMenuID(ctx context.Context, menuID uint) ([]*systemDto.ButtonVO, error)
	GetByMenuIDs(ctx context.Context, menuIDs []uint) ([]*systemDto.ButtonVO, error)
	GetByRoleID(ctx context.Context, roleID uint) ([]*systemEntity.Button, error)
	GetAll(ctx context.Context) ([]*systemDto.ButtonVO, error)
}

type buttonService struct {
	buttonRepo systemRepo.ButtonRepository
	cacheMgr   cache.LazyCacheManager
}

func NewButtonService(buttonRepo systemRepo.ButtonRepository, cacheMgr cache.LazyCacheManager) ButtonService {
	return &buttonService{
		buttonRepo: buttonRepo,
		cacheMgr:   cacheMgr,
	}
}

func (s *buttonService) List(ctx context.Context, req *systemDto.ButtonQuery) ([]*systemDto.ButtonVO, int64, error) {
	query := &systemRepo.ButtonRepoQuery{
		Label:   req.Label,
		Code:    req.Code,
		MenuID:  req.MenuID,
		Current: req.Current,
		Size:    req.Size,
	}

	buttons, total, err := s.buttonRepo.List(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	items := make([]*systemDto.ButtonVO, 0, len(buttons))
	for _, b := range buttons {
		items = append(items, &systemDto.ButtonVO{
			ID:        b.ID,
			MenuID:    b.MenuID,
			Code:      b.Code,
			Label:     b.Label,
			CreatedAt: b.CreatedAt.Format(time.DateTime),
			UpdatedAt: b.UpdatedAt.Format(time.DateTime),
		})
	}

	return items, total, nil
}

func (s *buttonService) GetByID(ctx context.Context, id uint) (*systemDto.ButtonVO, error) {
	button, err := s.buttonRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errorx.New(errorx.CodeNotFound, "按钮不存在")
	}

	return &systemDto.ButtonVO{
		ID:        button.ID,
		MenuID:    button.MenuID,
		Code:      button.Code,
		Label:     button.Label,
		CreatedAt: button.CreatedAt.Format(time.DateTime),
		UpdatedAt: button.UpdatedAt.Format(time.DateTime),
	}, nil
}

func (s *buttonService) Create(ctx context.Context, req *systemDto.CreateButtonReq) (uint, error) {
	exists, err := s.buttonRepo.ExistsByCode(ctx, req.Code)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, errorx.New(errorx.CodeAlreadyExists, "按钮编码已存在")
	}

	button := &systemEntity.Button{
		MenuID: req.MenuID,
		Code:   req.Code,
		Label:  req.Name, // DTO 中目前是 Name
	}

	if err := s.buttonRepo.Create(ctx, button); err != nil {
		return 0, err
	}

	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu)

	return button.ID, nil
}

func (s *buttonService) Update(ctx context.Context, req *systemDto.UpdateButtonReq) error {
	button, err := s.buttonRepo.GetByID(ctx, req.ID)
	if err != nil {
		return errorx.New(errorx.CodeNotFound, "按钮不存在")
	}

	if req.Code != "" && req.Code != button.Code {
		exists, err := s.buttonRepo.ExistsByCode(ctx, req.Code, req.ID)
		if err != nil {
			return err
		}
		if exists {
			return errorx.New(errorx.CodeAlreadyExists, "按钮编码已存在")
		}
		button.Code = req.Code
	}

	button.MenuID = req.MenuID
	button.Label = req.Name

	err = s.buttonRepo.Update(ctx, button)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu, cache.TagRBACRole)
	}
	return err
}

func (s *buttonService) Delete(ctx context.Context, id uint) error {
	err := s.buttonRepo.Delete(ctx, id)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu, cache.TagRBACRole)
	}
	return err
}

func (s *buttonService) GetByMenuID(ctx context.Context, menuID uint) ([]*systemDto.ButtonVO, error) {
	buttons, err := s.buttonRepo.GetByMenuID(ctx, menuID)
	if err != nil {
		return nil, err
	}

	items := make([]*systemDto.ButtonVO, 0, len(buttons))
	for _, b := range buttons {
		items = append(items, &systemDto.ButtonVO{
			ID:     b.ID,
			MenuID: b.MenuID,
			Code:   b.Code,
			Label:  b.Label,
		})
	}

	return items, nil
}

func (s *buttonService) GetByMenuIDs(ctx context.Context, menuIDs []uint) ([]*systemDto.ButtonVO, error) {
	buttons, err := s.buttonRepo.GetByMenuIDs(ctx, menuIDs)
	if err != nil {
		return nil, err
	}

	items := make([]*systemDto.ButtonVO, 0, len(buttons))
	for _, b := range buttons {
		items = append(items, &systemDto.ButtonVO{
			ID:     b.ID,
			MenuID: b.MenuID,
			Code:   b.Code,
			Label:  b.Label,
		})
	}

	return items, nil
}

func (s *buttonService) GetByRoleID(ctx context.Context, roleID uint) ([]*systemEntity.Button, error) {
	return s.buttonRepo.GetByRoleID(ctx, roleID)
}

func (s *buttonService) GetAll(ctx context.Context) ([]*systemDto.ButtonVO, error) {
	buttons, err := s.buttonRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*systemDto.ButtonVO, 0, len(buttons))
	for _, b := range buttons {
		items = append(items, &systemDto.ButtonVO{
			ID:     b.ID,
			MenuID: b.MenuID,
			Code:   b.Code,
			Label:  b.Label,
		})
	}

	return items, nil
}
