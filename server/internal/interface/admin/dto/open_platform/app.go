package open_platform

type AppQuery struct {
	Current int    `form:"current"`
	Size    int    `form:"size"`
	Name    string `form:"name"`
	AppKey  string `form:"appKey"`
	Status  *int   `form:"status"`
}

type CreateAppReq struct {
	Name              string   `json:"name" binding:"required"`
	Status            int      `json:"status" binding:"oneof=0 1"`
	IPFilterEnabled   bool     `json:"ipFilterEnabled"`
	RateLimitEnabled  bool     `json:"rateLimitEnabled"`
	Remark            string   `json:"remark"`
	QuotaConfig       string   `json:"quotaConfig"`
	CacheTTL          int      `json:"cacheTTL"`
	StorageID         uint     `json:"storageId"`
	Scopes            []string `json:"scopes"`
}

// UpdateAppReq 应用更新请求。
// AppKey 为业务唯一标识，创建后不可变更（基座设计原则，与 RBAC role code、菜单 code、字典 Code 一致），
// 因此本 DTO 不含 AppKey 字段；AppKey 初始值由 CreateApp 生成（ULID），后续只读。
// AppSecret 同样不在本接口修改，由独立的 ResetAppSecret 方法处理轮换。
type UpdateAppReq struct {
	ID                string   `json:"id" binding:"required"`
	Name              string   `json:"name" binding:"required"`
	Status            int      `json:"status" binding:"oneof=0 1"`
	IPFilterEnabled   bool     `json:"ipFilterEnabled"`
	RateLimitEnabled  bool     `json:"rateLimitEnabled"`
	Remark            string   `json:"remark"`
	QuotaConfig       string   `json:"quotaConfig"`
	CacheTTL          int      `json:"cacheTTL"`
	StorageID         uint     `json:"storageId"`
	Scopes            []string `json:"scopes"`
}

type ResetSecretReq struct {
	ID string `json:"id" binding:"required"`
}

type LinkIPRulesReq struct {
	ID      string `json:"id" binding:"required"`
	RuleIDs []uint `json:"ruleIds"`
}

type CreateScopeGroupReq struct {
	Code        string `json:"code" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Status      int    `json:"status" binding:"oneof=0 1"`
}

type UpdateScopeGroupReq struct {
	ID          uint64 `json:"id" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Status      int    `json:"status" binding:"oneof=0 1"`
}
