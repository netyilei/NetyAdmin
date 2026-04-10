package route

import (
	"github.com/gin-gonic/gin"

	systemVO "NetyAdmin/internal/domain/vo/system"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	systemService "NetyAdmin/internal/service/system"
)

type RouteHandler struct {
	menuService systemService.MenuService
}

func NewRouteHandler(menuService systemService.MenuService) *RouteHandler {
	return &RouteHandler{menuService: menuService}
}

func traverseTree(menus []*systemVO.MenuTreeVO) []UserRouteItem {
	var res []UserRouteItem
	if menus == nil {
		return res
	}
	for _, m := range menus {
		item := UserRouteItem{
			Name:      m.RouteName,
			Path:      m.RoutePath,
			Component: m.Component,
			Meta: RouteMeta{
				Title:      m.Label,
				I18nKey:    m.I18nKey,
				Icon:       m.Icon,
				Order:      m.Order,
				HideInMenu: m.Hidden,
				KeepAlive:  m.KeepAlive,
				Href:       m.Href,
			},
		}
		if len(m.Children) > 0 {
			item.Children = traverseTree(m.Children)
		}
		res = append(res, item)
	}
	return res
}

func (h *RouteHandler) GetUserRoutes(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.FailWithCode(c, errorx.CodeUnauthorized, "未授权")
		return
	}
	_ = userID

	tree, err := h.menuService.GetTree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	routes := traverseTree(tree)

	response.Success(c, GetUserRoutesResp{
		Routes: routes,
		Home:   "home",
	})
}

func (h *RouteHandler) IsRouteExist(c *gin.Context) {
	routeName := c.Query("routeName")
	if routeName == "" {
		response.FailWithCode(c, errorx.CodeInvalidParams, "routeName不能为空")
		return
	}

	exists, err := h.menuService.IsRouteExist(c.Request.Context(), routeName)
	if err != nil {
		response.Fail(c, err)
		return
	}

	// 前端直接取 data 作为 boolean
	response.Success(c, exists)
}

type RouteMeta struct {
	Title      string `json:"title"`
	I18nKey    string `json:"i18nKey,omitempty"`
	Icon       string `json:"icon,omitempty"`
	Order      int    `json:"order,omitempty"`
	HideInMenu bool   `json:"hideInMenu,omitempty"`
	KeepAlive  bool   `json:"keepAlive,omitempty"`
	Href       string `json:"href,omitempty"`
}

type UserRouteItem struct {
	Name      string          `json:"name"`
	Path      string          `json:"path"`
	Component string          `json:"component"`
	Meta      RouteMeta       `json:"meta"`
	Children  []UserRouteItem `json:"children,omitempty"`
}

type GetUserRoutesResp struct {
	Routes []UserRouteItem `json:"routes"`
	Home   string          `json:"home"`
}
