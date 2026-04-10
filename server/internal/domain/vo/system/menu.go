package system

type MenuVO struct {
	ID              uint          `json:"id"`
	ParentID        uint          `json:"parentId"`
	Type            string        `json:"type"`
	Name            string        `json:"name"`
	RouteName       string        `json:"routeName"`
	RoutePath       string        `json:"routePath"`
	Component       string        `json:"component"`
	I18nKey         string        `json:"i18nKey"`
	Icon            string        `json:"icon"`
	IconType        string        `json:"iconType"`
	Order           int           `json:"order"`
	Status          string        `json:"status"`
	Hidden          bool          `json:"hideInMenu"`
	KeepAlive       bool          `json:"keepAlive"`
	Constant        bool          `json:"constant"`
	ActiveMenu      string        `json:"activeMenu"`
	MultiTab        bool          `json:"multiTab"`
	FixedIndexInTab *int          `json:"fixedIndexInTab"`
	Query           []QueryItem   `json:"query"`
	Href            string        `json:"href"`
	Buttons         []*MenuButton `json:"buttons"`
}

type QueryItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MenuButton struct {
	Code string `json:"code"`
	Desc string `json:"desc"`
}

type MenuSimpleVO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type MenuTreeVO struct {
	ID        uint          `json:"id"`
	ParentID  uint          `json:"parentId"`
	Type      string        `json:"type"`
	Label     string        `json:"label"`
	RouteName string        `json:"routeName"`
	RoutePath string        `json:"routePath"`
	Component string        `json:"component"`
	I18nKey   string        `json:"i18nKey"`
	Icon      string        `json:"icon"`
	IconType  string        `json:"iconType"`
	Order     int           `json:"order"`
	Hidden    bool          `json:"hideInMenu"`
	KeepAlive bool          `json:"keepAlive"`
	Constant  bool          `json:"constant"`
	Href      string        `json:"href"`
	Children  []*MenuTreeVO `json:"children"`
	Buttons   []*MenuButton `json:"buttons"`
}

type MenuButtonTreeVO struct {
	ID       string              `json:"id"`
	Label    string              `json:"label"`
	Type     string              `json:"type"` // "menu" or "button"
	Children []*MenuButtonTreeVO `json:"children,omitempty"`
}

type MenuApiTreeVO struct {
	ID       string           `json:"id"`
	Label    string           `json:"label"`
	Type     string           `json:"type"` // "menu" or "api"
	Method   string           `json:"method,omitempty"`
	Path     string           `json:"path,omitempty"`
	Children []*MenuApiTreeVO `json:"children,omitempty"`
}
