package log

type OperationVO struct {
	ID         uint   `json:"id"`
	UserID     uint   `json:"userId"`
	Username   string `json:"username"`
	Action     string `json:"action"`
	Resource   string `json:"resource"`
	Detail     string `json:"detail"`
	IP         string `json:"ip"`
	UserAgent  string `json:"userAgent"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	RequestID  string `json:"requestId"`
	StatusCode int    `json:"statusCode"`
	CostTime   int64  `json:"costTime"`
	CreatedAt  string `json:"createdAt"`
}

type OperationListVO struct {
	Records []OperationVO `json:"records"`
	Current int           `json:"current"`
	Size    int           `json:"size"`
	Total   int64         `json:"total"`
}
