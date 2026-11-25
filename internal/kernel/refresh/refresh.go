// Package refresh 提供 Broadcast 域订阅者的两层自愈原语（
// at-most-once 的消费侧补偿——不改 Bus 投递机制，ack/重投归机制级升级）：
//   - Retry：订阅 handler 装载失败即进程内短退避重试（3 次，100ms0ms0ms）
//     ——DB 瞬断/连接重建窗口内当场收敛，不把「一次抖动」放大成「节点陈旧
//     直至下一变更事件」；
//   - Job：周期对账盲 Reload（全量替换幂等、无版本依赖）——兜住 Bus 丢消息
//     （Pub/Sub 掉线/队列满丢弃）与重试仍失败的持久陈旧，5min 收敛上界。
//
// 三面消费方：gw（apps/rules.changed）、configcenter（config.changed）、
// rbac（policy.changed）。选型=盲 Reload（方案 a）：三面均无对订阅方可见的
// 统一版本号（仅 configcenter 有局部 version，gw/rbac 无；且 version 对账仍需
// 额外记录「已装载版本」，反而多一份状态），数据量小（应用登记/配置/Casbin
// p-g 行），周期全量读成本可接受——最简实现优先（G1-15）。
package refresh

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/bus"
	"github.com/netyilei/netyadmin/internal/kernel/scheduler"
)

// 订阅侧退避重试参数（具名；G1-08）。3 次重试退避 100ms0ms0ms，
// 总窗≈1.3s——小于 Bus 默认 handler 超时 2s，重试不会把 handler 拖成超时。
const (
	RetryAttempts  = 4                      // 首发 + 3 次重试
	RetryBaseDelay = 100 * time.Millisecond // 首次重试退避
	RetryFactor    = 3                      // 100ms → 300ms → 900ms
)

// Retry 至多调用 fn RetryAttempts 次；失败按 RetryBaseDelay×RetryFactor^(n-1)
// 退避后重试。ctx 取消/超时立即返回最后一次错误（Bus handler 超时保护面）。
func Retry(ctx context.Context, fn func(context.Context) error) error {
	delay := RetryBaseDelay
	var err error
	for attempt := 1; attempt <= RetryAttempts; attempt++ {
		if err = fn(ctx); err == nil {
			return nil
		}
		if attempt == RetryAttempts || ctx.Err() != nil {
			break
		}
		if !sleepCtx(ctx, delay) {
			break
		}
		delay *= RetryFactor
	}
	return err
}

// sleepCtx 可被 ctx 取消的退避等待；返回 false=ctx 已取消。
func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// Subscribe 在 topic 上接线 reload 订阅（三面共用：gw/configcenter/rbac——
// 原先逐字重复的 Subscribe→Retry→Error→回错收口于此）。handler 内先 Retry
// （进程内短退避），穷尽后 Error 告警（node stale until reconcile）并回错给
// Bus 计数（陈旧由 Job 周期对账兜底）；订阅注册失败返回包装错误。name 是
// 错误/日志前缀（如 "gw: app snapshot reload"）。
func Subscribe(b *bus.Bus, topic, name string, reload func(context.Context) error, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	if _, err := b.Subscribe(topic, func(ctx context.Context, _ bus.Message) error {
		rerr := Retry(ctx, reload)
		if rerr != nil {
			logger.Error(name+" failed after retries (node stale until reconcile)",
				"err", rerr, "attempts", RetryAttempts)
			return rerr
		}
		return nil
	}); err != nil {
		return fmt.Errorf("%s: subscribe: %w", name, err)
	}
	return nil
}

// 周期对账参数（具名）。5min=陈旧上界；1min 超时防 DB 挂起占住单飞行槽；
// 调度层短退避重试 2 次覆盖周期内的瞬断，穷尽留待下一周期。
const (
	ReconcileInterval = 5 * time.Minute
	reconcileTimeout  = 1 * time.Minute
	reconcileRetries  = 2
)

// Job 是周期对账盲 Reload 的 scheduler.Job 实现：三面共用一份（Job 语义/
// 周期/超时/执行域单一事实源），模块只提供名称与 reload 函数。
type Job struct {
	mu     sync.RWMutex
	name   string
	reload func(context.Context) error
}

// NewJob 创建对账 Job（模块 Init 构造并登记；Manifest jobs 双源同字面）。
func NewJob(name string) *Job {
	return &Job{name: name}
}

// SetReload 接线 reload 函数（模块 AttachDB 阶段；未接线=Run 空操作——模块
// 未接 DB 的装配/单测无对账面，非错误）。
func (j *Job) SetReload(fn func(context.Context) error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.reload = fn
}

// Name 实现 scheduler.Job。
func (j *Job) Name() string { return j.name }

// Spec 实现 scheduler.Job：interval 5min、node 域——快照是节点本地面，每节点
// 必须各自收敛（cluster 域只跑 leader 是错误语义）。
func (j *Job) Spec() scheduler.Spec {
	return scheduler.Spec{
		Trigger:  scheduler.TriggerInterval,
		Duration: ReconcileInterval,
		Timeout:  reconcileTimeout,
		Retries:  reconcileRetries,
		Scope:    scheduler.ScopeNode,
	}
}

// Run 实现 scheduler.Job：盲 Reload（全量替换幂等，无副作用——写路径语义
// 不因周期重载改变）。
func (j *Job) Run(ctx context.Context, _ scheduler.Execution) error {
	j.mu.RLock()
	fn := j.reload
	j.mu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn(ctx)
}
