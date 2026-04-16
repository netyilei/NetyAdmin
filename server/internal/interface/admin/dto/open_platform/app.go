package open_platform

type AppQuery struct {
	Current int    `form:"current"`
	Size    int    `form:"size"`
	Name    string `form:"name"`
	AppKey  string `form:"appKey"`
	Type    *int   `form:"type"`
	Status  *int   `form:"status"`
}

type CreateAppReq struct {
	Name       string   `json:"name" binding:"required"`
	Type       int      `json:"type" binding:"required,oneof=1 2"`
	Status     int      `json:"status" binding:"oneof=0 1"`
	IPStrategy int      `json:"ipStrategy" binding:"oneof=1 2"`
	Remark     string   `json:"remark"`
	Scopes     []string `json:"scopes"`
}

type UpdateAppReq struct {
	ID         string   `json:"id" binding:"required"`
	Name       string   `json:"name" binding:"required"`
	Type       int      `json:"type" binding:"required,oneof=1 2"`
	Status     int      `json:"status" binding:"oneof=0 1"`
	IPStrategy int      `json:"ipStrategy" binding:"oneof=1 2"`
	Remark     string   `json:"remark"`
	Scopes     []string `json:"scopes"`
}

type ResetSecretReq struct {
	ID string `json:"id" binding:"required"`
}
