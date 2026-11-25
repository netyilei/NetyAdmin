// spec.go：Job 模型与调度规格（语义）。
// memory 驱动范围：Trigger=cron/interval/once/delay/manual（event/queue
// 随消费方接入登记）；MissedRun=skip（持久化补跑随 durable 驱动）；
// 幂等责任在 Job 作者（leader 失联切换可能补跑）。
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/robfig/cron/v3"
)

// Trigger 触发方式（§6.2）。
type Trigger string

const (
	TriggerCron     Trigger = "cron"     // Schedule=标准 cron 表达式（Timezone 生效）
	TriggerInterval Trigger = "interval" // Duration=间隔
	TriggerOnce     Trigger = "once"     // Start 后执行一次
	TriggerDelay    Trigger = "delay"    // Duration 延迟后执行一次
	TriggerManual   Trigger = "manual"   // TriggerNow 手动触发（管理面）
)

// Scope 执行域（§6.3）。
type Scope string

const (
	ScopeNode    Scope = "node"    // 每节点各跑一次
	ScopeCluster Scope = "cluster" // 全集群只跑一次（leader 调度）
	ScopeShard   Scope = "shard"   // 按 node 哈希取分片，每分片一次
)

// BackoffType 重试退避类型（§6.2 fixed | exponential + jitter）。
type BackoffType string

const (
	BackoffFixed       BackoffType = "fixed"
	BackoffExponential BackoffType = "exponential"
)

// Spec 调度规格。字段缺省值见 validate。Concurrency=每 Job 并发上限
// （§6.4；单飞行守卫使其缺省 1，AllowOverlap=true 时按上限放行；
// MissedRun=memory 驱动恒 skip（随 durable 驱动）；幂等=Job 作者责任（
// leader 切换补跑），跨节点机制面见 LockEnabled/IdempotencyTTL（默认关）；
// event/queue 触发随消费方接入）。
type Spec struct {
	Trigger      Trigger       // 必填
	Schedule     string        // cron 表达式（TriggerCron 必填，标准 5/6 段）
	Duration     time.Duration // interval 间隔 / delay 延迟
	Timeout      time.Duration // 单次执行上限（<=0 取默认 10m；§6.2）
	Retries      int           // 失败重试次数（默认 0）
	Backoff      BackoffType   // 退避类型（默认 exponential）
	RetryBase    time.Duration // 退避基数（<=0 取 1s）
	RetryMax     time.Duration // 退避上限（<=0 取 30s）
	AllowOverlap bool          // 允许上次未完成时重叠触发（§6.4 防重入豁免，显式开启）
	Concurrency  int           // 每 Job 并发上限（§6.4；默认 1，AllowOverlap 时生效）
	Scope        Scope         // 默认 node
	ShardTotal   int           // Scope=shard 必填 >=2
	Timezone     string        // IANA 时区
	DSTAware     bool          // 时区含夏令时时必须显式 true（fail-closed；robfig 歧义时刻可能双触发/跳过，使用方验收——审计 A12-5）
	// LockEnabled 跨节点互斥（销项：node 域跨节点重叠敏感任务）。
	// true=每次调度执行前经 coordination LockKey 获取互斥锁（TTL 覆盖最坏执行
	// 窗），拿不到=skip+计数；本节点单飞行/Concurrency 守卫不受影响。Coord=nil
	// （单机降级）无跨节点重叠，锁面无操作。默认 false（无消费者强制启用——G1-15）。
	LockEnabled bool
	// IdempotencyTTL 跨节点幂等窗口（销项：inboxRunKeyTTL 接线）。
	// >0=每次调度执行前经 coordination BeginIdempotency SETNX（键=job 维度），
	// TTL 内已执行=skip；执行失败（非成功终态）释放键——下个触发窗口可重试，
	// 成功则占用至 TTL 过期（同窗口至多一次成功执行）。Coord=nil 单机无操作。
	// 默认 0=关闭（幂等责任仍在 Job 作者）。
	IdempotencyTTL time.Duration
}

// Job 调度任务：模块实现（必须幂等）。
type Job interface {
	Name() string
	Spec() Spec
	Run(ctx context.Context, e Execution) error
}

// Execution 单次执行上下文（§6.2）。
type Execution struct {
	RunID      string // 本次运行 ULID
	JobName    string
	NodeID     string
	Attempt    int          // 1-based（含重试）
	ShardIndex int          // Scope=shard 时本节点分片；其他 -1
	Trigger    Trigger      // 本次触发来源
	TraceID    string       // 触发链路 trace_id（manual 继承调用方；调度触发多为空）
	Logger     *slog.Logger // job 维度 logger（附着 job/run_id 字段）
}

// 默认值（具名；无消费者的不设 Options 项——G1-15）。
const (
	defaultJobTimeout = 10 * time.Minute
	defaultRetryBase  = time.Second
	defaultRetryMax   = 30 * time.Second
	defaultTimezone   = "UTC"
	jobKeyMaxLen      = 128

	shardNone = -1
)

// validate 校验并就地补默认值。
func (s *Spec) validate(name string) error {
	if name == "" || len(name) > jobKeyMaxLen {
		return fmt.Errorf("scheduler: job name length must be 1..%d", jobKeyMaxLen)
	}
	switch s.Trigger {
	case TriggerCron:
		if s.Schedule == "" {
			return fmt.Errorf("scheduler: %s: cron schedule required", name)
		}
		if _, err := parseCron(s.Schedule); err != nil {
			return fmt.Errorf("scheduler: %s: invalid cron: %w", name, err)
		}
	case TriggerInterval, TriggerDelay:
		if s.Duration <= 0 {
			return fmt.Errorf("scheduler: %s: duration must be positive for %s trigger", name, s.Trigger)
		}
	case TriggerOnce:
	default:
		return fmt.Errorf("scheduler: %s: unknown trigger %q", name, s.Trigger)
	}
	if s.Timeout <= 0 {
		s.Timeout = defaultJobTimeout
	}
	if s.Retries < 0 {
		return fmt.Errorf("scheduler: %s: retries must be >= 0", name)
	}
	switch s.Backoff {
	case "":
		s.Backoff = BackoffExponential
	case BackoffFixed, BackoffExponential:
	default:
		return fmt.Errorf("scheduler: %s: unknown backoff %q", name, s.Backoff)
	}
	if s.RetryBase <= 0 {
		s.RetryBase = defaultRetryBase
	}
	if s.RetryMax <= 0 {
		s.RetryMax = defaultRetryMax
	}
	if s.Concurrency < 0 {
		return fmt.Errorf("scheduler: %s: concurrency must be >= 0", name)
	}
	if s.Concurrency == 0 {
		s.Concurrency = 1
	}
	switch s.Scope {
	case "":
		s.Scope = ScopeNode
	case ScopeNode, ScopeCluster:
	case ScopeShard:
		if s.ShardTotal < 2 {
			return fmt.Errorf("scheduler: %s: shard scope requires ShardTotal >= 2", name)
		}
	default:
		return fmt.Errorf("scheduler: %s: unknown scope %q", name, s.Scope)
	}
	if s.Timezone == "" {
		s.Timezone = defaultTimezone
	} else {
		loc, err := time.LoadLocation(s.Timezone)
		if err != nil {
			return fmt.Errorf("scheduler: %s: invalid timezone: %w", name, err)
		}
		// DST fail-closed（显式策略；审计 A12-5）：时区含夏令时转换而
		// 未声明 DSTAware → 拒绝（歧义时刻可能双触发/跳过，必须显式验收）
		if !s.DSTAware && hasDST(loc) {
			return fmt.Errorf("scheduler: %s: timezone %q has DST transitions; set DSTAware=true to accept wall-clock semantics (possible double-fire/skip at transitions)", name, s.Timezone)
		}
	}
	return nil
}

// hasDST 时区一年内是否存在偏移量变化（Go 1.20 ZoneBounds；UTC/固定偏移=false）。
func hasDST(loc *time.Location) bool {
	_, end := time.Now().In(loc).ZoneBounds()
	return !end.IsZero()
}

// backoff 第 attempt 次重试（attempt 从 1 计）的等待时长：exponential=
// base*2^(n-1) 封顶 RetryMax；fixed=恒 RetryBase（仍封顶）；±20% jitter。
func (s *Spec) backoff(attempt int) time.Duration {
	d := s.RetryBase
	if s.Backoff == BackoffExponential {
		d = s.RetryBase << (attempt - 1) // base*2^(n-1)
	}
	if d <= 0 || d > s.RetryMax {
		d = s.RetryMax
	}
	jitter := time.Duration(float64(d) * (rand.Float64()*0.4 - 0.2)) // [-20%,+20%)
	return d + jitter
}

// parseCron 标准 cron（分时日月周 + 可选秒段）。
func parseCron(spec string) (cron.Schedule, error) {
	return cron.ParseStandard(spec)
}

// runJobCtx 包装：执行函数签名统一（tests 复用）。
type jobFunc func(ctx context.Context) error

// runWithTimeout 带超时执行 fn（§6.2 Timeout），panic 隔离转错误。
// 返回 wait（等待 fn goroutine 实际退出的保证）：超时路径上调用方在重试前
// 必须 wait——防无视 ctx 的僵尸 fn 与下一次重试并发跑同一任务（审计 A12-4；
// 僵尸最长存活至 fn 自返，Job 契约=必须响应 ctx）。
func runWithTimeout(ctx context.Context, timeout time.Duration, fn jobFunc) (func(), error) {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic: %v", r)
			}
		}()
		done <- fn(tctx)
	}()
	// wait：确保 fn goroutine 退出（cancel 协作式中断 + 收尾发送）
	wait := func() {
		cancel()
		<-done // 缓冲 1：fn 必发一次（正常/panic/error）
	}
	select {
	case err := <-done:
		cancel()
		return func() {}, err
	case <-tctx.Done():
		// 超时优先于业务错误上报（业务结果已无意义）
		if errors.Is(tctx.Err(), context.DeadlineExceeded) {
			return wait, fmt.Errorf("job timeout after %s", timeout)
		}
		return wait, tctx.Err()
	}
}
