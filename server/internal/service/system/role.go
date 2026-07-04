package system

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	systemVO "NetyAdmin/internal/domain/vo/system"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/utils"
	systemRepo "NetyAdmin/internal/repository/system"
)

type RoleService interface {
	List(ctx context.Context, req *systemDto.RoleQuery) ([]*systemVO.RoleItemVO, int64, error)
	GetByID(ctx context.Context, id uint) (*systemVO.RoleVO, error)
	Create(ctx context.Context, req *systemDto.CreateRoleReq, operatorID uint) (uint, error)
	Update(ctx context.Context, req *systemDto.UpdateRoleReq, operatorID uint) error
	Delete(ctx context.Context, id uint) error
	DeleteBatch(ctx context.Context, ids []uint) error
	GetAll(ctx context.Context) ([]*systemVO.RoleSimpleVO, error)
	GetRoleMenusWithHome(ctx context.Context, roleID uint) (map[string]interface{}, error)
	UpdateMenus(ctx context.Context, roleID uint, menuIDs []uint, homeRouteName string) error
	GetRoleButtons(ctx context.Context, roleID uint) ([]uint, error)
	UpdateButtons(ctx context.Context, roleID uint, buttonIDs []uint) error
	GetRoleAPIs(ctx context.Context, roleID uint) ([]uint, error)
	UpdateAPIs(ctx context.Context, roleID uint, apiIDs []uint) error
	VerifyApiAuth(ctx context.Context, method, path string, roleCodes []string) (hasPermission bool, apiFound bool, err error)
}

type roleService struct {
	roleRepo   systemRepo.RoleRepository
	menuRepo   systemRepo.MenuRepository
	apiRepo    systemRepo.APIRepository
	buttonRepo systemRepo.ButtonRepository
	cacheMgr   cache.LazyCacheManager
	tm         *database.TransactionManager
}

func NewRoleService(
	roleRepo systemRepo.RoleRepository,
	menuRepo systemRepo.MenuRepository,
	apiRepo systemRepo.APIRepository,
	buttonRepo systemRepo.ButtonRepository,
	cacheMgr cache.LazyCacheManager,
	tm *database.TransactionManager,
) RoleService {
	return &roleService{
		roleRepo:   roleRepo,
		menuRepo:   menuRepo,
		apiRepo:    apiRepo,
		buttonRepo: buttonRepo,
		cacheMgr:   cacheMgr,
		tm:         tm,
	}
}

func (s *roleService) List(ctx context.Context, req *systemDto.RoleQuery) ([]*systemVO.RoleItemVO, int64, error) {
	query := &systemRepo.RoleRepoQuery{
		Name:    req.Name,
		Code:    req.Code,
		Status:  &req.Status,
		Current: req.Current,
		Size:    req.Size,
	}

	roles, total, err := s.roleRepo.List(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	items := make([]*systemVO.RoleItemVO, 0, len(roles))
	for _, r := range roles {
		items = append(items, &systemVO.RoleItemVO{
			ID:        r.ID,
			Name:      r.Name,
			Code:      r.Code,
			Desc:      r.Description,
			Status:    r.Status,
			Creator:   r.CreatorName(),
			CreatedAt: r.CreatedAt.Format(time.DateTime),
			Updater:   r.UpdaterName(),
			UpdatedAt: r.UpdatedAt.Format(time.DateTime),
		})
	}

	return items, total, nil
}

func (s *roleService) GetByID(ctx context.Context, id uint) (*systemVO.RoleVO, error) {
	role, err := s.roleRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeNotFound, "角色不存在")
		}
		slog.Error("roleRepo.GetByID failed", "roleID", id, "err", err)
		return nil, fmt.Errorf("roleRepo.GetByID: %w", err)
	}

	menuIDs := make([]uint, 0, len(role.Menus))
	for _, m := range role.Menus {
		menuIDs = append(menuIDs, m.ID)
	}

	buttonIDs := make([]uint, 0, len(role.Buttons))
	for _, b := range role.Buttons {
		buttonIDs = append(buttonIDs, b.ID)
	}

	apiIDs := make([]uint, 0, len(role.Apis))
	for _, a := range role.Apis {
		apiIDs = append(apiIDs, a.ID)
	}

	return &systemVO.RoleVO{
		ID:        role.ID,
		Name:      role.Name,
		Code:      role.Code,
		Desc:      role.Description,
		Status:    role.Status,
		CreatedAt: role.CreatedAt.Format(time.DateTime),
		Menus:     menuIDs,
		Buttons:   buttonIDs,
		Apis:      apiIDs,
	}, nil
}

func (s *roleService) Create(ctx context.Context, req *systemDto.CreateRoleReq, operatorID uint) (uint, error) {
	// 禁止创建超级管理员角色，防止越权
	if req.Code == systemEntity.SuperRoleCode {
		return 0, errorx.New(errorx.CodeCannotModifySuper, "不允许创建超级管理员角色")
	}

	exists, err := s.roleRepo.ExistsByCode(ctx, req.Code)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, errorx.New(errorx.CodeAlreadyExists, "角色编码已存在")
	}

	role := &systemEntity.Role{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Desc,
		Status:      req.Status,
	}
	role.CreatedBy = operatorID

	if len(req.Menus) > 0 {
		role.Menus = make([]*systemEntity.Menu, 0, len(req.Menus))
		for _, id := range req.Menus {
			role.Menus = append(role.Menus, &systemEntity.Menu{Model: entity.Model{ID: id}})
		}
	}

	if err := s.roleRepo.Create(ctx, role); err != nil {
		return 0, err
	}

	return role.ID, nil
}

func (s *roleService) Update(ctx context.Context, req *systemDto.UpdateRoleReq, operatorID uint) error {
	// 复用 old 实例做 patch 后 Save，避免构造新 entity 导致 CreatedAt 等零值字段覆盖数据库已有值（Save 是全字段更新）。
	// Code 为业务唯一标识，创建后不可变更，Update 不修改 Code（基座设计原则）。
	role, err := s.roleRepo.GetByID(ctx, req.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, "角色不存在")
		}
		slog.Error("roleRepo.GetByID failed", "roleID", req.ID, "err", err)
		return fmt.Errorf("roleRepo.GetByID: %w", err)
	}

	if role.Code == systemEntity.SuperRoleCode {
		return errorx.New(errorx.CodeCannotModifySuper, "不允许修改超级管理员角色")
	}

	role.Name = req.Name
	role.Description = req.Desc
	role.Status = req.Status
	role.UpdatedBy = operatorID

	if err := s.roleRepo.Update(ctx, role); err != nil {
		return err
	}
	// Code 未变更，只需失效当前 RBAC 缓存（角色 + 菜单）
	if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu); cErr != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagRBACRole, "err", cErr)
	}
	return nil
}

func (s *roleService) Delete(ctx context.Context, id uint) error {
	// 事务前预校验：取角色 + 超管保护（用原始 ctx，不进事务）
	role, err := s.roleRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, "角色不存在")
		}
		slog.Error("roleRepo.GetByID failed", "roleID", id, "err", err)
		return fmt.Errorf("roleRepo.GetByID: %w", err)
	}

	if role.Code == systemEntity.SuperRoleCode {
		return errorx.New(errorx.CodeCannotDeleteSuper, "不允许删除超级管理员角色")
	}

	// 用 WithTransaction 闭包 API 包裹「清理 admin_user_roles + 清理权限关联表 + 硬删除 role」三步原子写。
	// 任一步失败自动 Rollback；panic 自动 Rollback 后重抛让 recovery 中间件捕获 + Sentry 上报。
	// 事务提交后失效 RBAC 缓存（角色 + 菜单）。
	err = s.tm.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.roleRepo.ClearUserRoles(txCtx, id); err != nil {
			slog.Error("role delete: clear user roles failed", "roleID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("角色 %d 删除失败", id))
		}
		if err := s.roleRepo.ClearPermissions(txCtx, id); err != nil {
			slog.Error("role delete: clear permissions failed", "roleID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("角色 %d 删除失败", id))
		}
		if err := s.roleRepo.Delete(txCtx, id); err != nil {
			slog.Error("role delete: delete role failed", "roleID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("角色 %d 删除失败", id))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu); cErr != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagRBACRole, "err", cErr)
	}
	return nil
}

func (s *roleService) DeleteBatch(ctx context.Context, ids []uint) error {
	// 逐条事务 fail-closed：任一 id 事务失败立即返回错误（已提交的 id 保持删除状态）。
	// 业务规则拒绝（不存在/超管保护）走 continue 跳过并记录，不阻断整个批量。
	// 设计权衡同 admin DeleteBatch：TM 单事务原子保证，事务失败立即返回；业务规则拒绝仍 continue。
	var skipped []string
	for _, id := range ids {
		role, err := s.roleRepo.GetByID(ctx, id)
		if err != nil {
			// 业务规则：不存在的 ID 跳过（fail-closed 语义仅针对事务失败）
			if errors.Is(err, gorm.ErrRecordNotFound) {
				skipped = append(skipped, fmt.Sprintf("角色 %d：不存在", id))
				continue
			}
			// DB 错误（非 record not found）：fail-closed 立即返回，已删除的 id 保持，未处理的不受影响
			slog.Error("role delete batch: GetByID failed", "roleID", id, "err", err)
			if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu); cErr != nil {
				slog.Error("invalidate cache failed after GetByID failure", "tag", cache.TagRBACRole, "err", cErr)
			}
			return fmt.Errorf("roleRepo.GetByID (id=%d): %w", id, err)
		}
		if role.Code == systemEntity.SuperRoleCode {
			skipped = append(skipped, fmt.Sprintf("角色 %d：不允许删除超级管理员角色", id))
			continue
		}
		// TM 单事务原子完成「清理 admin_user_roles + 清理权限关联表 + 硬删除 role」
		txCtx, tx := s.tm.Begin(ctx)
		if err := s.roleRepo.ClearUserRoles(txCtx, id); err != nil {
			slog.Error("role delete batch: clear user roles failed", "roleID", id, "err", err)
			s.tm.Rollback(tx)
			// 事务失败立即返回（fail-closed）：已提交的 id 保持删除状态，未处理的 id 不受影响
			// 但已成功删除的 id 会导致 RBAC 缓存过期，必须失效
			if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu); cErr != nil {
				slog.Error("invalidate cache failed after tx failure", "tag", cache.TagRBACRole, "err", cErr)
			}
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("角色 %d 删除失败", id))
		}
		if err := s.roleRepo.ClearPermissions(txCtx, id); err != nil {
			slog.Error("role delete batch: clear permissions failed", "roleID", id, "err", err)
			s.tm.Rollback(tx)
			if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu); cErr != nil {
				slog.Error("invalidate cache failed after tx failure", "tag", cache.TagRBACRole, "err", cErr)
			}
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("角色 %d 删除失败", id))
		}
		if err := s.roleRepo.Delete(txCtx, id); err != nil {
			slog.Error("role delete batch: delete role failed", "roleID", id, "err", err)
			s.tm.Rollback(tx)
			if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu); cErr != nil {
				slog.Error("invalidate cache failed after tx failure", "tag", cache.TagRBACRole, "err", cErr)
			}
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("角色 %d 删除失败", id))
		}
		if err := s.tm.Commit(tx); err != nil {
			slog.Error("role delete batch: commit failed", "roleID", id, "err", err)
			if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu); cErr != nil {
				slog.Error("invalidate cache failed after tx failure", "tag", cache.TagRBACRole, "err", cErr)
			}
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("角色 %d 删除失败", id))
		}
	}
	// 全部处理完成后，失效 RBAC 缓存（角色 + 菜单）
	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagRBACRole, "err", err)
	}
	if len(skipped) > 0 {
		return errorx.New(errorx.CodeForbidden, fmt.Sprintf("部分角色被跳过：%s", strings.Join(skipped, "; ")))
	}
	return nil
}

func (s *roleService) GetAll(ctx context.Context) ([]*systemVO.RoleSimpleVO, error) {
	roles, err := s.roleRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*systemVO.RoleSimpleVO, 0, len(roles))
	for _, r := range roles {
		items = append(items, &systemVO.RoleSimpleVO{
			ID:   r.ID,
			Name: r.Name,
			Code: r.Code,
		})
	}

	return items, nil
}

func (s *roleService) GetRoleMenusWithHome(ctx context.Context, roleID uint) (map[string]interface{}, error) {
	var result map[string]interface{}
	key := cache.KeyRoleMenus(roleID)
	err := s.cacheMgr.Fetch(ctx, key, "rbac_menu", []string{cache.TagRBACRole}, cache.TTL_RBAC, &result, func() (interface{}, error) {
		role, err := s.roleRepo.GetByID(ctx, roleID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.New(errorx.CodeNotFound, "角色不存在")
			}
			slog.Error("roleRepo.GetByID failed", "roleID", roleID, "err", err)
			return nil, fmt.Errorf("roleRepo.GetByID: %w", err)
		}

		menuIds := utils.SliceMap(role.Menus, func(m *systemEntity.Menu) uint { return m.ID })

		homeRouteName := ""
		if role.HomeMenu != nil {
			homeRouteName = role.HomeMenu.RouteName
		}

		return map[string]interface{}{
			"menuIds":       menuIds,
			"homeRouteName": homeRouteName,
		}, nil
	})
	return result, err
}

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

func (s *roleService) GetRoleAPIs(ctx context.Context, roleID uint) ([]uint, error) {
	var apiIDs []uint
	key := cache.KeyRoleApiIDs(roleID) // 这里专门缓存 ID列表用于编辑
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

// 废弃旧的简陋 matchPath
// func matchPath(pattern, path string) bool { ... }

// 复用 Gin 的路由匹配树进行高精度匹配
func matchPath(pattern, path string) bool {
	if pattern == path {
		return true
	}

	// 如果不包含参数占位符，直接返回不匹配
	if !strings.Contains(pattern, ":") && !strings.Contains(pattern, "*") {
		return false
	}

	// 动态路由匹配：如 /admin/v1/user/:id 匹配 /admin/v1/user/123
	// 我们利用 Gin 内部的逻辑，或者用一个简单的正则/切片匹配，但要处理不同长度问题。
	// 改进后的安全切片匹配算法：
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	// 如果段数不同，且没有 * 通配符，则绝对不匹配
	if len(patternParts) != len(pathParts) && !strings.Contains(pattern, "*") {
		return false
	}

	for i := 0; i < len(patternParts); i++ {
		pPattern := patternParts[i]

		// 如果遇到 * 通配符 (如 /*action)，直接匹配后续所有路径
		if strings.HasPrefix(pPattern, "*") {
			return true
		}

		// 防止越界
		if i >= len(pathParts) {
			return false
		}

		pPath := pathParts[i]

		// 遇到 : 占位符，跳过当前段的比对
		if strings.HasPrefix(pPattern, ":") {
			continue
		}

		if pPattern != pPath {
			return false
		}
	}

	// 确保 pathParts 也匹配完了 (除非有 * 号)
	if len(patternParts) != len(pathParts) {
		return false
	}

	return true
}

func (s *roleService) VerifyApiAuth(ctx context.Context, method, path string, roleCodes []string) (hasPermission bool, apiFound bool, err error) {
	// 1. Fetch 全部 API
	var allApis []*systemEntity.API
	err = s.cacheMgr.Fetch(ctx, cache.KeyAllApis(), "rbac_auth", []string{cache.TagRBACAPI}, cache.TTL_RBAC, &allApis, func() (interface{}, error) {
		return s.apiRepo.GetAll(ctx)
	})
	if err != nil {
		return false, false, err
	}

	var targetAPI *systemEntity.API
	for _, api := range allApis {
		if api.Method == method && matchPath(api.Path, path) {
			targetAPI = api
			break
		}
	}

	if targetAPI == nil {
		return false, false, nil // API 不存在于记录
	}

	if targetAPI.Auth == systemEntity.APINotRequireAuth {
		return true, true, nil // API 存在且免鉴权
	}

	for _, roleCode := range roleCodes {
		if roleCode == systemEntity.SuperRoleCode {
			return true, true, nil
		}
	}

	// 2. 依次 Fetch 每一个角色的拥有的 API 列表
	for _, roleCode := range roleCodes {
		var apis []*systemEntity.API
		key := cache.KeyRoleApis(roleCode)
		err = s.cacheMgr.Fetch(ctx, key, "rbac_auth", []string{cache.TagRBACRole}, cache.TTL_RBAC, &apis, func() (interface{}, error) {
			role, repoErr := s.roleRepo.GetByCode(ctx, roleCode)
			if repoErr != nil {
				return nil, repoErr
			}
			return role.Apis, nil
		})

		if err != nil {
			continue // 该角色查不到或出错，跳过检查其他角色
		}

		for _, api := range apis {
			if api.Method == method && matchPath(api.Path, path) {
				return true, true, nil
			}
		}
	}

	return false, true, nil
}
