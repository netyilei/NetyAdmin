package system

type CreateButtonReq struct {
	MenuID uint   `json:"menuId" binding:"required"`
	Name   string `json:"name" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

type UpdateButtonReq struct {
	ID     uint   `json:"id" binding:"required"`
	MenuID uint   `json:"menuId" binding:"required"`
	Name   string `json:"name" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

type ButtonVO struct {
	ID        uint   `json:"id"`
	MenuID    uint   `json:"menuId"`
	Code      string `json:"code"`
	Label     string `json:"label"`
	CreatedAt string `json:"createTime"`
	UpdatedAt string `json:"updateTime"`
}

type ButtonQuery struct {
	Label   string `form:"label"`
	Code    string `form:"code"`
	MenuID  *uint  `form:"menuId"`
	Current int    `form:"current"`
	Size    int    `form:"size"`
}
