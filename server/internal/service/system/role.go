package system

import (
	"context"
	"time"

	"NetyAdmin/internal/domain/entity"
	systemEntity "NetyAdmin/internal/domain/entity/system"
	systemVO "NetyAdmin/internal/domain/vo/system"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/utils"
	systemRepo "NetyAdmin/internal/repository/system"
	"strings"
)

type RoleService interface {
	List(ctx context.Context, req *systemDto.RoleQuery) ([]*systemVO.RoleItemVO, int64, error)
	GetByID(ctx context.Context, id uint) (*systemVO.RoleVO, error)
	Create(ctx context.Context, req *systemDto.CreateRoleReq, operatorID uint) (uint, error)
	Update(ctx context.Context, req *systemDto.UpdateRoleReq, operatorID uint) error
	Delete(ctx context.Context, id uint) error
	DeleteBatch(ctx context.Context, ids []uint) error
	GetAll(ctx context.Context) ([]*systemVO.RoleSimpleVO, error)
	UpdatePermissions(ctx context.Context, req *systemDto.UpdateRolePermissionsReq) error
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
}

func NewRoleService(
	roleRepo systemRepo.RoleRepository,
	menuRepo systemRepo.MenuRepository,
	apiRepo systemRepo.APIRepository,
	buttonRepo systemRepo.ButtonRepository,
	cacheMgr cache.LazyCacheManager,
) RoleService {
	return &roleService{
		roleRepo:   roleRepo,
		menuRepo:   menuRepo,
		apiRepo:    apiRepo,
		buttonRepo: buttonRepo,
		cacheMgr:   cacheMgr,
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
		return nil, errorx.New(errorx.CodeNotFound, "角色不存在")
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
	role, err := s.roleRepo.GetByID(ctx, req.ID)
	if err != nil {
		return errorx.New(errorx.CodeNotFound, "角色不存在")
	}

	if role.Code == systemEntity.SuperRoleCode {
		return errorx.New(errorx.CodeCannotModifySuper, "不允许修改超级管理员角色")
	}

	if req.Code != "" && req.Code != role.Code {
		exists, err := s.roleRepo.ExistsByCode(ctx, req.Code, req.ID)
		if err != nil {
			return err
		}
		if exists {
			return errorx.New(errorx.CodeAlreadyExists, "角色编码已存在")
		}
		role.Code = req.Code
	}

	role.Name = req.Name
	role.Description = req.Desc
	role.Status = req.Status
	role.UpdatedBy = operatorID

	err = s.roleRepo.Update(ctx, role)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu)
	}
	return err
}

func (s *roleService) Delete(ctx context.Context, id uint) error {
	role, err := s.roleRepo.GetByID(ctx, id)
	if err != nil {
		return errorx.New(errorx.CodeNotFound, "角色不存在")
	}

	if role.Code == systemEntity.SuperRoleCode {
		return errorx.New(errorx.CodeCannotDeleteSuper, "不允许删除超级管理员角色")
	}

	err = s.roleRepo.Delete(ctx, id)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu)
	}
	return err
}

func (s *roleService) DeleteBatch(ctx context.Context, ids []uint) error {
	for _, id := range ids {
		role, err := s.roleRepo.GetByID(ctx, id)
		if err != nil {
			continue
		}
		if role.Code == systemEntity.SuperRoleCode {
			continue
		}
		_ = s.roleRepo.Delete(ctx, id)
	}
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu)
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

func (s *roleService) UpdatePermissions(ctx context.Context, req *systemDto.UpdateRolePermissionsReq) error {
	role, err := s.roleRepo.GetByID(ctx, req.ID)
	if err != nil {
		return errorx.New(errorx.CodeNotFound, "角色不存在")
	}

	role.Menus = make([]*systemEntity.Menu, 0, len(req.Menus))
	for _, id := range req.Menus {
		role.Menus = append(role.Menus, &systemEntity.Menu{Model: entity.Model{ID: id}})
	}

	role.Apis = make([]*systemEntity.API, 0, len(req.Apis))
	for _, id := range req.Apis {
		role.Apis = append(role.Apis, &systemEntity.API{Model: entity.Model{ID: id}})
	}

	role.Buttons = make([]*systemEntity.Button, 0, len(req.Buttons))
	for _, id := range req.Buttons {
		role.Buttons = append(role.Buttons, &systemEntity.Button{Model: entity.Model{ID: id}})
	}

	err = s.roleRepo.Update(ctx, role)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu)
	}
	return err
}

func (s *roleService) GetRoleMenusWithHome(ctx context.Context, roleID uint) (map[string]interface{}, error) {
	var result map[string]interface{}
	key := cache.KeyRoleMenus(roleID)
	err := s.cacheMgr.Fetch(ctx, key, "rbac_menu", []string{cache.TagRBACRole}, cache.TTL_RBAC, &result, func() (interface{}, error) {
		role, err := s.roleRepo.GetByID(ctx, roleID)
		if err != nil {
			return nil, errorx.New(errorx.CodeNotFound, "角色不存在")
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
		return errorx.New(errorx.CodeNotFound, "角色不存在")
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
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole, cache.TagRBACMenu)
	}
	return err
}

func (s *roleService) GetRoleButtons(ctx context.Context, roleID uint) ([]uint, error) {
	var buttonIDs []uint
	key := cache.KeyRoleButtons(roleID)
	err := s.cacheMgr.Fetch(ctx, key, "rbac_menu", []string{cache.TagRBACRole}, cache.TTL_RBAC, &buttonIDs, func() (interface{}, error) {
		role, err := s.roleRepo.GetByID(ctx, roleID)
		if err != nil {
			return nil, errorx.New(errorx.CodeNotFound, "角色不存在")
		}

		return utils.SliceMap(role.Buttons, func(b *systemEntity.Button) uint { return b.ID }), nil
	})
	return buttonIDs, err
}

func (s *roleService) UpdateButtons(ctx context.Context, roleID uint, buttonIDs []uint) error {
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return errorx.New(errorx.CodeNotFound, "角色不存在")
	}

	role.Buttons = make([]*systemEntity.Button, 0, len(buttonIDs))
	for _, id := range buttonIDs {
		role.Buttons = append(role.Buttons, &systemEntity.Button{Model: entity.Model{ID: id}})
	}

	err = s.roleRepo.Update(ctx, role)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole)
	}
	return err
}

func (s *roleService) GetRoleAPIs(ctx context.Context, roleID uint) ([]uint, error) {
	var apiIDs []uint
	key := cache.KeyRoleApiIDs(roleID) // 这里专门缓存 ID列表用于编辑
	err := s.cacheMgr.Fetch(ctx, key, "rbac_auth", []string{cache.TagRBACRole}, cache.TTL_RBAC, &apiIDs, func() (interface{}, error) {
		role, err := s.roleRepo.GetByID(ctx, roleID)
		if err != nil {
			return nil, errorx.New(errorx.CodeNotFound, "角色不存在")
		}

		return utils.SliceMap(role.Apis, func(a *systemEntity.API) uint { return a.ID }), nil
	})
	return apiIDs, err
}

func (s *roleService) UpdateAPIs(ctx context.Context, roleID uint, apiIDs []uint) error {
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return errorx.New(errorx.CodeNotFound, "角色不存在")
	}

	role.Apis = make([]*systemEntity.API, 0, len(apiIDs))
	for _, id := range apiIDs {
		role.Apis = append(role.Apis, &systemEntity.API{Model: entity.Model{ID: id}})
	}

	err = s.roleRepo.Update(ctx, role)
	if err == nil {
		// 失效角色相关缓存（包括权限 ID 列表和鉴权所用的 API 列表）
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACRole)
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
