// scheduler.go：全局调度器门面——注册/触发引擎/执行域/重试/排空
// 。语义：
//   - 触发：cron（per-tz）/interval/once/delay/manual（TriggerNow 管理面）；
//     event/queue 触发随消费方接入。
//   - 执行域：node=每节点跑；cluster=仅 leader 跑（Redis 租约选主，
//     Coord 为空=自任 leader，仅单机降级合法，AGENTS §3）；shard=hash(nodeID)
//     取分片（静态拓扑确定性；动态再平衡随成员感知）。
//   - 防重入：单飞行守卫（未完成即跳过本次触发），AllowOverlap 显式豁免；
//     跨节点面（node 域重叠敏感任务互斥 / 幂等窗口）经 Spec.LockEnabled /
//     Spec.IdempotencyTTL 显式启用（coordination 原语，默认关——机制面）；
//     未启用时 leader 失联切换窗口的重复触发由 Job 幂等兜底。
//   - 重试：指数退避+±20% jitter，Retries 上限后标记 failed（运行记录即死信，
//     手动触发即重放——不建独立 DLQ 表）。
//   - 运行记录：先写 running 再执行；启动时恢复本节点上次生命周期的
//     running 行为 unknown（崩溃恢复）；DB 未挂接则无记录。
//   - 排空：Close 停触发→等在途 Job（drain 上限）→超时标记 unknown→释放 leader。
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/kernel/state/coordination"
)

// 具名默认值（G1-15：无消费者不设 Options 项）。
const (
	globalWorkers    = 8                // 全局并发上限（§6.4 worker 池）
	leaderTTL        = 15 * time.Second // 选主租约 TTL
	leaderRenewEvery = 5 * time.Second
	heartbeatTTL     = 15 * time.Second // 节点存活标记 TTL（审计 A12-3）
	heartbeatEvery   = 5 * time.Second
	staleSweepEvery  = 30 * time.Second // 他节点僵尸行扫荡周期（仅 leader）
	drainTimeout     = 5 * time.Second  // §6.4 优雅关闭等待在途 Job
	leaderGroup      = "na-scheduler"   // 选主键（基座唯一调度权）

	lockNamespace     = "na-scheduler/lock"     // LockEnabled 互斥锁键空间（job 维度）
	idemNamespace     = "na-scheduler/run-idem" // IdempotencyTTL 幂等键空间（job 维度）
	lockReleaseBudget = 2 * time.Second         // 锁释放限时（对齐 resign 收尾口径）
)

// 哨兵错误。
var (
	ErrClosed         = errors.New("scheduler: closed")
	ErrAlreadyStart   = errors.New("scheduler: already started")
	ErrDuplicateJob   = errors.New("scheduler: duplicate job")
	ErrJobNotFound    = errors.New("scheduler: job not found")
	ErrNoDB           = errors.New("scheduler: database not attached")
	ErrStarted        = errors.New("scheduler: cannot attach after start")
	ErrRunningOverlap = errors.New("scheduler: job already running")
	// ErrNotLeader 手动触发 cluster 域任务时本节点非 leader（F2：触发结果如实
	// 上报——此前 dispatch 静默 skip 而管理面恒报成功）。
	ErrNotLeader = errors.New("scheduler: not leader (cluster job dispatch skipped)")
	// ErrPaused 手动触发处于暂停态的任务（同 F2 如实上报语义）。
	ErrPaused = errors.New("scheduler: job paused")
)

// Options 构造选项。
type Options struct {
	NodeID  string                     // 必填（运行记录 node 列 + 分片哈希）
	Logger  *slog.Logger               // 必填
	Metrics *observability.Metrics     // 必填（na_scheduler_*）
	Clock   primitives.Clock           // 可空（默认 SystemClock）
	Coord   *coordination.Coordination // 可空：cluster 域选主；nil=自任 leader（单机降级）
	DB      *database.Pool             // 可空：运行记录/RecentRuns 依赖（AttachDB 延迟注入亦可）
	Tracing *observability.Tracing     // 可空：job span（；nil/未接线=无 span）
}

type jobState struct {
	job     Job
	spec    Spec
	paused  atomic.Bool
	running atomic.Bool   // 单飞行守卫（Concurrency=1 时生效）
	sem     chan struct{} // 每 Job 并发上限（AllowOverlap=true 时生效；nil=单飞行）
}

// Scheduler 全局调度器门面。
type Scheduler struct {
	nodeID  string
	cfg     Options
	logger  *slog.Logger
	clock   primitives.Clock
	metrics *observability.Metrics
	tracing *observability.Tracing
	coord   *coordination.Coordination
	db      *database.Pool

	mu    sync.RWMutex
	jobs  map[string]*jobState
	crons map[string]*cron.Cron // per-tz cron 实例（实例级隔离；goroutine 进程级常驻）
	order []string              // 注册序（诊断面）

	sem          chan struct{} // 全局 worker 信号量
	wg           sync.WaitGroup
	inflight     sync.WaitGroup // 在途执行（spawn 前 Add，Close 排空等待可见全部触发）
	startCtx     context.Context
	startCan     context.CancelFunc
	executeCtx   context.Context // 执行域：排空窗口内存活（在途任务完成），超时才取消
	executeCan   context.CancelFunc
	leaderMu     sync.Mutex
	leaderHolder string // 非空=本节点当前持锁
	started      atomic.Bool
	closed       atomic.Bool
}

// New 构造调度器（注册在 Start 前完成）。
func New(opts Options) (*Scheduler, error) {
	if opts.NodeID == "" {
		return nil, fmt.Errorf("scheduler: node id required")
	}
	if opts.Logger == nil {
		return nil, fmt.Errorf("scheduler: logger required")
	}
	if opts.Metrics == nil {
		return nil, fmt.Errorf("scheduler: metrics required")
	}
	s := &Scheduler{
		nodeID:  opts.NodeID,
		cfg:     opts,
		logger:  opts.Logger,
		clock:   opts.Clock,
		metrics: opts.Metrics,
		tracing: opts.Tracing,
		coord:   opts.Coord,
		db:      opts.DB,
		jobs:    map[string]*jobState{},
		crons:   map[string]*cron.Cron{},
		sem:     make(chan struct{}, globalWorkers),
	}
	if s.clock == nil {
		s.clock = primitives.SystemClock{}
	}
	return s, nil
}

// AttachDB 延迟注入运行记录库（须在 Start 前；幂等/不同池 fail-closed，同 bus）。
func (s *Scheduler) AttachDB(pool *database.Pool) error {
	if s.closed.Load() {
		return ErrClosed
	}
	if s.started.Load() {
		return ErrStarted
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		s.db = pool
		return nil
	}
	if s.db != pool {
		return fmt.Errorf("scheduler: attach db mismatch (different pool already attached)")
	}
	return nil
}

// Register 注册任务（Start 前必须全部完成；同名重复 fail）。
func (s *Scheduler) Register(job Job) error {
	if s.closed.Load() {
		return ErrClosed
	}
	if s.started.Load() {
		return fmt.Errorf("%w: register after start (job %s would never trigger)", ErrAlreadyStart, job.Name())
	}
	if job == nil {
		return fmt.Errorf("scheduler: nil job")
	}
	name := job.Name()
	spec := job.Spec()
	if err := spec.validate(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.jobs[name]; dup {
		return fmt.Errorf("%w: %s", ErrDuplicateJob, name)
	}
	js := &jobState{job: job, spec: spec}
	js.sem = make(chan struct{}, spec.Concurrency) // 恒创建：容量=Concurrency（默认 1，§6.4 每 Job 上限）
	s.jobs[name] = js
	s.order = append(s.order, name)
	return nil
}

// Start 启动：崩溃恢复（本节点 running→unknown）→ leader 竞选/续租循环 →
// 逐 job 启动触发循环。
func (s *Scheduler) Start(ctx context.Context) error {
	if s.closed.Load() {
		return ErrClosed
	}
	if !s.started.CompareAndSwap(false, true) {
		return ErrAlreadyStart
	}
	s.startCtx, s.startCan = context.WithCancel(context.Background())
	s.executeCtx, s.executeCan = context.WithCancel(context.Background())
	if s.db != nil {
		// 崩溃恢复失败（如迁移未执行缺表）不阻断启动：运行记录是可观测设施，
		// 触发/执行核心不受影响（对齐 bus relay 降级语义；迁移执行命令化随）
		if err := s.recoverRuns(ctx); err != nil {
			s.logger.Warn("scheduler run recovery skipped", "err", err)
		}
		// 他节点僵尸行扫荡（审计 A12-3）：其存活标记（Heartbeat TTL）已过期
		// → 视为永久宕机，running 标记 unknown。存活判定随 Redis 心跳（无
		// Coord=单机无他节点，不扫荡）。
		if s.coord != nil {
			if err := s.sweepStaleRuns(ctx); err != nil {
				s.logger.Warn("scheduler stale sweep skipped", "err", err)
			}
		}
	}
	if s.coord != nil {
		s.wg.Add(2)
		go func() { defer s.wg.Done(); s.heartbeatLoop(s.startCtx) }()
		go func() { defer s.wg.Done(); s.staleSweepLoop(s.startCtx) }()
	}
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.leaderLoop(s.startCtx) }()

	s.mu.RLock()
	names := append([]string(nil), s.order...)
	s.mu.RUnlock()
	for _, name := range names {
		s.mu.RLock()
		js := s.jobs[name]
		s.mu.RUnlock()
		s.startTriggers(name, js)
	}
	mode := "leader(standalone)"
	if s.coord != nil {
		mode = "leader(elected)"
	}
	s.logger.Info("scheduler started", "node", s.nodeID, "jobs", len(names), "mode", mode)
	return nil
}

// heartbeatLoop 节点存活标记续期（多节点僵尸判定依据；审计 A12-3）。
// Coord==nil 单机不启动（无他节点）。心跳续期失败仅告警——TTL 过期后他节点
// 将本节点 running 视为 unknown（Job 幂等兜底）。
func (s *Scheduler) heartbeatLoop(ctx context.Context) {
	name := "scheduler:" + s.nodeID
	for {
		if err := s.coord.Heartbeat(ctx, name, heartbeatTTL); err != nil && ctx.Err() == nil {
			s.logger.Warn("scheduler heartbeat failed", "err", err)
		}
		if !sleepCtx(ctx, heartbeatEvery) {
			return
		}
	}
}

// staleSweepLoop 僵尸行扫荡（仅 leader 执行，多写者避免）：running 行的归属
// 节点存活标记已过期 → 标记 unknown（按策略补跑/TriggerNow 手动重放）。
func (s *Scheduler) staleSweepLoop(ctx context.Context) {
	if s.db == nil {
		return
	}
	for {
		if s.isLeader() {
			if err := s.sweepStaleRuns(ctx); err != nil && ctx.Err() == nil {
				s.logger.Warn("scheduler stale sweep failed", "err", err)
			}
		}
		if !sleepCtx(ctx, staleSweepEvery) {
			return
		}
	}
}

// sweepStaleRuns 扫荡他节点僵尸 running 行：归属节点存活标记过期 → unknown。
func (s *Scheduler) sweepStaleRuns(ctx context.Context) error {
	nodes, err := s.db.Query(ctx, `SELECT DISTINCT node FROM scheduler_runs WHERE status = $1 AND node <> $2`, runRunning, s.nodeID)
	if err != nil {
		return fmt.Errorf("scheduler: stale nodes query: %w", err)
	}
	var dead []string
	for nodes.Next() {
		var node string
		if err := nodes.Scan(&node); err != nil {
			nodes.Close()
			return fmt.Errorf("scheduler: stale nodes scan: %w", err)
		}
		alive, aerr := s.coord.Alive(ctx, "scheduler:"+node)
		if aerr != nil {
			continue // 存活判定失败保守跳过（宁可暂留 phantom 不误标死）
		}
		if !alive {
			dead = append(dead, node)
		}
	}
	nodes.Close()
	if err := nodes.Err(); err != nil {
		return fmt.Errorf("scheduler: stale nodes rows: %w", err)
	}
	for _, node := range dead {
		if _, err := s.db.Exec(ctx, `UPDATE scheduler_runs
			SET status = $2, finished_at = $3, updated_at = $3
			WHERE node = $4 AND status = $1`, runRunning, runUnknown, s.clock.Now(), node); err != nil {
			s.logger.Warn("scheduler stale mark failed", "node", node, "err", err)
			continue
		}
		s.logger.Warn("scheduler stale node runs marked unknown", "node", node)
	}
	return nil
}

// leaderLoop 选主/续租循环：Coord 空 → 恒 leader；否则 Campaign+Renew，
// 失主后释放标记（cluster 域任务停止触发），重竞选。
// 注意：Campaign 失败时返回**持有者**的 holder——仅 isLeader 时才登记，
// 否则本节点会误认自己持锁（cluster 守卫失效）。
func (s *Scheduler) leaderLoop(ctx context.Context) {
	if s.coord == nil {
		return // 单机降级：恒 leader
	}
	for {
		holder, isLeader, err := s.coord.Campaign(ctx, leaderGroup, leaderTTL)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			s.logger.Warn("scheduler campaign failed", "err", err)
			s.resign()
			if !sleepCtx(ctx, leaderRenewEvery) {
				return
			}
			continue
		}
		if !isLeader {
			s.resign() // 置空标记（他人持有；Campaign 返回的 holder 是持有者的）
			if !sleepCtx(ctx, leaderRenewEvery) {
				return
			}
			continue
		}
		s.leaderMu.Lock()
		s.leaderHolder = holder
		s.leaderMu.Unlock()
		s.logger.Info("scheduler leadership acquired", "holder", holder)
		// 续租循环：失主后回竞选
		for {
			if !sleepCtx(ctx, leaderRenewEvery) {
				s.resign()
				return
			}
			s.leaderMu.Lock()
			h := s.leaderHolder
			s.leaderMu.Unlock()
			if h == "" {
				break // 已失主（Run 检测失败置空）→ 重新竞选
			}
			if err := s.coord.RenewLeadership(ctx, leaderGroup, h, leaderTTL); err != nil {
				s.logger.Warn("scheduler leadership lost", "err", err)
				s.resign()
				break
			}
		}
	}
}

// resign 释放 leader 标记与租约。
func (s *Scheduler) resign() {
	s.leaderMu.Lock()
	h := s.leaderHolder
	s.leaderHolder = ""
	s.leaderMu.Unlock()
	if h != "" && s.coord != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.coord.Resign(ctx, leaderGroup, h)
	}
}

// isLeader 当前节点是否持有调度权（cluster 域触发前置）。
func (s *Scheduler) isLeader() bool {
	s.leaderMu.Lock()
	defer s.leaderMu.Unlock()
	return s.coord == nil || s.leaderHolder != ""
}

// shardIndexOf 本节点分片：FNV hash(nodeID) % total（静态拓扑确定性；
// 动态再平衡随成员感知）。
func (s *Scheduler) shardIndexOf(total int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s.nodeID))
	return int(h.Sum32() % uint32(total))
}

// Close 优雅关闭：停触发 → 等在途 Job（drain 上限，超时标记 unknown）→
// 释放 leader。幂等；Close 后 Register/TriggerNow 返回 ErrClosed。
func (s *Scheduler) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	if s.startCan != nil {
		s.startCan() // 停触发/leader 循环（§6.4 第一步）
	}
	s.wg.Wait()
	// 排空：执行域 ctx 存活，在途任务（含重试退避）继续至完成；超时才取消
	// 并标记 unknown（未完成标记 unknown 按策略补跑）
	done := make(chan struct{})
	go func() {
		s.inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(drainTimeout):
		s.markAbandonedRunning(context.Background(), drainTimeout)
		s.logger.Warn("scheduler drain exceeded; in-flight jobs cancelled and marked unknown")
	}
	s.executeCan()
	s.resign()
	return nil
}

// TriggerNow 手动触发（管理面；§6.5 手动触发/立即重跑）。
// 走同一执行管线（防重入守卫生效）。执行域=生命周期 executeCtx（与 fireOnce
// 同域——HTTP 请求 ctx 在响应返回即取消，会杀掉在途任务；联测审计 A2 根修）。
// 入参 ctx 仅作为「提交前取消」信号：提交后任务不受调用方取消影响。
// 确定性守卫（暂停/cluster 非 leader）同步前置判定并返回哨兵错误——管理面
// 如实上报（F2），dispatch 内守卫保留为权威防线（提交与执行间状态翻转仍拦截）。
func (s *Scheduler) TriggerNow(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.closed.Load() {
		return ErrClosed
	}
	s.mu.RLock()
	js, ok := s.jobs[name]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrJobNotFound, name)
	}
	if js.paused.Load() {
		return fmt.Errorf("%w: %s", ErrPaused, name)
	}
	if js.spec.Scope == ScopeCluster && !s.isLeader() {
		return fmt.Errorf("%w: %s", ErrNotLeader, name)
	}
	s.executeAsync(s.executeCtx, name, js, TriggerManual)
	return nil
}

// Pause/Resume 暂停/恢复调度（§6.5；触发循环保留，tick 跳过并计数）。
// 本方法只改本节点内存态——跨节点一致的外置暂停态由消费方（cmd taskcenter）
// 经 configcenter 键广播应用（F2），kernel 不依赖任何模块。
func (s *Scheduler) Pause(name string) error  { return s.setPaused(name, true) }
func (s *Scheduler) Resume(name string) error { return s.setPaused(name, false) }

// Has 报告任务是否已注册（管理面预检——避免为未注册任务写外置状态键）。
func (s *Scheduler) Has(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.jobs[name]
	return ok
}

func (s *Scheduler) setPaused(name string, v bool) error {
	if s.closed.Load() {
		return ErrClosed
	}
	s.mu.RLock()
	js, ok := s.jobs[name]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrJobNotFound, name)
	}
	js.paused.Store(v)
	return nil
}

// JobInfo 任务目录条目（诊断面/管理面数据源）。
type JobInfo struct {
	Name     string `json:"name"`
	Scope    string `json:"scope"`
	Trigger  string `json:"trigger"`
	Schedule string `json:"schedule,omitempty"` // cron 表达式 / interval|delay 的 duration
	Timezone string `json:"timezone"`
	Paused   bool   `json:"paused"`
	Running  bool   `json:"running"`
}

// Catalog 任务目录快照（§6.5 管理面数据源；诊断面 scheduler 条目）。
func (s *Scheduler) Catalog() []JobInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]JobInfo, 0, len(s.order))
	for _, name := range s.order {
		js := s.jobs[name]
		schedule := js.spec.Schedule
		if js.spec.Trigger == TriggerInterval || js.spec.Trigger == TriggerDelay {
			schedule = js.spec.Duration.String()
		}
		out = append(out, JobInfo{
			Name:     name,
			Scope:    string(js.spec.Scope),
			Trigger:  string(js.spec.Trigger),
			Schedule: schedule,
			Timezone: js.spec.Timezone,
			Paused:   js.paused.Load(),
			Running:  js.running.Load(),
		})
	}
	return out
}
