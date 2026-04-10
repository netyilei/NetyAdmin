package content

type ContentCategoryDTO struct {
	ID          uint   `json:"id"`
	ParentID    uint   `json:"parentId"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Icon        string `json:"icon"`
	Sort        int    `json:"sort"`
	ContentType string `json:"contentType"`
	Status      string `json:"status"`
	Remark      string `json:"remark"`
	CreatedBy   uint   `json:"createdBy"`
	UpdatedBy   uint   `json:"updatedBy"`
}

type CreateContentCategoryDTO struct {
	ParentID    uint   `json:"parentId" form:"parentId" binding:"omitempty"`
	Name        string `json:"name" form:"name" binding:"required,max=50"`
	Code        string `json:"code" form:"code" binding:"omitempty,max=50"`
	Icon        string `json:"icon" form:"icon" binding:"omitempty,max=100"`
	Sort        int    `json:"sort" form:"sort" binding:"omitempty"`
	ContentType string `json:"contentType" form:"contentType" binding:"omitempty,oneof=plaintext richtext"`
	Status      string `json:"status" form:"status" binding:"omitempty,oneof=0 1"`
	Remark      string `json:"remark" form:"remark" binding:"omitempty"`
}

type UpdateContentCategoryDTO struct {
	ParentID    uint   `json:"parentId" form:"parentId" binding:"omitempty"`
	Name        string `json:"name" form:"name" binding:"omitempty,max=50"`
	Code        string `json:"code" form:"code" binding:"omitempty,max=50"`
	Icon        string `json:"icon" form:"icon" binding:"omitempty,max=100"`
	Sort        int    `json:"sort" form:"sort" binding:"omitempty"`
	ContentType string `json:"contentType" form:"contentType" binding:"omitempty,oneof=plaintext richtext"`
	Status      string `json:"status" form:"status" binding:"omitempty,oneof=0 1"`
	Remark      string `json:"remark" form:"remark" binding:"omitempty"`
}

type ContentCategoryTreeDTO struct {
	ID          uint                     `json:"id"`
	ParentID    uint                     `json:"parentId"`
	Name        string                   `json:"name"`
	Code        string                   `json:"code"`
	Icon        string                   `json:"icon"`
	Sort        int                      `json:"sort"`
	ContentType string                   `json:"contentType"`
	Status      string                   `json:"status"`
	Children    []ContentCategoryTreeDTO `json:"children"`
}

type ContentCategoryListQueryDTO struct {
	Current     int    `form:"current"`
	Size        int    `form:"size"`
	Name        string `form:"name"`
	Code        string `form:"code"`
	ContentType string `form:"contentType"`
	Status      string `form:"status"`
}
