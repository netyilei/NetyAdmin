// watcher.go：进程内配置快照 + 回调注册表（热更消费面；rbac/ipac reload
// 同模式）。Get 走内存快照（L1 hot_local 语义——小/全局/读多写少/有 version
// 失效）；消费者（log 级别等）经 OnChange 注册回调，cmd 组合根订阅 Bus 后调
// Reload（含本节点——Bus Local 域同步）。快照未装载时 Get 走 repo 直读兜底。
package application

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/netyilei/netyadmin/internal/modules/configcenter/api/keys"
	"github.com/netyilei/netyadmin/internal/modules/configcenter/domain"
)

// Watcher 是动态配置消费面（cmd 经 Expose 注入；跨模块只经此 Port）。
type Watcher struct {
	mu     sync.RWMutex
	repo   domain.Repo // 未装载时直读兜底（Attach 后由 module 注入）
	snap   atomic.Pointer[map[string]domain.Config]
	cbs    []func() // reload 后回调（消费方接线：log 级别等；量级=个位数）
	loaded bool
}

// NewWatcher 创建（module.go AttachDB 注入 repo；Reload 前为空快照）。
func NewWatcher() *Watcher {
	return &Watcher{}
}

// SetRepo 注入存储（与 Service 同 repo 实例）。
func (w *Watcher) SetRepo(repo domain.Repo) {
	w.mu.Lock()
	w.repo = repo
	w.mu.Unlock()
}

// OnChange 注册 reload 回调（消费方接线；幂等性由调用方保证）。
func (w *Watcher) OnChange(fn func()) {
	w.mu.Lock()
	w.cbs = append(w.cbs, fn)
	w.mu.Unlock()
}

// Reload 全量装载快照并触发回调（Bus 订阅回调/启动装载；失败保留旧快照——
// 配置面非安全判定面，fail-open 保留旧值与 rbac fail-closed 语义分域）。
func (w *Watcher) Reload(ctx context.Context) error {
	w.mu.RLock()
	repo := w.repo
	cbs := append([]func(){}, w.cbs...)
	w.mu.RUnlock()
	if repo == nil {
		return nil // 未接线（无 DB 部署/装配前广播）——空操作
	}
	all, err := repo.ListAll(ctx)
	if err != nil {
		return err
	}
	idx := make(map[string]domain.Config, len(all))
	for _, c := range all {
		idx[keys.SnapshotKey(c.GroupName, c.ConfigKey)] = c
	}
	w.snap.Store(&idx)
	w.mu.Lock()
	w.loaded = true
	w.mu.Unlock()
	for _, fn := range cbs {
		fn()
	}
	return nil
}

// Get 读单键（快照命中优先；未装载/未命中走 repo 直读兜底）。
func (w *Watcher) Get(ctx context.Context, group, key string) (domain.Config, bool, error) {
	if m := w.snap.Load(); m != nil {
		if c, ok := (*m)[keys.SnapshotKey(group, key)]; ok {
			return c, true, nil
		}
	}
	w.mu.RLock()
	repo := w.repo
	w.mu.RUnlock()
	if repo == nil {
		return domain.Config{}, false, nil
	}
	c, err := repo.Get(ctx, group, key)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Config{}, false, nil
		}
		return domain.Config{}, false, err
	}
	return c, true, nil
}

// Snapshot 快照全量读（消费方遍历映射用——如 log.group 键→级别接线；nil=未装载）。
func (w *Watcher) Snapshot() map[string]domain.Config {
	if m := w.snap.Load(); m != nil {
		return *m
	}
	return nil
}

// Loaded 快照是否已装载（诊断面）。
func (w *Watcher) Loaded() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.loaded
}
