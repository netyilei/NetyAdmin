// Package api 是 gw 模块的跨模块端口（开放平台治理与调用统计只读面；
// 禁止 import application/domain）。
package api

import "context"

// Governance 是治理只读端口（注册名 `gw.Governance`）：
//
//	gov, err := module.ResolveAs[gwapi.Governance](registry.Services, "gw.Governance")
//	app, err := gov.App(ctx, appKey)
//
// 应用/scope/API 的写面属管理端职责，不经业务模块。
type Governance interface {
	// App 按 AppKey 取应用（含授权 scope 列表）。
	App(ctx context.Context, appKey string) (App, error)
	// Apps 分页列出应用。
	Apps(ctx context.Context, offset, limit int) ([]App, int64, error)
	// APIs 返回已登记网关 API 目录。
	APIs(ctx context.Context) ([]API, error)
}

// Stats 是调用统计端口（注册名 `gw.Stats`；时间窗按小时）。
type Stats interface {
	// Overview 总览（请求量/错误率/延迟/应用与 API 计数）。
	Overview(hours int) Overview
	// Trend 逐小时趋势。
	Trend(hours int) []Trend
	// Top 排行（kind：app | api）。
	Top(kind string, limit, hours int) []Top
	// StatusDist HTTP 状态分布。
	StatusDist(hours int) []StatusCount
	// Latency 延迟分位摘要（毫秒）。
	Latency(hours int) Latency
}

// App 是开放平台应用投影（secret 永不出域）。
type App struct {
	ID               string
	AppKey           string
	Name             string
	Status           int8 // 1=enabled / 2=disabled
	RateLimitEnabled bool
	Scopes           []string
}

// API 是网关 API 登记项。
type API struct {
	ID     string
	Method string
	Path   string
	Name   string
	Group  string
	Status int8
}

// Overview 是统计总览。
type Overview struct {
	Hours      int
	Total      int64
	Errors     int64
	ErrorRate  float64
	AvgLatency float64
	P95        float64
	Apps       int
	APIs       int
}

// Trend 是逐小时趋势项。
type Trend struct {
	Hour       int64
	Total      int64
	Success    int64
	Fail       int64
	AvgLatency float64
}

// Top 是排行项。
type Top struct {
	Key    string
	Calls  int64
	Errors int64
}

// StatusCount 是状态分布项。
type StatusCount struct {
	Code  int
	Calls int64
}

// Latency 是延迟分位摘要（毫秒）。
type Latency struct {
	Avg float64
	P50 float64
	P95 float64
}
