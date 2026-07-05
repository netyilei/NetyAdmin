package system

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/utils"
)

// 本文件包含 RoleService 的 many2many 关联更新方法（菜单/按钮/API）。
// 从 role.go 拆分而来，降低单文件行数（B6-2 / RULES.md §1）。

// UpdateMenus 更新角色关联的菜单，并设置首页路由。
func (s *roleService) UpdateMenus(ctx context.Context, roleID uint, menuIDs []uint, homeRouteName string) error {
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, "角色不存在")
		}
		slog.Error("roleRepo.GetByID failed", "roleID", roleID, "err", err)
		return fmt.Errorf("roleRepo.GetByID: %w", err)
	}

	if role.Code == systemEntity.SuperRoleCode {
		return errorx.New(errorx.CodeCannotModifySuper, "不允许修改超级管理员角色")
	}

	role.Menus = make([]*systemEntity.Menu, 0, len(menuIDs))
	for _, id := range menuIDs {
		role.Menus = append(role.Menus, &systemEntity.Menu{Model: entity.Model{ID: id}})
	}

	if homeRouteName != "" {
		homeMenu, _ := s.menuRepo.GetByRouteName(ctx, homeRouteName)
		if homeMenu != nil {
			role.HomeMenuID = homeMenu.ID
		}
	} else {
		role.HomeMenuID = 0
	}

	err = s.roleRepo.Update(ctx, role)
	if err == nil {
		if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu); cErr != nil {
			slog.Error("invalidate cache failed", "tag", cache.TagRBACRole, "err", cErr)
		}
	}
	return err
}

// GetRoleButtons 获取角色关联的按钮 ID 列表。
func (s *roleService) GetRoleButtons(ctx context.Context, roleID uint) ([]uint, error) {
	var buttonIDs []uint
	key := cache.KeyRoleButtons(roleID)
	err := s.cacheMgr.Fetch(ctx, key, "rbac_menu", []string{cache.TagRBACRole}, cache.TTL_RBAC, &buttonIDs, func() (interface{}, error) {
		role, err := s.roleRepo.GetByID(ctx, roleID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.New(errorx.CodeNotFound, "角色不存在")
			}
			slog.Error("roleRepo.GetByID failed", "roleID", roleID, "err", err)
			return nil, fmt.Errorf("roleRepo.GetByID: %w", err)
		}

		return utils.SliceMap(role.Buttons, func(b *systemEntity.Button) uint { return b.ID }), nil
	})
	return buttonIDs, err
}

// UpdateButtons 更新角色关联的按钮。
func (s *roleService) UpdateButtons(ctx context.Context, roleID uint, buttonIDs []uint) error {
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, "角色不存在")
		}
		slog.Error("roleRepo.GetByID failed", "roleID", roleID, "err", err)
		return fmt.Errorf("roleRepo.GetByID: %w", err)
	}

	if role.Code == systemEntity.SuperRoleCode {
		return errorx.New(errorx.CodeCannotModifySuper, "不允许修改超级管理员角色")
	}

	role.Buttons = make([]*systemEntity.Button, 0, len(buttonIDs))
	for _, id := range buttonIDs {
		role.Buttons = append(role.Buttons, &systemEntity.Button{Model: entity.Model{ID: id}})
	}

	err = s.roleRepo.Update(ctx, role)
	if err == nil {
		if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole); cErr != nil {
			slog.Error("invalidate cache failed", "tag", cache.TagRBACRole, "err", cErr)
		}
	}
	return err
}

// GetRoleAPIs 获取角色关联的 API ID 列表（用于编辑页）。
func (s *roleService) GetRoleAPIs(ctx context.Context, roleID uint) ([]uint, error) {
	var apiIDs []uint
	key := cache.KeyRoleApiIDs(roleID)
	err := s.cacheMgr.Fetch(ctx, key, "rbac_auth", []string{cache.TagRBACRole}, cache.TTL_RBAC, &apiIDs, func() (interface{}, error) {
		role, err := s.roleRepo.GetByID(ctx, roleID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.New(errorx.CodeNotFound, "角色不存在")
			}
			slog.Error("roleRepo.GetByID failed", "roleID", roleID, "err", err)
			return nil, fmt.Errorf("roleRepo.GetByID: %w", err)
		}

		return utils.SliceMap(role.Apis, func(a *systemEntity.API) uint { return a.ID }), nil
	})
	return apiIDs, err
}

// UpdateAPIs 更新角色关联的 API。
func (s *roleService) UpdateAPIs(ctx context.Context, roleID uint, apiIDs []uint) error {
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, "角色不存在")
		}
		slog.Error("roleRepo.GetByID failed", "roleID", roleID, "err", err)
		return fmt.Errorf("roleRepo.GetByID: %w", err)
	}

	if role.Code == systemEntity.SuperRoleCode {
		return errorx.New(errorx.CodeCannotModifySuper, "不允许修改超级管理员角色")
	}

	role.Apis = make([]*systemEntity.API, 0, len(apiIDs))
	for _, id := range apiIDs {
		role.Apis = append(role.Apis, &systemEntity.API{Model: entity.Model{ID: id}})
	}

	err = s.roleRepo.Update(ctx, role)
	if err == nil {
		// 失效角色相关缓存（包括权限 ID 列表和鉴权所用的 API 列表）
		if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole); cErr != nil {
			slog.Error("invalidate cache failed", "tag", cache.TagRBACRole, "err", cErr)
		}
	}
	return err
}
