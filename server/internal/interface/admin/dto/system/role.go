package system

type RoleQuery struct {
	Current int    `form:"current"`
	Size    int    `form:"size"`
	Name    string `form:"name"`
	Code    string `form:"code"`
	Status  string `form:"status"`
}

type CreateRoleReq struct {
	Name    string `json:"name" binding:"required"`
	Code    string `json:"code" binding:"required"`
	Desc    string `json:"desc"`
	Status  string `json:"status" binding:"oneof=0 1"`
	Menus   []uint `json:"menus"`
	Buttons []uint `json:"buttons"`
	Apis    []uint `json:"apis"`
}

type UpdateRoleReq struct {
	ID      uint   `json:"id" binding:"required"`
	Name    string `json:"name" binding:"required"`
	Code    string `json:"code" binding:"required"`
	Desc    string `json:"desc"`
	Status  string `json:"status" binding:"oneof=0 1"`
	Menus   []uint `json:"menus"`
	Buttons []uint `json:"buttons"`
	Apis    []uint `json:"apis"`
}

type UpdateRolePermissionsReq struct {
	ID      uint   `json:"id" binding:"required"`
	Menus   []uint `json:"menus"`
	Buttons []uint `json:"buttons"`
	Apis    []uint `json:"apis"`
}
