package open_platform

type OpenApiQuery struct {
	Current int    `form:"current"`
	Size    int    `form:"size"`
	Method  string `form:"method"`
	Path    string `form:"path"`
	Name    string `form:"name"`
	Group   string `form:"group"`
	Status  *int   `form:"status"`
}

type CreateOpenApiReq struct {
	Method      string `json:"method" binding:"required,oneof=GET POST PUT DELETE PATCH"`
	Path        string `json:"path" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Group       string `json:"group"`
	Description string `json:"description"`
	Status      int    `json:"status" binding:"oneof=0 1"`
}

type UpdateOpenApiReq struct {
	ID          uint64 `json:"id" binding:"required"`
	Method      string `json:"method" binding:"required,oneof=GET POST PUT DELETE PATCH"`
	Path        string `json:"path" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Group       string `json:"group"`
	Description string `json:"description"`
	Status      int    `json:"status" binding:"oneof=0 1"`
}

type UpdateScopeApisReq struct {
	ScopeID uint64   `json:"scopeId" binding:"required"`
	ApiIDs  []uint64 `json:"apiIds"`
}

type GroupedOpenApi struct {
	Group string         `json:"group"`
	Apis  []*OpenApiItem `json:"apis"`
}

type OpenApiItem struct {
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Method string `json:"method"`
	Path   string `json:"path"`
}
