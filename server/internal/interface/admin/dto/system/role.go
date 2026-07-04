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

// UpdateRoleReq 更新角色请求
// Code 为业务唯一标识，创建后不可变更（基座设计原则，与 RBAC role code、菜单 code、字典 Code 一致）。
type UpdateRoleReq struct {
	ID      uint   `json:"id" binding:"required"`
	Name    string `json:"name" binding:"required"`
	Desc    string `json:"desc"`
	Status  string `json:"status" binding:"oneof=0 1"`
	Menus   []uint `json:"menus"`
	Buttons []uint `json:"buttons"`
	Apis    []uint `json:"apis"`
}
