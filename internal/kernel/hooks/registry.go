// registry.go：统一钩子注册表——On 注册 / RunStage 驱动 / Hooks 诊断。
// 语义（§7.5）：优先级降序 + 注册顺序（确定性）；per-hook 超时（默认随阶段，
// 覆盖无"不限时"）；panic 隔离按阶段策略；并发安全（On 可并发，Run 期注册
// 约定冻结——AfterBootstrap 即冻结点）。
package hooks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// HookFunc 是钩子函数：payload 按阶段约定（request 段传 *gin.Context 由
// transport 层注入；lifecycle 段 nil；ws/job 段传各自上下文对象——/ 落地时定契约）。
type HookFunc func(ctx context.Context, payload any) error

// HookOption 定制单个钩子。
type HookOption func(*hook)

// WithTimeout 覆盖该钩子的超时；d<=0 回退阶段默认（无"不限时"语义）。
func WithTimeout(d time.Duration) HookOption {
	return func(h *hook) {
		if d > 0 {
			h.timeout = d
		}
	}
}

type hook struct {
	name     string
	priority int
	fn       HookFunc
	timeout  time.Duration // <=0 = 阶段默认
}

// Registry 是统一钩子注册表（四类共用一套优先级/超时/panic 语义）。
type Registry struct {
	mu            sync.RWMutex
	hooks         map[Stage][]*hook
	stageTimeouts map[Stage]time.Duration // 阶段默认覆盖
	logger        *slog.Logger
}

// New 创建（logger 由调用方注入；nil 用 slog.Default）。
func New(logger *slog.Logger) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	r := &Registry{
		hooks:         map[Stage][]*hook{},
		stageTimeouts: map[Stage]time.Duration{},
		logger:        logger,
	}
	for s := range stageMetas {
		r.stageTimeouts[s] = stageMetas[s].timeout
	}
	return r
}

// SetStageTimeout 覆盖阶段默认超时（如 BeforeBootstrap 的 DB 连接段）。
func (r *Registry) SetStageTimeout(stage Stage, d time.Duration) {
	if _, err := meta(stage); err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stageTimeouts[stage] = d
}

// On 注册钩子（同阶段 priority 降序 + 注册序；nil fn 忽略）。
func (r *Registry) On(stage Stage, name string, priority int, fn HookFunc, opts ...HookOption) {
	if _, err := meta(stage); err != nil {
		r.logger.Error("hooks: On 拒绝未登记阶段", "stage", string(stage), "err", err)
		return
	}
	if fn == nil {
		return
	}
	h := &hook{name: name, priority: priority, fn: fn, timeout: 0}
	for _, o := range opts {
		o(h)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks[stage] = append(r.hooks[stage], h)
}

// Unregister 注销钩子（依赖下线/测试）。
func (r *Registry) Unregister(stage Stage, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	hs := r.hooks[stage]
	for i, h := range hs {
		if h.name == name {
			r.hooks[stage] = append(hs[:i], hs[i+1:]...)
			return
		}
	}
}

// Hooks 返回各阶段已注册钩子名（诊断数据面， 消费）。
func (r *Registry) Hooks() map[Stage][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := map[Stage][]string{}
	for s, hs := range r.hooks {
		names := make([]string, 0, len(hs))
		for _, h := range hs {
			names = append(names, h.name)
		}
		out[s] = names
	}
	return out
}

// sortedHooks 优先级降序（稳定）副本。
func (r *Registry) sortedHooks(stage Stage) []*hook {
	r.mu.RLock()
	defer r.mu.RUnlock()
	hs := make([]*hook, len(r.hooks[stage]))
	copy(hs, r.hooks[stage])
	sort.SliceStable(hs, func(i, j int) bool { return hs[i].priority > hs[j].priority })
	return hs
}

// RunStage 驱动一个阶段的钩子链（payload 透传给每个钩子）。
//
//	fail-fast 阶段：首错中止并返回（带 stage/hook 名）；
//	aggregate 阶段：全部执行，错误聚合返回；
//	async 阶段：调用方应使用 RunStageAsync。
func (r *Registry) RunStage(ctx context.Context, stage Stage, payload any) error {
	m, err := meta(stage)
	if err != nil {
		return err
	}
	hs := r.sortedHooks(stage)
	stageT := r.stageTimeout(stage)

	if m.async {
		r.RunStageAsync(ctx, stage, payload)
		return nil
	}
	var errs []error
	for _, h := range hs {
		if herr := r.runHook(ctx, stage, h, payload, stageT); herr != nil {
			wrapped := fmt.Errorf("hooks: %s/%s: %w", stage, h.name, herr)
			if m.policy == policyFailFast {
				return wrapped
			}
			errs = append(errs, wrapped)
		}
	}
	return errors.Join(errs...)
}

// RunStageAsync 异步触发（ResponseSent/RequestError 等 async 阶段；
// fire-and-forget：panic/错误只记日志，不回传）。
func (r *Registry) RunStageAsync(ctx context.Context, stage Stage, payload any) {
	m, err := meta(stage)
	if err != nil {
		return
	}
	if !m.async {
		r.logger.Warn("hooks: 同步阶段误用 RunStageAsync，转同步执行", "stage", string(stage))
		_ = r.RunStage(ctx, stage, payload)
		return
	}
	ctx = context.WithoutCancel(ctx) // fire-and-forget：不随调用方 ctx 取消
	hs := r.sortedHooks(stage)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				r.logger.Error("hooks: async stage panic", "stage", string(stage), "panic", fmt.Sprint(p))
			}
		}()
		stageT := r.stageTimeout(stage)
		for _, h := range hs {
			if herr := r.runHook(ctx, stage, h, payload, stageT); herr != nil {
				r.logger.Warn("hooks: async hook failed", "stage", string(stage), "hook", h.name, "err", herr)
			}
		}
	}()
}

// runHook 单钩子：超时 + panic 隔离。
func (r *Registry) runHook(ctx context.Context, stage Stage, h *hook, payload any, stageDefault time.Duration) (err error) {
	timeout := h.timeout
	if timeout <= 0 {
		timeout = stageDefault
	}
	hctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
			r.logger.Error("hooks: hook panic", "stage", string(stage), "hook", h.name, "panic", fmt.Sprint(p))
		}
	}()
	return h.fn(hctx, payload)
}

// stageTimeout 锁内读取阶段默认超时。
func (r *Registry) stageTimeout(stage Stage) time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stageTimeouts[stage]
}
