package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"gorm.io/gorm"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	systemVO "NetyAdmin/internal/domain/vo/system"
	systemDto "NetyAdmin/internal/interface/admin/dto/system"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/utils"
	systemRepo "NetyAdmin/internal/repository/system"
)

type MenuService interface {
	List(ctx context.Context, req *systemDto.MenuQuery) ([]*systemVO.MenuVO, int64, error)
	GetTree(ctx context.Context) ([]*systemVO.MenuTreeVO, error)
	GetByID(ctx context.Context, id uint) (*systemVO.MenuVO, error)
	Create(ctx context.Context, req *systemDto.CreateMenuReq, operatorID uint) (uint, error)
	GetMenuButtonTree(ctx context.Context) ([]*systemVO.MenuButtonTreeVO, error)
	GetMenuApiTree(ctx context.Context) ([]*systemVO.MenuApiTreeVO, error)
	Update(ctx context.Context, req *systemDto.UpdateMenuReq, operatorID uint) error
	Delete(ctx context.Context, id uint) error
	DeleteBatch(ctx context.Context, ids []uint) error
	GetAllPages(ctx context.Context) ([]*systemVO.MenuSimpleVO, error)
	IsRouteExist(ctx context.Context, routeName string) (bool, error)
	GetTreeByRoleCodes(ctx context.Context, roleCodes []string) ([]*systemVO.MenuTreeVO, error)
}

type menuService struct {
	menuRepo   systemRepo.MenuRepository
	buttonRepo systemRepo.ButtonRepository
	apiRepo    systemRepo.APIRepository
	roleRepo   systemRepo.RoleRepository
	cacheMgr   cache.LazyCacheManager
	tm         *database.TransactionManager
}

func NewMenuService(menuRepo systemRepo.MenuRepository, buttonRepo systemRepo.ButtonRepository, apiRepo systemRepo.APIRepository, roleRepo systemRepo.RoleRepository, cacheMgr cache.LazyCacheManager, tm *database.TransactionManager) MenuService {
	return &menuService{
		menuRepo:   menuRepo,
		buttonRepo: buttonRepo,
		apiRepo:    apiRepo,
		roleRepo:   roleRepo,
		cacheMgr:   cacheMgr,
		tm:         tm,
	}
}

func toMenuVO(m *systemEntity.Menu) *systemVO.MenuVO {
	var query []systemVO.QueryItem
	if m.Query != "" {
		if err := json.Unmarshal([]byte(m.Query), &query); err != nil {
			slog.Warn("unmarshal menu query failed", "menuID", m.ID, "err", err)
		}
	}

	buttons := make([]*systemVO.MenuButton, 0, len(m.Buttons))
	for _, b := range m.Buttons {
		buttons = append(buttons, &systemVO.MenuButton{
			Code: b.Code,
			Desc: b.Label,
		})
	}

	return &systemVO.MenuVO{
		ID:              m.ID,
		ParentID:        m.ParentID,
		Type:            m.Type,
		Name:            m.Name,
		RouteName:       m.RouteName,
		RoutePath:       m.RoutePath,
		Component:       m.Component,
		I18nKey:         m.I18nKey,
		Icon:            m.Icon,
		IconType:        m.IconType,
		Order:           m.Order,
		Status:          m.Status,
		Hidden:          m.Hidden,
		KeepAlive:       m.KeepAlive,
		Constant:        m.Constant,
		ActiveMenu:      m.ActiveMenu,
		MultiTab:        m.MultiTab,
		FixedIndexInTab: m.FixedIndexInTab,
		Query:           query,
		Href:            m.Href,
		Buttons:         buttons,
	}
}

func (s *menuService) List(ctx context.Context, req *systemDto.MenuQuery) ([]*systemVO.MenuVO, int64, error) {
	query := &systemRepo.MenuRepoQuery{
		Name:    req.Name,
		Status:  req.Status,
		Current: req.Current,
		Size:    req.Size,
	}

	menus, total, err := s.menuRepo.List(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	vos := make([]*systemVO.MenuVO, 0, len(menus))
	for _, m := range menus {
		vos = append(vos, toMenuVO(m))
	}
	return vos, total, nil
}

func (s *menuService) GetTree(ctx context.Context) ([]*systemVO.MenuTreeVO, error) {
	var tree []*systemVO.MenuTreeVO
	err := s.cacheMgr.Fetch(ctx, cache.KeyMenuTree(), "rbac", []string{cache.TagRBACMenu}, cache.TTL_RBAC, &tree, func() (interface{}, error) {
		menus, err := s.menuRepo.GetAll(ctx)
		if err != nil {
			return nil, err
		}
		return s.buildTree(menus), nil
	})
	return tree, err
}

func (s *menuService) GetMenuButtonTree(ctx context.Context) ([]*systemVO.MenuButtonTreeVO, error) {
	var tree []*systemVO.MenuButtonTreeVO
	err := s.cacheMgr.Fetch(ctx, cache.KeyMenuButtonTree(), "rbac", []string{cache.TagRBACMenu}, cache.TTL_RBAC, &tree, func() (interface{}, error) {
		menus, err := s.menuRepo.GetAllWithButtons(ctx)
		if err != nil {
			return nil, err
		}
		return s.buildButtonTree(menus), nil
	})
	return tree, err
}

func (s *menuService) GetMenuApiTree(ctx context.Context) ([]*systemVO.MenuApiTreeVO, error) {
	var tree []*systemVO.MenuApiTreeVO
	err := s.cacheMgr.Fetch(ctx, cache.KeyMenuApiTree(), "rbac", []string{cache.TagRBACMenu}, cache.TTL_RBAC, &tree, func() (interface{}, error) {
		menus, err := s.menuRepo.GetAllWithApis(ctx)
		if err != nil {
			return nil, err
		}
		return s.buildApiTree(menus), nil
	})
	return tree, err
}

func (s *menuService) buildButtonTree(menus []systemEntity.Menu) []*systemVO.MenuButtonTreeVO {
	return utils.BuildTree(
		menus,
		func(m systemEntity.Menu) uint { return m.ParentID },
		func(m systemEntity.Menu) uint { return m.ID },
		func(m systemEntity.Menu, subChildren []*systemVO.MenuButtonTreeVO) (*systemVO.MenuButtonTreeVO, bool) {
			// 收集当前项按钮
			var buttonNodes []*systemVO.MenuButtonTreeVO
			if len(m.Buttons) > 0 {
				for _, b := range m.Buttons {
					buttonNodes = append(buttonNodes, &systemVO.MenuButtonTreeVO{
						ID:    fmt.Sprintf("b_%d", b.ID),
						Label: b.Label,
						Type:  "button",
					})
				}
			}

			// 过滤: 仅当有按钮或有子菜单(其下有按钮)时才保留
			if len(subChildren) > 0 || len(buttonNodes) > 0 {
				return &systemVO.MenuButtonTreeVO{
					ID:       fmt.Sprintf("m_%d", m.ID),
					Label:    m.Name,
					Type:     "menu",
					Children: append(subChildren, buttonNodes...),
				}, true
			}
			return nil, false
		},
	)
}

func (s *menuService) buildApiTree(menus []systemEntity.Menu) []*systemVO.MenuApiTreeVO {
	return utils.BuildTree(
		menus,
		func(m systemEntity.Menu) uint { return m.ParentID },
		func(m systemEntity.Menu) uint { return m.ID },
		func(m systemEntity.Menu, subChildren []*systemVO.MenuApiTreeVO) (*systemVO.MenuApiTreeVO, bool) {
			// 收集当前项 API
			var apiNodes []*systemVO.MenuApiTreeVO
			if len(m.Apis) > 0 {
				for _, a := range m.Apis {
					apiNodes = append(apiNodes, &systemVO.MenuApiTreeVO{
						ID:     fmt.Sprintf("a_%d", a.ID),
						Label:  a.Name,
						Type:   "api",
						Method: a.Method,
						Path:   a.Path,
					})
				}
			}

			// 过滤: 仅当有 api 或有子菜单(其下有 api)时才保留
			if len(subChildren) > 0 || len(apiNodes) > 0 {
				return &systemVO.MenuApiTreeVO{
					ID:       fmt.Sprintf("m_%d", m.ID),
					Label:    m.Name,
					Type:     "menu",
					Children: append(subChildren, apiNodes...),
				}, true
			}
			return nil, false
		},
	)
}

func (s *menuService) GetByID(ctx context.Context, id uint) (*systemVO.MenuVO, error) {
	m, err := s.menuRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeNotFound, "菜单不存在")
		}
		slog.Error("menuRepo.GetByID failed", "menuID", id, "err", err)
		return nil, fmt.Errorf("menuRepo.GetByID: %w", err)
	}
	return toMenuVO(m), nil
}

func (s *menuService) Create(ctx context.Context, req *systemDto.CreateMenuReq, operatorID uint) (uint, error) {
	queryJson, _ := json.Marshal(req.Query)

	menu := &systemEntity.Menu{
		ParentID:        req.ParentID,
		Type:            req.Type,
		Name:            req.Name,
		RouteName:       req.RouteName,
		RoutePath:       req.RoutePath,
		Component:       req.Component,
		I18nKey:         req.I18nKey,
		Icon:            req.Icon,
		IconType:        req.IconType,
		Order:           req.Order,
		Status:          req.Status,
		Hidden:          req.Hidden,
		KeepAlive:       req.KeepAlive,
		Constant:        req.Constant,
		ActiveMenu:      req.ActiveMenu,
		MultiTab:        req.MultiTab,
		FixedIndexInTab: req.FixedIndexInTab,
		Query:           string(queryJson),
		Href:            req.Href,
	}
	menu.CreatedBy = operatorID

	// TM 单事务原子完成「创建 menu + 创建 buttons」，任一失败整体回滚。
	// 不再像旧实现那样 button 创建失败仅记录日志导致 menu 残留但 buttons 缺失的中间态。
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.menuRepo.Create(txCtx, menu); err != nil {
		slog.Error("menu create: create menu failed", "err", err)
		s.tm.Rollback(tx)
		return 0, err
	}
	for _, b := range req.Buttons {
		button := &systemEntity.Button{
			MenuID: menu.ID,
			Code:   b.Code,
			Label:  b.Desc,
		}
		button.CreatedBy = operatorID
		if err := s.buttonRepo.Create(txCtx, button); err != nil {
			slog.Error("menu create: create button failed", "menuID", menu.ID, "buttonCode", b.Code, "err", err)
			s.tm.Rollback(tx)
			return 0, err
		}
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("menu create: commit failed", "err", err)
		return 0, err
	}

	// 失效缓存
	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagRBACMenu, "err", err)
	}

	return menu.ID, nil
}

func (s *menuService) Update(ctx context.Context, req *systemDto.UpdateMenuReq, operatorID uint) error {
	menu, err := s.menuRepo.GetByID(ctx, req.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, "菜单不存在")
		}
		slog.Error("menuRepo.GetByID failed", "menuID", req.ID, "err", err)
		return fmt.Errorf("menuRepo.GetByID: %w", err)
	}

	queryJson, _ := json.Marshal(req.Query)

	menu.ParentID = req.ParentID
	menu.Type = req.Type
	menu.Name = req.Name
	menu.RouteName = req.RouteName
	menu.RoutePath = req.RoutePath
	menu.Component = req.Component
	menu.I18nKey = req.I18nKey
	menu.Icon = req.Icon
	menu.IconType = req.IconType
	menu.Order = req.Order
	menu.Status = req.Status
	menu.Hidden = req.Hidden
	menu.KeepAlive = req.KeepAlive
	menu.Constant = req.Constant
	menu.ActiveMenu = req.ActiveMenu
	menu.MultiTab = req.MultiTab
	menu.FixedIndexInTab = req.FixedIndexInTab
	menu.Query = string(queryJson)
	menu.Href = req.Href
	menu.UpdatedBy = operatorID

	// 自引用校验：禁止将菜单的 parent_id 设为自身（会导致 BuildTree 无限递归）
	if req.ParentID != 0 && req.ParentID == req.ID {
		return errorx.New(errorx.CodeInvalidParams, "父菜单不能为自身")
	}
	// 循环引用校验：禁止创建 A→B→A 循环链（沿 parentID 向上查找，若回到当前 id 则拒绝）
	if req.ParentID != 0 {
		currentParentID := req.ParentID
		for i := 0; i < 100; i++ { // 最大深度 100，防止恶意构造超长链
			if currentParentID == 0 {
				break // 到达根节点
			}
			if currentParentID == req.ID {
				return errorx.New(errorx.CodeInvalidParams, "父菜单形成循环引用")
			}
			parent, err := s.menuRepo.GetByID(ctx, currentParentID)
			if err != nil {
				break // 父菜单不存在，由后续逻辑处理
			}
			currentParentID = parent.ParentID
		}
	}

	// 构造新 buttons 切片（替换语义）
	buttons := make([]*systemEntity.Button, 0, len(req.Buttons))
	for _, b := range req.Buttons {
		button := &systemEntity.Button{
			MenuID: menu.ID,
			Code:   b.Code,
			Label:  b.Desc,
		}
		button.CreatedBy = operatorID
		buttons = append(buttons, button)
	}

	// TM 单事务原子完成「更新 menu 主数据 + 清理旧 buttons 角色关联 + 硬删除旧 buttons + 创建新 buttons」。
	// 不再用 _ = 吞错（旧实现 Update 成功但 button 操作失败时数据不一致且被静默）。
	// 清理 admin_role_buttons 在硬删除 buttons 之前，避免孤儿角色-按钮关联（与 Delete 的 C4 修复保持一致）。
	txCtx, tx := s.tm.Begin(ctx)
	if err := s.menuRepo.Update(txCtx, menu); err != nil {
		slog.Error("menu update: update menu failed", "menuID", menu.ID, "err", err)
		s.tm.Rollback(tx)
		return err
	}
	// 先清理 admin_role_buttons 中指向本 menu 下 buttons 的角色关联（子查询定位），
	// 否则硬删除 buttons 后 admin_role_buttons 残留孤儿引用。
	if err := s.buttonRepo.ClearRoleButtonsByMenuID(txCtx, menu.ID); err != nil {
		slog.Error("menu update: clear role buttons failed", "menuID", menu.ID, "err", err)
		s.tm.Rollback(tx)
		return err
	}
	if err := s.buttonRepo.DeleteByMenuID(txCtx, menu.ID); err != nil {
		slog.Error("menu update: delete old buttons failed", "menuID", menu.ID, "err", err)
		s.tm.Rollback(tx)
		return err
	}
	for _, button := range buttons {
		if err := s.buttonRepo.Create(txCtx, button); err != nil {
			slog.Error("menu update: create button failed", "menuID", menu.ID, "buttonCode", button.Code, "err", err)
			s.tm.Rollback(tx)
			return err
		}
	}
	if err := s.tm.Commit(tx); err != nil {
		slog.Error("menu update: commit failed", "menuID", menu.ID, "err", err)
		return err
	}

	// 失效缓存：TagRBACMenu（menu/button 树结构变化）+ TagRBACRole（角色-按钮关联被清理，KeyRoleButtons 过期）
	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu, cache.TagRBACRole); err != nil {
		slog.Error("invalidate cache failed", "tags", []string{cache.TagRBACMenu, cache.TagRBACRole}, "err", err)
	}

	return nil
}

func (s *menuService) Delete(ctx context.Context, id uint) error {
	// 用 WithTransaction 闭包 API 包裹「预校验子菜单 + 清理 buttons 角色关联 + 硬删除 buttons
	// + 清理 APIs 角色关联 + 硬删除 APIs + 清理 admin_role_menus + 清理 admin_role.home_menu_id 引用
	// + 硬删除 menu」七步原子写。
	// 任一步失败自动 Rollback；panic 自动 Rollback 后重抛让 recovery 中间件捕获 + Sentry 上报。
	// 预校验（HasChildren）放在事务内并使用 txCtx，与 §13.7 前置校验模式一致，避免 TOCTOU 竞态
	// （事务外校验与事务内删除之间存在窗口，子菜单可能在两者之间被创建）。
	// Bug 修复保留：C2 级联删除归属该 menu 的 api + 清理 admin_role_apis；
	//              C3 清理 admin_role.home_menu_id 引用（置 0）；
	//              C4 在硬删除 buttons 之前，先清理 admin_role_buttons 关联。
	err := s.tm.WithTransaction(ctx, func(txCtx context.Context) error {
		// 0. 预校验：存在子菜单时拒绝删除（子菜单是独立实体，不在本事务内级联删除）。
		// 调用方应先删除所有子菜单再删除父菜单，避免误删整个菜单子树。
		hasChildren, err := s.menuRepo.HasChildren(txCtx, id)
		if err != nil {
			slog.Error("menu delete: check children failed", "menuID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, "查询子菜单失败")
		}
		if hasChildren {
			return errorx.New(errorx.CodeMenuHasChildren)
		}

		// 1. 清理 buttons 角色关联 (admin_role_buttons via 子查询按 menu_id 定位 button)
		if err := s.buttonRepo.ClearRoleButtonsByMenuID(txCtx, id); err != nil {
			slog.Error("menu delete: clear role buttons by menu failed", "menuID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, "菜单删除失败")
		}
		// 2. 硬删除 buttons (Unscoped, by menu_id)
		if err := s.buttonRepo.DeleteByMenuID(txCtx, id); err != nil {
			slog.Error("menu delete: delete buttons failed", "menuID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, "菜单删除失败")
		}
		// 3. 清理 APIs 角色关联 (admin_role_apis via 子查询按 menu_id 定位 api)
		if err := s.apiRepo.ClearRoleApisByMenuID(txCtx, id); err != nil {
			slog.Error("menu delete: clear role apis by menu failed", "menuID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, "菜单删除失败")
		}
		// 4. 硬删除 APIs (Unscoped, by menu_id)
		if err := s.apiRepo.DeleteByMenuID(txCtx, id); err != nil {
			slog.Error("menu delete: delete apis failed", "menuID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, "菜单删除失败")
		}
		// 5. 清理 admin_role_menus 关联
		if err := s.menuRepo.ClearRoleMenus(txCtx, id); err != nil {
			slog.Error("menu delete: clear role menus failed", "menuID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, "菜单删除失败")
		}
		// 6. 清理 admin_role.home_menu_id 引用 (置 0)
		if err := s.roleRepo.ClearHomeMenuRef(txCtx, id); err != nil {
			slog.Error("menu delete: clear home menu ref failed", "menuID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, "菜单删除失败")
		}
		// 7. 硬删除 menu 主数据
		if err := s.menuRepo.Delete(txCtx, id); err != nil {
			slog.Error("menu delete: delete menu failed", "menuID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, "菜单删除失败")
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 事务后失效缓存
	// 菜单删除级联清理了 admin_role_buttons / admin_role_apis / admin_role_menus，
	// 直接改变了各角色可访问的 menu/button/api 权限集，因此需同时失效三类缓存。
	if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu, cache.TagRBACRole, cache.TagRBACAPI); cErr != nil {
		slog.Error("invalidate cache failed", "tags", []string{cache.TagRBACMenu, cache.TagRBACRole, cache.TagRBACAPI}, "err", cErr)
	}
	return nil
}

// DeleteBatch 批量删除菜单（fail-closed 语义）。
//
// 设计（方案 A：复用 Delete，外层包 skipped + fail-closed）：
//   - 复用 s.Delete 已完善的 7 步 TM 事务逻辑，不重复实现级联清理
//   - 业务规则拒绝（菜单不存在 CodeNotFound / 存在子菜单 CodeMenuHasChildren）→ skip + continue
//   - 事务失败（CodeInternalError 或其他）→ 失效缓存 + 立即 return（fail-closed）
//   - 已提交的 id 保持删除状态，未处理的 id 不受影响
//
// 与 admin DeleteBatch 设计一致（参考 admin_manage.go:316）。
func (s *menuService) DeleteBatch(ctx context.Context, ids []uint) error {
	var skipped []string
	for _, id := range ids {
		err := s.Delete(ctx, id)
		if err == nil {
			continue
		}
		// 区分业务规则拒绝 vs 事务失败
		var bizErr *errorx.BizError
		if errors.As(err, &bizErr) {
			switch bizErr.Code {
			case errorx.CodeNotFound:
				skipped = append(skipped, fmt.Sprintf("菜单 %d：不存在", id))
				continue
			case errorx.CodeMenuHasChildren:
				skipped = append(skipped, fmt.Sprintf("菜单 %d：存在子菜单，请先删除子菜单", id))
				continue
			}
		}
		// 其他错误（包括 CodeInternalError 事务失败）→ fail-closed 立即返回
		// 已删的 menu 缓存必须失效（兜底），避免列表/树缓存残留已删菜单
		slog.Error("menu delete batch: delete menu failed", "menuID", id, "err", err)
		if cErr := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu, cache.TagRBACRole, cache.TagRBACAPI); cErr != nil {
			slog.Error("invalidate cache failed after tx failure", "err", cErr)
		}
		return err
	}
	// 全部处理完后统一失效缓存
	if err := s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu, cache.TagRBACRole, cache.TagRBACAPI); err != nil {
		slog.Error("invalidate cache failed", "err", err)
	}
	if len(skipped) > 0 {
		return errorx.New(errorx.CodeForbidden, fmt.Sprintf("部分菜单被跳过：%s", strings.Join(skipped, "; ")))
	}
	return nil
}

func (s *menuService) GetAllPages(ctx context.Context) ([]*systemVO.MenuSimpleVO, error) {
	menus, err := s.menuRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	var vos []*systemVO.MenuSimpleVO
	for _, m := range menus {
		if m.Type == systemEntity.MenuTypePage {
			vos = append(vos, &systemVO.MenuSimpleVO{
				ID:   m.ID,
				Name: m.Name,
			})
		}
	}
	return vos, nil
}

func (s *menuService) buildTree(menus []systemEntity.Menu) []*systemVO.MenuTreeVO {
	return utils.BuildTree(
		menus,
		func(m systemEntity.Menu) uint { return m.ParentID },
		func(m systemEntity.Menu) uint { return m.ID },
		func(m systemEntity.Menu, children []*systemVO.MenuTreeVO) (*systemVO.MenuTreeVO, bool) {
			node := &systemVO.MenuTreeVO{
				ID:        m.ID,
				ParentID:  m.ParentID,
				Type:      m.Type,
				Label:     m.Name,
				RouteName: m.RouteName,
				RoutePath: m.RoutePath,
				Component: m.Component,
				I18nKey:   m.I18nKey,
				Icon:      m.Icon,
				IconType:  m.IconType,
				Order:     m.Order,
				Hidden:    m.Hidden,
				KeepAlive: m.KeepAlive,
				Constant:  m.Constant,
				Href:      m.Href,
			}
			if len(children) > 0 {
				node.Children = children
			}
			return node, true
		},
	)
}

func (s *menuService) IsRouteExist(ctx context.Context, routeName string) (bool, error) {
	return s.menuRepo.ExistsByRouteName(ctx, routeName)
}

func (s *menuService) GetTreeByRoleCodes(ctx context.Context, roleCodes []string) ([]*systemVO.MenuTreeVO, error) {
	// 如果是超级管理员，直接返回全量树
	for _, code := range roleCodes {
		if code == systemEntity.SuperRoleCode {
			return s.GetTree(ctx)
		}
	}

	var tree []*systemVO.MenuTreeVO
	// 使用角色编码生成的 key，确保不同角色的菜单缓存隔离
	// 简单起见，这里按角色编码排序后拼接作为 key
	utils.SliceSort(roleCodes)
	key := fmt.Sprintf("cache:rbac:menu:roles:%v", roleCodes)

	err := s.cacheMgr.Fetch(ctx, key, "rbac", []string{cache.TagRBACMenu}, cache.TTL_RBAC, &tree, func() (interface{}, error) {
		menus, err := s.menuRepo.GetByRoleCodes(ctx, roleCodes)
		if err != nil {
			return nil, err
		}

		// 将 []*systemEntity.Menu 转换为 []systemEntity.Menu 以匹配 buildTree 的参数类型
		menuEntities := make([]systemEntity.Menu, len(menus))
		for i, m := range menus {
			menuEntities[i] = *m
		}
		return s.buildTree(menuEntities), nil
	})
	return tree, err
}
