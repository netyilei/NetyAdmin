package system

import (
	"context"
	"encoding/json"
	"fmt"

	systemEntity "silentorder/internal/domain/entity/system"
	systemVO "silentorder/internal/domain/vo/system"
	systemDto "silentorder/internal/interface/admin/dto/system"
	"silentorder/internal/pkg/cache"
	"silentorder/internal/pkg/errorx"
	"silentorder/internal/pkg/utils"
	systemRepo "silentorder/internal/repository/system"
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
	GetAllPages(ctx context.Context) ([]*systemVO.MenuSimpleVO, error)
	IsRouteExist(ctx context.Context, routeName string) (bool, error)
}

type menuService struct {
	menuRepo   systemRepo.MenuRepository
	buttonRepo systemRepo.ButtonRepository
	cacheMgr   cache.LazyCacheManager
}

func NewMenuService(menuRepo systemRepo.MenuRepository, buttonRepo systemRepo.ButtonRepository, cacheMgr cache.LazyCacheManager) MenuService {
	return &menuService{
		menuRepo:   menuRepo,
		buttonRepo: buttonRepo,
		cacheMgr:   cacheMgr,
	}
}

func toMenuVO(m *systemEntity.Menu) *systemVO.MenuVO {
	var query []systemVO.QueryItem
	if m.Query != "" {
		_ = json.Unmarshal([]byte(m.Query), &query)
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
		return nil, errorx.New(errorx.CodeNotFound, "菜单不存在")
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

	if err := s.menuRepo.Create(ctx, menu); err != nil {
		return 0, err
	}

	// Create buttons
	for _, b := range req.Buttons {
		button := &systemEntity.Button{
			MenuID: menu.ID,
			Code:   b.Code,
			Label:  b.Desc,
		}
		button.CreatedBy = operatorID
		_ = s.buttonRepo.Create(ctx, button)
	}

	// 失效缓存
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu)

	return menu.ID, nil
}

func (s *menuService) Update(ctx context.Context, req *systemDto.UpdateMenuReq, operatorID uint) error {
	menu, err := s.menuRepo.GetByID(ctx, req.ID)
	if err != nil {
		return errorx.New(errorx.CodeNotFound, "菜单不存在")
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

	if err := s.menuRepo.Update(ctx, menu); err != nil {
		return err
	}

	// Update buttons: Delete old ones and create new ones for simplicity
	_ = s.buttonRepo.DeleteByMenuID(ctx, menu.ID)
	for _, b := range req.Buttons {
		button := &systemEntity.Button{
			MenuID: menu.ID,
			Code:   b.Code,
			Label:  b.Desc,
		}
		button.CreatedBy = operatorID
		_ = s.buttonRepo.Create(ctx, button)
	}

	// 失效缓存
	_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu)

	return nil
}

func (s *menuService) Delete(ctx context.Context, id uint) error {
	_ = s.buttonRepo.DeleteByMenuID(ctx, id)
	err := s.menuRepo.Delete(ctx, id)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagRBACMenu)
	}
	return err
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
