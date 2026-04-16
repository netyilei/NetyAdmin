package dict

// CreateDictTypeReq 创建字典类型请求
type CreateDictTypeReq struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Status      string `json:"status" binding:"required,oneof=0 1"`
	Description string `json:"description"`
}

// UpdateDictTypeReq 更新字典类型请求
type UpdateDictTypeReq struct {
	ID          uint   `json:"id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
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
type UpdateDictDataReq struct {
	ID       uint   `json:"id" binding:"required"`
	DictCode string `json:"dictCode" binding:"required"`
	Label    string `json:"label" binding:"required"`
	Value    string `json:"value" binding:"required"`
	TagType  string `json:"tagType"`
	OrderBy  int    `json:"orderBy"`
	Status   string `json:"status" binding:"required,oneof=0 1"`
	Remark   string `json:"remark"`
}
