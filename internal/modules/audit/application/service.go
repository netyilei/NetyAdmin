// service.go：审计服务编排——Sink 注册表 + 异步批量 + 保留清理 job。
// 语义：Write 异步入有界队列（满 → 同步降级直写，审计不丢；丢弃=违规，计数告警）；
// flush loop 定时/定量批量投递 Sink；Close 排空残余。记录只产生一次。
// goroutine 惯例与 scheduler 一致（named loop + wg + recover；不提供 kernel.GoSafe
// 统一封装——各模块自带此惯例，本函数为范例）。
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/audit/domain"
)

// Config 是服务参数（config.audit 段绑定，module.go 注入）。
type Config struct {
	RetentionDays int `yaml:"retention_days"` // operation_logs 保留天数（observability.md §1 基线 180）
	// 批量参数无具名消费者调优诉求（G1-15），固定为常量不设配置项。
}

const (
	flushBatchSize = 200             // 单批上限
	flushInterval  = 2 * time.Second // 定时 flush 周期
	queueSize      = 4096            // 有界队列上限
)

// ErrClosed 是关停后写入的错误（调用方应停止产生审计而非重试）。
var ErrClosed = errors.New("audit: service closed")

// Service 是审计编排入口（AttachDB 后 Start；Close 排空）。
type Service struct {
	cfg    Config
	logger *slog.Logger
	clock  primitives.Clock

	mu    sync.RWMutex
	sinks []domain.Sink
	ch    chan domain.Record

	drainMu   sync.RWMutex // Write(读)/Close 排空(写) 互斥——竞态丢记录根修
	wg        sync.WaitGroup
	closeOnce sync.Once
	done      chan struct{}
	closed    atomic.Bool
}

// New 创建（logger 必填；clock 可空默认 SystemClock——测试注入）。
func New(cfg Config, logger *slog.Logger, clock primitives.Clock) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if clock == nil {
		clock = primitives.SystemClock{}
	}
	return &Service{
		cfg:    cfg,
		logger: logger,
		clock:  clock,
		ch:     make(chan domain.Record, queueSize),
		done:   make(chan struct{}),
	}
}

// RegisterSink 注册写入端点（Start 前完成；重复名 fail）。
func (s *Service) RegisterSink(sink domain.Sink) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ex := range s.sinks {
		if ex.Name() == sink.Name() {
			return fmt.Errorf("audit: duplicate sink %q", sink.Name())
		}
	}
	s.sinks = append(s.sinks, sink)
	return nil
}

// Start 启动 flush loop（goroutine 惯例：named loop + wg + panic recover）。
// ctx 仅 trace 传播——生命周期由 s.done 驱动（Close 关停）：装配钩子 ctx 传
// 进来会在钩子返回时被 cancel，flush loop 监听它即挂死（code-audit P0 实证：
// db-connect 钩子 ctx → 3s 内 0 次 flush），故 WithoutCancel 隔离。
func (s *Service) Start(ctx context.Context) {
	if s.closed.Load() {
		return
	}
	loopCtx := context.WithoutCancel(ctx) //nolint:govet // 生命周期归 s.done，非调用方 ctx
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if p := recover(); p != nil {
				s.logger.Error("audit: flush loop panic", "panic", fmt.Sprint(p))
			}
		}()
		s.flushLoop(loopCtx)
	}()
}

// flushLoop 定时/定量批量投递（退出仅由 Close 的 done 驱动；残余由 Close 排空）。
func (s *Service) flushLoop(ctx context.Context) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.flushAvailable(ctx)
		}
	}
}

// flushAvailable 非阻塞取尽当前队列（≤单批上限）投递。
func (s *Service) flushAvailable(ctx context.Context) {
	batch := make([]domain.Record, 0, flushBatchSize)
	for len(batch) < flushBatchSize {
		select {
		case r := <-s.ch:
			batch = append(batch, r)
			continue
		default:
		}
		break
	}
	s.dispatch(ctx, batch)
}

// dispatch 批量投递全部 sink（单 sink 失败计数告警不阻断其余——记录唯一
// 归属，失败记录由该 sink 自行补偿；DB sink 失败=依赖不可用，健康面可见）。
func (s *Service) dispatch(ctx context.Context, batch []domain.Record) {
	if len(batch) == 0 {
		return
	}
	s.mu.RLock()
	sinks := append([]domain.Sink(nil), s.sinks...)
	s.mu.RUnlock()
	for _, sink := range sinks {
		if err := sink.Flush(ctx, batch); err != nil {
			s.logger.Error("audit: sink flush failed", "sink", sink.Name(), "count", len(batch), "err", err)
		}
	}
}

// Write 异步入队（满 → 同步降级直写全部 sink：审计不丢，代价是调用方延迟；
// 关停后 ErrClosed）。
// 入队与排空互斥见 drainMu（code-audit 竞态窗口内入队即无人读=静默丢）。
// 降级直写用独立预算 ctx——调用方 ctx 可能已取消，取消的 ctx 直写必败（P1 实证）。
func (s *Service) Write(ctx context.Context, r domain.Record) error {
	s.drainMu.RLock()
	defer s.drainMu.RUnlock()
	if s.closed.Load() {
		return ErrClosed
	}
	if r.ID == "" {
		r.ID = primitives.NewULID(primitives.SystemClock{})
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = s.clock.Now().UTC()
	}
	select {
	case s.ch <- r:
		return nil
	default:
		s.logger.Warn("audit: queue full, sync fallback", "action", r.Action)
		fbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.dispatch(fbCtx, []domain.Record{r})
		return nil
	}
}

// Close 排空残余并关停（幂等；bootstrap BeforeShutdown 段调用）。
// 排空用独立预算——排空段 5s 由全部清理动作共享，居末的 audit 拿过期 ctx =
// 全部 dispatch 失败丢记录（code-audit P1 实证）。
func (s *Service) Close(context.Context) error {
	s.drainMu.Lock() // 阻塞新 Write 入队，排空无并发写
	defer s.drainMu.Unlock()
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		close(s.done)
	})
	s.wg.Wait() // flush loop 退出

	deadlineCtx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	batch := make([]domain.Record, 0, flushBatchSize)
	drain := make(chan struct{})
	s.wg.Add(1)
	go func() { // named drain：与 flush loop 同惯例（wg + recover）
		defer s.wg.Done()
		defer close(drain)
		defer func() {
			if p := recover(); p != nil {
				s.logger.Error("audit: drain panic", "panic", fmt.Sprint(p))
			}
		}()
		for {
			select {
			case r := <-s.ch:
				batch = append(batch, r)
				if len(batch) >= flushBatchSize {
					s.dispatch(deadlineCtx, batch)
					batch = batch[:0]
				}
			default:
				return
			}
		}
	}()
	select {
	case <-drain:
	case <-deadlineCtx.Done():
	}
	s.wg.Wait()
	if len(batch) > 0 {
		s.dispatch(deadlineCtx, batch)
	}
	return nil
}
