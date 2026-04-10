package system

type RoleVO struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Desc      string `json:"desc"`
	Status    string `json:"status"`
	CreatedAt string `json:"createTime"`
	Menus     []uint `json:"menus"`
	Buttons   []uint `json:"buttons"`
	Apis      []uint `json:"apis"`
}

type RoleSimpleVO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

type RoleItemVO struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Desc      string `json:"desc"`
	Status    string `json:"status"`
	Creator   string `json:"creator"`
	CreatedAt string `json:"createTime"`
	Updater   string `json:"updater"`
	UpdatedAt string `json:"updateTime"`
}
