package system

import "netyadmin/internal/interface/admin/dto"

type APIQuery struct {
	dto.PageQuery
	Name   string `form:"name"`
	Path   string `form:"path"`
	Method string `form:"method"`
	Group  string `form:"group"`
}

type CreateAPIReq struct {
	Name   string `json:"name" binding:"required"`
	Path   string `json:"path" binding:"required"`
	Method string `json:"method" binding:"required"`
	Group  string `json:"group" binding:"required"`
	Desc   string `json:"desc"`
}

type UpdateAPIReq struct {
	ID     uint   `json:"id" binding:"required"`
	Name   string `json:"name" binding:"required"`
	Path   string `json:"path" binding:"required"`
	Method string `json:"method" binding:"required"`
	Group  string `json:"group"`
	Desc   string `json:"desc"`
}

type APIVO struct {
	ID          uint   `json:"id"`
	MenuID      uint   `json:"menuId"`
	MenuName    string `json:"menuName"`
	Name        string `json:"name"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"desc"`
	Auth        string `json:"auth"`
	CreatedAt   string `json:"createTime"`
	UpdatedAt   string `json:"updateTime"`
}
