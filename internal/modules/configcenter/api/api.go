// Package api 是 configcenter 模块的跨模块端口（下游模块 import 本包即可读
// 动态配置；禁止 import application/domain）。
package api

import "context"

// Provider 是动态配置读取端口（注册名 `configcenter.Provider`）：
//
//	cfg, err := module.ResolveAs[ccapi.Provider](registry.Services, "configcenter.Provider")
//	entry, err := cfg.Get(ctx, "silentorder", "sms.daily_limit")
//	limit := entry.Value
//
// 写面（创建/更新/回滚）属管理端职责，不经业务模块。
type Provider interface {
	// Get 读取单个配置（不存在返回错误）。
	Get(ctx context.Context, group, key string) (Entry, error)
	// All 返回全量配置快照（含系统键；下游一般只需 Get）。
	All(ctx context.Context) ([]Entry, error)
}

// Watcher 是配置变更订阅端口（注册名 `configcenter.Watcher`）：
//
//	w, err := module.ResolveAs[ccapi.Watcher](registry.Services, "configcenter.Watcher")
//	w.OnChange(func() { reloadMyCache(w.Snapshot()) })
//
// OnChange 在热更广播/启动装载后回调（回调内禁止阻塞——只做标记/刷新）。
type Watcher interface {
	// OnChange 注册配置变更回调（多次注册=多回调）。
	OnChange(fn func())
	// Snapshot 返回当前配置快照（key = "group:key"）。
	Snapshot() map[string]Entry
	// Loaded 表示首次装载是否完成（装配早期为 false）。
	Loaded() bool
}

// Entry 是配置项（值以字符串承载，按业务自行解析）。
type Entry struct {
	Group    string
	Key      string
	Value    string
	IsPublic bool
}
