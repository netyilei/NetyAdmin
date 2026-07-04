package open_platform

type OpenLogQuery struct {
	Current    int    `form:"current"`
	Size       int    `form:"size"`
	AppID      string `form:"appId"`
	AppKey     string `form:"appKey"`
	ApiPath    string `form:"apiPath"`
	StatusCode *int   `form:"statusCode"`
	StartTime  string `form:"startTime"`
	EndTime    string `form:"endTime"`
}

// RecordOpenLogReq 用于 OpenLogService.Record 内部 API
// 调用方为 middleware（OpenPlatformAuth 中间件），非 handler
type RecordOpenLogReq struct {
	AppID         string
	AppKey        string
	ApiPath       string
	ApiMethod     string
	ClientIP      string
	StatusCode    int
	Latency       int64
	RequestHeader string
	RequestBody   string
	ResponseBody  string
	ErrorMsg      string
	// RequestID 由 OpenPlatformAuth 中间件从 ctx 提取并填入（Task 8.5），
	// service 层透传到 entity 后由 LogBus 刷盘写入 sys_open_platform_logs.request_id 列。
	RequestID string
}
