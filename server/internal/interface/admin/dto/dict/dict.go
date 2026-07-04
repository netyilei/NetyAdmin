package dict

// CreateDictTypeReq 创建字典类型请求
type CreateDictTypeReq struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Status      string `json:"status" binding:"required,oneof=0 1"`
	Description string `json:"description"`
}

// UpdateDictTypeReq 更新字典类型请求
// Code 为业务唯一标识，创建后不可变更（基座设计原则，与 RBAC role code、菜单 code 一致）。
type UpdateDictTypeReq struct {
	ID          uint   `json:"id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Status      string `json:"status" binding:"required,oneof=0 1"`
	Description string `json:"description"`
}

// CreateDictDataReq 创建字典数据请求
type CreateDictDataReq struct {
	DictCode string `json:"dictCode" binding:"required"`
	Label    string `json:"label" binding:"required"`
	Value    string `json:"value" binding:"required"`
	TagType  string `json:"tagType"`
	OrderBy  int    `json:"orderBy"`
	Status   string `json:"status" binding:"required,oneof=0 1"`
	Remark   string `json:"remark"`
}

// UpdateDictDataReq 更新字典数据请求
// DictCode 为业务关联标识，创建后不可变更（字典数据归属固定的字典类型，不可跨类型迁移）。
type UpdateDictDataReq struct {
	ID      uint   `json:"id" binding:"required"`
	Label   string `json:"label" binding:"required"`
	Value   string `json:"value" binding:"required"`
	TagType string `json:"tagType"`
	OrderBy int    `json:"orderBy"`
	Status  string `json:"status" binding:"required,oneof=0 1"`
	Remark  string `json:"remark"`
}
