package system

import systemVO "silentorder/internal/domain/vo/system"

type CreateMenuReq struct {
	ParentID        uint                  `json:"parentId"`
	Type            string                `json:"type" binding:"required"`
	Name            string                `json:"name" binding:"required"`
	RouteName       string                `json:"routeName" binding:"required"`
	RoutePath       string                `json:"routePath" binding:"required"`
	Component       string                `json:"component"`
	I18nKey         string                `json:"i18nKey"`
	Icon            string                `json:"icon"`
	IconType        string                `json:"iconType"`
	Order           int                   `json:"order"`
	Status          string                `json:"status" binding:"oneof=0 1"`
	Hidden          bool                  `json:"hideInMenu"`
	KeepAlive       bool                  `json:"keepAlive"`
	Constant        bool                  `json:"constant"`
	ActiveMenu      string                `json:"activeMenu"`
	MultiTab        bool                  `json:"multiTab"`
	FixedIndexInTab *int                  `json:"fixedIndexInTab"`
	Query           []systemVO.QueryItem  `json:"query"`
	Href            string                `json:"href"`
	Buttons         []systemVO.MenuButton `json:"buttons"`
}

type UpdateMenuReq struct {
	ID              uint                  `json:"id" binding:"required"`
	ParentID        uint                  `json:"parentId"`
	Type            string                `json:"type" binding:"required"`
	Name            string                `json:"name" binding:"required"`
	RouteName       string                `json:"routeName" binding:"required"`
	RoutePath       string                `json:"routePath" binding:"required"`
	Component       string                `json:"component"`
	I18nKey         string                `json:"i18nKey"`
	Icon            string                `json:"icon"`
	IconType        string                `json:"iconType"`
	Order           int                   `json:"order"`
	Status          string                `json:"status" binding:"oneof=0 1"`
	Hidden          bool                  `json:"hideInMenu"`
	KeepAlive       bool                  `json:"keepAlive"`
	Constant        bool                  `json:"constant"`
	ActiveMenu      string                `json:"activeMenu"`
	MultiTab        bool                  `json:"multiTab"`
	FixedIndexInTab *int                  `json:"fixedIndexInTab"`
	Query           []systemVO.QueryItem  `json:"query"`
	Href            string                `json:"href"`
	Buttons         []systemVO.MenuButton `json:"buttons"`
}

type MenuQuery struct {
	Name     string  `form:"name"`
	Status   *string `form:"status"`
	ParentID *uint   `form:"parentId"`
	Current  int     `form:"current"`
	Size     int     `form:"size"`
}
