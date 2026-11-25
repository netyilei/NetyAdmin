// trigger.go：触发循环与执行管线。
// 触发引擎：cron（per-tz 实例共享，时区语义；回调内检查生命周期 ctx）/
// interval ticker/once 立即/delay 定时。执行管线：调度守卫（暂停/leader/
// 单飞行）→ 全局信号量 → 跨节点守卫（LockEnabled 互斥锁/IdempotencyTTL 幂等，
// 显式启用生效）→ 运行记录 running → 重试循环（指数退避+jitter）→ 终态回写
// → 排空等待。分片：各节点只执行本节点分片（Execution.ShardIndex 携带），
// 无 skip 分支——分片选择即归属。
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/kernel/state/coordination"
)

// 触发跳过原因（na_scheduler_skipped_total[reason]）。
const (
	skipPaused     = "paused"
	skipNotLeader  = "not-leader"
	skipOverlap    = "overlap"
	skipWorkerFull = "worker-full"
	skipLock       = "lock"       // LockEnabled：互斥锁被他节点持有/不可用（fail-closed）
	skipIdem       = "idempotent" // IdempotencyTTL：窗口内已有执行
)

// lockKeyOf/job 维度互斥锁键（coordination 追加 ns 环境段——铁律 6）。
func lockKeyOf(job string) string { return lockNamespace + "/" + job }

// runIdemKeyOf/job 维度幂等键。
func runIdemKeyOf(job string) string { return idemNamespace + "/" + job }

// lockTTL 锁 TTL=最坏执行窗（Timeout×全部尝试 + 退避上限×重试次数）：覆盖
// 本节点整次调度（含重试），持锁节点崩溃由 TTL 自然释放（无续租面——机制
// 面最小实现；执行普遍超 TTL 的任务应调 Timeout 而非依赖锁续期）。
func lockTTL(spec Spec) time.Duration {
	return spec.Timeout*time.Duration(spec.Retries+1) + spec.RetryMax*time.Duration(spec.Retries)
}

// startTriggers 按 Spec.Trigger 启动对应触发循环。
func (s *Scheduler) startTriggers(name string, js *jobState) {
	switch js.spec.Trigger {
	case TriggerCron:
		s.startCron(name, js)
	case TriggerInterval:
		s.startTicker(name, js, js.spec.Duration)
	case TriggerOnce:
		s.fireOnce(name, js, 0) // Start 后立即一次
	case TriggerDelay:
		s.fireOnce(name, js, js.spec.Duration)
	case TriggerManual:
		// 仅 TriggerNow 触发，无循环
	}
}

// startCron 注册 cron 条目（per-tz cron 实例进程级共享；回调检查生命周期，
// Close 后不再入队）。robfig 内部 goroutine 为进程级常驻（goleak 惯例忽略）。
func (s *Scheduler) startCron(name string, js *jobState) {
	loc, err := time.LoadLocation(js.spec.Timezone)
	if err != nil {
		loc = time.UTC
	}
	c := cronFor(loc)
	if _, err := c.AddFunc(js.spec.Schedule, func() {
		s.executeAsync(s.startCtx, name, js, TriggerCron)
	}); err != nil {
		// Register 时已校验可解析，此处防御性兜底
		s.logger.Error("scheduler cron add failed", "job", name, "err", err)
	}
}

// cronRegistry per-tz cron 实例（同 tz 共享一个调度循环；进程级常驻）。
var (
	cronMu       sync.Mutex
	cronRegistry = map[string]*cron.Cron{}
)

// cronFor per-tz cron 实例（同 tz 共享一个调度循环）。
func cronFor(loc *time.Location) *cron.Cron {
	cronMu.Lock()
	defer cronMu.Unlock()
	if c, ok := cronRegistry[loc.String()]; ok {
		return c
	}
	c := cron.New(cron.WithLocation(loc))
	c.Start()
	return c
}

// startTicker interval 循环：循环生命周期随 startCtx（Close 即停触发，
// 审计 F4），执行投递随 executeCtx（interval 按单调钟）。
func (s *Scheduler) startTicker(name string, js *jobState, interval time.Duration) {
	loopCtx, execCtx := s.startCtx, s.executeCtx
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-loopCtx.Done():
				return
			case <-t.C:
				s.executeAsync(execCtx, name, js, TriggerInterval)
			}
		}
	}()
}

// fireOnce 延迟后单次触发。
func (s *Scheduler) fireOnce(name string, js *jobState, delay time.Duration) {
	ctx := s.startCtx    // 延迟等待随生命周期
	exec := s.executeCtx // 执行随排空域
	go func() {
		if delay > 0 {
			if !sleepCtx(ctx, delay) {
				return
			}
		} else if !sleepCtx(ctx, time.Millisecond) {
			return
		}
		s.executeAsync(exec, name, js, TriggerOnce)
	}()
}

// executeAsync 触发投递（触发循环与执行解耦，§6.4 worker 池语义——执行不
// 阻塞触发循环）。inflight.Add 前置于 spawn：Close 的 inflight.Wait 必须
// 覆盖所有已投递触发（防排空窗口内 tick 被 executeCan 杀掉——审计 B1）。
func (s *Scheduler) executeAsync(ctx context.Context, name string, js *jobState, trigger Trigger) {
	if ctx.Err() != nil {
		return
	}
	tickAt := s.clock.Now()
	s.metrics.SchedulerTicks.WithLabelValues(name).Inc() // 调度触达（含守卫跳过）
	s.inflight.Add(1)
	go func() {
		defer s.inflight.Done()
		s.dispatch(ctx, tickAt, name, js, trigger)
	}()
}

// dispatch 守卫（暂停/leader/每 Job 并发上限）→ 全局信号量 → 跨节点锁/幂等
// （LockEnabled/IdempotencyTTL 显式启用；Coord=nil 单机无操作）→ 执行。
// 每 Job 信号量恒存在（容量=Concurrency，默认 1=单飞行语义，§6.4 每 Job 上限）。
func (s *Scheduler) dispatch(ctx context.Context, tickAt time.Time, name string, js *jobState, trigger Trigger) {
	if ctx.Err() != nil {
		return
	}
	if js.paused.Load() {
		s.metrics.SchedulerSkipped.WithLabelValues(name, skipPaused).Inc()
		return
	}
	if js.spec.Scope == ScopeCluster && !s.isLeader() {
		s.metrics.SchedulerSkipped.WithLabelValues(name, skipNotLeader).Inc()
		return
	}
	// 每 Job 并发上限（§6.4）：非阻塞尝试——槽满即跳过（防重入守卫语义=skip
	// 而非排队，interval 快于执行时无界积压不可接受）；AllowOverlap 提高上限
	select {
	case js.sem <- struct{}{}:
	default:
		s.metrics.SchedulerSkipped.WithLabelValues(name, skipOverlap).Inc()
		return
	}
	defer func() { <-js.sem }()

	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		s.metrics.SchedulerSkipped.WithLabelValues(name, skipWorkerFull).Inc()
		return
	}
	// 跨节点守卫（销项——机制面，默认关；Coord=nil 单机降级无跨节点
	// 重叠面，两守卫均无操作）：先锁后幂等——锁未拿到不消耗幂等窗口。
	if s.coord != nil && js.spec.LockEnabled {
		holder, acquired, lerr := s.coord.LockKey(ctx, lockKeyOf(name), lockTTL(js.spec))
		if lerr != nil {
			s.logger.Warn("scheduler lock acquire failed; skip (fail-closed)", "job", name, "err", lerr)
		}
		if !acquired {
			s.metrics.SchedulerSkipped.WithLabelValues(name, skipLock).Inc()
			return
		}
		defer func() { // 锁覆盖整次调度（含重试）；释放失败由 TTL 兜底
			rctx, cancel := context.WithTimeout(context.Background(), lockReleaseBudget)
			defer cancel()
			if uerr := s.coord.UnlockKey(rctx, lockKeyOf(name), holder); uerr != nil {
				s.logger.Warn("scheduler lock release failed (TTL 兜底)", "job", name, "err", uerr)
			}
		}()
	}
	idemAcquired := false
	if s.coord != nil && js.spec.IdempotencyTTL > 0 {
		if err := s.coord.BeginIdempotency(ctx, runIdemKeyOf(name), js.spec.IdempotencyTTL); err != nil {
			if !errors.Is(err, coordination.ErrDenied) {
				s.logger.Warn("scheduler idempotency check failed; skip (fail-closed)", "job", name, "err", err)
			}
			s.metrics.SchedulerSkipped.WithLabelValues(name, skipIdem).Inc()
			return
		}
		idemAcquired = true
	}
	err := s.runWithRetries(ctx, tickAt, name, js, trigger)
	if idemAcquired && err != nil {
		// 非成功终态释放幂等键：下个触发窗口可重试（成功占用至 TTL——窗口内
		// 至多一次成功执行；释放失败键按 TTL 过期，重试窗口收窄不破坏正确性）。
		rctx, cancel := context.WithTimeout(context.Background(), lockReleaseBudget)
		defer cancel()
		if rerr := s.coord.ReleaseIdempotency(rctx, runIdemKeyOf(name)); rerr != nil {
			s.logger.Warn("scheduler idempotency release failed", "job", name, "err", rerr)
		}
	}
}

// runWithRetries 单次调度的完整执行：运行记录 running → 重试循环 → 终态。
// 返回 nil=成功终态；非 nil=失败/中断（dispatch 据此释放幂等键——下窗重试）。
// 全局信号量按 attempt 持有（退避等待期间释放，防 8 槽被睡眠占满饿死其他
// 任务——审计 F2）；重试穷尽标记 failed（运行记录即死信；TriggerNow 手动重放）。
func (s *Scheduler) runWithRetries(ctx context.Context, tickAt time.Time, name string, js *jobState, trigger Trigger) error {
	spec := js.spec
	runID := primitives.NewULID(s.clock)
	shardIdx := shardNone
	if spec.Scope == ScopeShard {
		shardIdx = s.shardIndexOf(spec.ShardTotal)
	}
	ctx, span := s.jobSpan(ctx, name) // ：span 先开，运行记录 trace_id 延续同链
	defer s.endJobSpan(span)
	traceID := observability.TraceIDFrom(ctx)

	if s.db != nil {
		if err := s.insertRun(ctx, runRecord{
			runID: runID, job: name, node: s.nodeID, triggerKind: string(trigger),
			shard: shardIdx, traceID: traceID, startedAt: tickAt,
		}); err != nil {
			s.logger.Warn("scheduler run record insert failed", "job", name, "err", err)
		}
	}

	e := Execution{
		RunID: runID, JobName: name, NodeID: s.nodeID,
		ShardIndex: shardIdx, Trigger: trigger, TraceID: traceID,
		Logger: s.logger.With("job", name, "run_id", runID),
	}
	var lastErr error
	for attempt := 1; attempt <= spec.Retries+1; attempt++ {
		if ctx.Err() != nil {
			s.markUnknown(ctx, runID, name)
			return ctx.Err()
		}
		e.Attempt = attempt
		// 全局槽按 attempt 持有：acquire→执行→释放，退避 sleep 不占槽
		select {
		case s.sem <- struct{}{}:
		case <-ctx.Done():
			s.markUnknown(ctx, runID, name)
			return ctx.Err()
		}
		s.metrics.SchedulerStarted.WithLabelValues(name).Inc()
		s.metrics.SchedulerStartLag.WithLabelValues(name).Observe(s.clock.Now().Sub(tickAt).Seconds())

		start := s.clock.Now()
		waitFn, err := runWithTimeout(ctx, spec.Timeout, func(rctx context.Context) error {
			return js.job.Run(rctx, e)
		})
		<-s.sem // 全局槽按 attempt 释放（退避与僵尸等待不占槽）
		s.metrics.SchedulerRunDuration.WithLabelValues(name).Observe(s.clock.Now().Sub(start).Seconds())
		if err == nil {
			s.metrics.SchedulerSucceeded.WithLabelValues(name).Inc()
			s.finishRun(ctx, runID, name, runSucceeded, attempt, "")
			return nil
		}
		lastErr = err
		recordJobError(span, err)
		s.logger.Warn("scheduler job failed", "job", name, "run_id", runID, "attempt", attempt, "err", err)
		if attempt <= spec.Retries {
			s.metrics.SchedulerRetrying.WithLabelValues(name).Inc()
			if !sleepCtx(ctx, spec.backoff(attempt)) {
				waitFn() // 僵尸收尾（有界：fn 响应 ctx 即返；无视 ctx 者阻塞至自返）
				s.markUnknown(ctx, runID, name)
				return ctx.Err()
			}
			waitFn() // 重试前等僵尸退出——防新旧 attempt 并发跑同一任务（审计 A12-4）
		}
	}
	// 重试穷尽：failed（运行记录即死信；TriggerNow 手动重放）
	s.metrics.SchedulerFailed.WithLabelValues(name).Inc()
	s.finishRun(ctx, runID, name, runFailed, spec.Retries+1, fmt.Sprintf("%v", lastErr))
	s.syncFailedDepth(ctx)
	return lastErr
}

// sleepCtx 可取消退避（与 bus 包同名函数各自独立——5 行小函数，抽公共包
// 反而引入耦合；评审如判重复可收拢）。
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

// jobSpan 为单次调度运行开 span（Tracing 未接线时返回原 ctx 与 nil）。
// span 先于运行记录创建——其 trace 回写自定义 trace_id ctx，运行记录 trace_id
// 列与 span 同链（传播链 Job 环节）。
func (s *Scheduler) jobSpan(ctx context.Context, job string) (context.Context, trace.Span) {
	if !s.tracing.Enabled() {
		return ctx, nil
	}
	tctx, span := s.tracing.StartSpan(ctx, "scheduler", "job/"+job,
		trace.SpanKindInternal, attribute.String("job.name", job))
	if sc := span.SpanContext(); sc.HasTraceID() {
		tctx = observability.WithTraceID(tctx, sc.TraceID().String())
	}
	return tctx, span
}

// endJobSpan nil-safe 收口（未接线时 span 为 nil）。
func (s *Scheduler) endJobSpan(span trace.Span) {
	if span != nil {
		span.End()
	}
}

// recordJobError nil-safe 错误记录（每次 attempt 失败记录；终态标记在运行记录）。
func recordJobError(span trace.Span, err error) {
	if span != nil && err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("error", true))
	}
}
