package system

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/database"
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
	tm         *database.TransactionManager
}

func NewButtonService(buttonRepo systemRepo.ButtonRepository, cacheMgr cache.LazyCacheManager, tm *database.TransactionManager) ButtonService {
	return &buttonService{
		buttonRepo: buttonRepo,
		cacheMgr:   cacheMgr,
		tm:         tm,
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeNotFound, "按钮不存在")
		}
		slog.Error("buttonRepo.GetByID failed", "buttonID", id, "err", err)
		return nil, fmt.Errorf("buttonRepo.GetByID: %w", err)
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

	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagRBACMenu, "err", err)
	}

	return button.ID, nil
}

func (s *buttonService) Update(ctx context.Context, req *systemDto.UpdateButtonReq) error {
	button, err := s.buttonRepo.GetByID(ctx, req.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, "按钮不存在")
		}
		slog.Error("buttonRepo.GetByID failed", "buttonID", req.ID, "err", err)
		return fmt.Errorf("buttonRepo.GetByID: %w", err)
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
		if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu, cache.TagRBACRole); cErr != nil {
			slog.Error("invalidate cache failed", "tag", cache.TagRBACMenu, "err", cErr)
		}
	}
	return err
}

func (s *buttonService) Delete(ctx context.Context, id uint) error {
	// TM 单事务原子完成「清理 admin_role_buttons 关联 + 硬删除 button」。
	// 任一步失败整体回滚，避免「关联已清但 button 未删」或「button 已删但关联未清」的中间态。
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.buttonRepo.ClearRoleButtons(txCtx, id); err != nil {
		slog.Error("button delete: clear role buttons failed", "buttonID", id, "err", err)
		s.tm.Rollback(tx)
		return err
	}
	if err := s.buttonRepo.Delete(txCtx, id); err != nil {
		slog.Error("button delete: delete button failed", "buttonID", id, "err", err)
		s.tm.Rollback(tx)
		return err
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("button delete: commit failed", "buttonID", id, "err", err)
		return err
	}
	if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu, cache.TagRBACRole); cErr != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagRBACMenu, "err", cErr)
	}
	return nil
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
