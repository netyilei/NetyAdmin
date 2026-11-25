// port.go：api.Governance / api.Stats 的模块侧实现（domain/application →
// api DTO 转换）。
package gw

import (
	"context"

	"github.com/netyilei/netyadmin/internal/modules/gw/api"
	"github.com/netyilei/netyadmin/internal/modules/gw/application"
	"github.com/netyilei/netyadmin/internal/modules/gw/domain"
)

// governancePort 实现 api.Governance（注册名 gw.Governance）。
type governancePort struct{ svc *application.Service }

var _ api.Governance = governancePort{}

// App 适配 Service.AppAuthSnapshot（带授权 scope 列表）。
func (p governancePort) App(ctx context.Context, appKey string) (api.App, error) {
	a, scopes, err := p.svc.AppAuthSnapshot(ctx, appKey)
	if err != nil {
		return api.App{}, err
	}
	return api.App{
		ID: a.ID, AppKey: a.AppKey, Name: a.Name, Status: int8(a.Status),
		RateLimitEnabled: a.RateLimitEnabled, Scopes: scopes,
	}, nil
}

// Apps 适配 Service.ListApps（AppView → App）。
func (p governancePort) Apps(ctx context.Context, offset, limit int) ([]api.App, int64, error) {
	list, total, err := p.svc.ListApps(ctx, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	out := make([]api.App, 0, len(list))
	for _, a := range list {
		out = append(out, api.App{
			ID: a.ID, AppKey: a.AppKey, Name: a.Name, Status: int8(a.Status),
			RateLimitEnabled: a.RateLimitEnabled, Scopes: a.Scopes,
		})
	}
	return out, total, nil
}

// APIs 适配 Service.ListApis。
func (p governancePort) APIs(ctx context.Context) ([]api.API, error) {
	list, err := p.svc.ListApis(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.API, 0, len(list))
	for _, a := range list {
		out = append(out, toAPI(a))
	}
	return out, nil
}

func toAPI(a domain.Api) api.API {
	return api.API{ID: a.ID, Method: a.Method, Path: a.Path, Name: a.Name, Group: a.Grp, Status: int8(a.Status)}
}

// statsPort 实现 api.Stats（注册名 gw.Stats）。
type statsPort struct{ svc *application.StatsService }

var _ api.Stats = statsPort{}

// Overview 适配 StatsService.Overview。
func (p statsPort) Overview(hours int) api.Overview {
	o := p.svc.Overview(hours)
	return api.Overview{
		Hours: o.Hours, Total: o.Total, Errors: o.Errors, ErrorRate: o.ErrorRate,
		AvgLatency: o.AvgLatency, P95: o.P95, Apps: o.Apps, APIs: o.APIs,
	}
}

// Trend 适配 StatsService.Trend。
func (p statsPort) Trend(hours int) []api.Trend {
	list := p.svc.Trend(hours)
	out := make([]api.Trend, 0, len(list))
	for _, t := range list {
		out = append(out, api.Trend{Hour: t.Hour, Total: t.Total, Success: t.Success, Fail: t.Fail, AvgLatency: t.AvgLatency})
	}
	return out
}

// Top 适配 StatsService.Top。
func (p statsPort) Top(kind string, limit, hours int) []api.Top {
	list := p.svc.Top(kind, limit, hours)
	out := make([]api.Top, 0, len(list))
	for _, t := range list {
		out = append(out, api.Top{Key: t.Key, Calls: t.Calls, Errors: t.Errors})
	}
	return out
}

// StatusDist 适配 StatsService.StatusDist。
func (p statsPort) StatusDist(hours int) []api.StatusCount {
	list := p.svc.StatusDist(hours)
	out := make([]api.StatusCount, 0, len(list))
	for _, s := range list {
		out = append(out, api.StatusCount{Code: s.Code, Calls: s.Calls})
	}
	return out
}

// Latency 适配 StatsService.Latency。
func (p statsPort) Latency(hours int) api.Latency {
	l := p.svc.Latency(hours)
	return api.Latency{Avg: l.Avg, P50: l.P50, P95: l.P95}
}
