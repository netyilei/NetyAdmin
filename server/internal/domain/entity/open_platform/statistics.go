package open_platform

// Statistics result entities returned by Repository, serialized directly as HTTP responses.
// Latency fields are in milliseconds (ns / 1_000_000).

type TrendItem struct {
	Date         string  `json:"date"`
	TotalCalls   int64   `json:"totalCalls"`
	SuccessCalls int64   `json:"successCalls"`
	FailCalls    int64   `json:"failCalls"`
	AvgLatency   float64 `json:"avgLatency"`
}

type AppStatItem struct {
	AppID   string  `json:"appId"`
	AppName string  `json:"appName"`
	Calls   int64   `json:"calls"`
	Percent float64 `json:"percent"`
}

type ApiStatItem struct {
	ApiPath   string  `json:"apiPath"`
	ApiMethod string  `json:"apiMethod"`
	Calls     int64   `json:"calls"`
	Percent   float64 `json:"percent"`
}

type StatusDistItem struct {
	StatusCode int     `json:"statusCode"`
	Calls      int64   `json:"calls"`
	Percent    float64 `json:"percent"`
}

type LatencyStats struct {
	AvgLatency float64 `json:"avgLatency"`
	P50        float64 `json:"p50"`
	P95        float64 `json:"p95"`
	P99        float64 `json:"p99"`
	MaxLatency float64 `json:"maxLatency"`
}

type OverviewStats struct {
	TotalCalls   int64   `json:"totalCalls"`
	SuccessCalls int64   `json:"successCalls"`
	FailCalls    int64   `json:"failCalls"`
	AvgLatency   float64 `json:"avgLatency"`
	AppCount     int64   `json:"appCount"`
	APICount     int64   `json:"apiCount"`
}
