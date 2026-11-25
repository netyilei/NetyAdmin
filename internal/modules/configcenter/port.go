// port.go：api.Provider 的模块侧实现（domain.Config → api.Entry 转换）。
package configcenter

import (
	"context"

	"github.com/netyilei/netyadmin/internal/modules/configcenter/api"
	"github.com/netyilei/netyadmin/internal/modules/configcenter/application"
	"github.com/netyilei/netyadmin/internal/modules/configcenter/domain"
)

// providerPort 实现 api.Provider（注册名 configcenter.Provider）。
type providerPort struct{ svc *application.Service }

var _ api.Provider = providerPort{}

// Get 适配 Service.Get。
func (p providerPort) Get(ctx context.Context, group, key string) (api.Entry, error) {
	c, err := p.svc.Get(ctx, group, key)
	if err != nil {
		return api.Entry{}, err
	}
	return toEntry(c), nil
}

// All 适配 Service.All（全量快照）。
func (p providerPort) All(ctx context.Context) ([]api.Entry, error) {
	list, err := p.svc.All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.Entry, 0, len(list))
	for _, c := range list {
		out = append(out, toEntry(c))
	}
	return out, nil
}

// toEntry 转换配置实体为端口 DTO。
func toEntry(c domain.Config) api.Entry {
	return api.Entry{Group: c.GroupName, Key: c.ConfigKey, Value: c.ConfigValue, IsPublic: c.IsPublic}
}

// watcherPort 实现 api.Watcher（注册名 configcenter.Watcher）。
type watcherPort struct{ w *application.Watcher }

var _ api.Watcher = watcherPort{}

// OnChange 注册变更回调（透传 Watcher）。
func (p watcherPort) OnChange(fn func()) { p.w.OnChange(fn) }

// Snapshot 转换快照（key = "group:key"）。
func (p watcherPort) Snapshot() map[string]api.Entry {
	snap := p.w.Snapshot()
	out := make(map[string]api.Entry, len(snap))
	for k, c := range snap {
		out[k] = toEntry(c)
	}
	return out
}

// Loaded 透传首次装载状态。
func (p watcherPort) Loaded() bool { return p.w.Loaded() }
