package content

type ContentBannerGroupDTO struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	Code            string `json:"code"`
	Description     string `json:"description"`
	Position        string `json:"position"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	MaxItems        int    `json:"maxItems"`
	AutoPlay        bool   `json:"autoPlay"`
	Interval        int    `json:"interval"`
	Sort            int    `json:"sort"`
	StorageConfigID *uint  `json:"storageConfigId"`
	Status          string `json:"status"`
	Remark          string `json:"remark"`
	CreatedBy       uint   `json:"createdBy"`
	UpdatedBy       uint   `json:"updatedBy"`
}

type CreateContentBannerGroupDTO struct {
	Name            string `json:"name" form:"name" binding:"required,max=100"`
	Code            string `json:"code" form:"code" binding:"required,max=50"`
	Description     string `json:"description" form:"description" binding:"omitempty,max=255"`
	Position        string `json:"position" form:"position" binding:"omitempty,max=50"`
	Width           int    `json:"width" form:"width"`
	Height          int    `json:"height" form:"height"`
	MaxItems        int    `json:"maxItems" form:"maxItems" binding:"omitempty,min=1"`
	AutoPlay        bool   `json:"autoPlay" form:"autoPlay"`
	Interval        int    `json:"interval" form:"interval" binding:"omitempty,min=1000"`
	Sort            int    `json:"sort" form:"sort"`
	StorageConfigID *uint  `json:"storageConfigId" form:"storageConfigId" binding:"omitempty"`
	Status          string `json:"status" form:"status" binding:"omitempty,oneof=0 1"`
	Remark          string `json:"remark" form:"remark"`
}

type ContentBannerGroupListQueryDTO struct {
	Current     int    `form:"current"`
	Size        int    `form:"size"`
	Name        string `form:"name"`
	Code        string `form:"code"`
	Description string `form:"description"`
	Position    string `form:"position"`
	Status      string `form:"status"`
}

type UpdateContentBannerGroupDTO struct {
	Name            *string `json:"name" form:"name" binding:"omitempty,max=100"`
	Code            *string `json:"code" form:"code" binding:"omitempty,min=1,max=50"`
	Description     string  `json:"description" form:"description" binding:"omitempty,max=255"`
	Position        string  `json:"position" form:"position" binding:"omitempty,max=50"`
	Width           int     `json:"width" form:"width"`
	Height          int     `json:"height" form:"height"`
	MaxItems        int     `json:"maxItems" form:"maxItems" binding:"omitempty,min=1"`
	AutoPlay        *bool   `json:"autoPlay" form:"autoPlay"`
	Interval        *int    `json:"interval" form:"interval" binding:"omitempty,min=1000"`
	Sort            int     `json:"sort" form:"sort"`
	StorageConfigID *uint   `json:"storageConfigId" form:"storageConfigId" binding:"omitempty"`
	Status          string  `json:"status" form:"status" binding:"omitempty,oneof=0 1"`
	Remark          string  `json:"remark" form:"remark"`
}
