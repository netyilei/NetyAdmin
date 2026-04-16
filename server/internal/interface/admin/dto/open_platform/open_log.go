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
