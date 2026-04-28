package open_platform

type AppQuery struct {
	Current int    `form:"current"`
	Size    int    `form:"size"`
	Name    string `form:"name"`
	AppKey  string `form:"appKey"`
	Status  *int   `form:"status"`
}

type CreateAppReq struct {
	Name            string   `json:"name" binding:"required"`
	Status          int      `json:"status" binding:"oneof=0 1"`
	IPFilterEnabled bool     `json:"ipFilterEnabled"`
	Remark          string   `json:"remark"`
	QuotaConfig     string   `json:"quotaConfig"`
	StorageID       uint     `json:"storageId"`
	Scopes          []string `json:"scopes"`
}

type UpdateAppReq struct {
	ID              string   `json:"id" binding:"required"`
	Name            string   `json:"name" binding:"required"`
	Status          int      `json:"status" binding:"oneof=0 1"`
	IPFilterEnabled bool     `json:"ipFilterEnabled"`
	Remark          string   `json:"remark"`
	QuotaConfig     string   `json:"quotaConfig"`
	StorageID       uint     `json:"storageId"`
	Scopes          []string `json:"scopes"`
}

type ResetSecretReq struct {
	ID string `json:"id" binding:"required"`
}

type LinkIPRulesReq struct {
	ID      string `json:"id" binding:"required"`
	RuleIDs []uint `json:"ruleIds"`
}
