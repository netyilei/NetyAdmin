// diagnostics.go：诊断注册表——各组件注册快照提供者，
// 汇聚为 /admin/v1/system/diagnostics/* 数据面（交付）。
package observability

import (
	"context"
	"sync"
)

// Provider 返回该组件的诊断快照（可 JSON 序列化）。
type Provider func(ctx context.Context) any

// Registry 是诊断提供者注册表（模块/组件在 Init 时注册）。
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewDiagnostics 创建诊断注册表。
func NewDiagnostics() *Registry {
	return &Registry{providers: map[string]Provider{}}
}

// Register 注册/覆盖诊断提供者（name 即诊断端点子路径标识）。
func (r *Registry) Register(name string, p Provider) {
	if name == "" || p == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = p
}

// Names 已注册诊断名（排序）。
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for n := range r.providers {
		names = append(names, n)
	}
	return names
}

// Snapshot 汇聚全部诊断快照（key=诊断名）。
func (r *Registry) Snapshot(ctx context.Context) map[string]any {
	r.mu.RLock()
	providers := make(map[string]Provider, len(r.providers))
	for n, p := range r.providers {
		providers[n] = p
	}
	r.mu.RUnlock()
	out := make(map[string]any, len(providers))
	for n, p := range providers {
		out[n] = p(ctx)
	}
	return out
}
