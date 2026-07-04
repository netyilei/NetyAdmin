package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"NetyAdmin/internal/pkg/recovery"
	"NetyAdmin/internal/pkg/requestid"
	pkgSentry "NetyAdmin/internal/pkg/sentry"
)

// EventBus 统一消息总线接口
type EventBus interface {
	Publish(ctx context.Context, topic string, msg interface{}) error
	Subscribe(topic string, handler func(ctx context.Context, msg []byte)) error
	Close() error
	// OnReconnect 注册 Redis 重连成功后的回调（Task 13.2 / 13.3 兜底机制）。
	// 仅 RedisDriver 会在 subscribeLoop 从断连恢复后触发；MemoryDriver 无重连概念，
	// 注册的回调永远不会被调用。可选：若不注册，subscribeLoop 重连后正常运行（no-op）。
	// 回调在独立 goroutine 中执行（GoSafe 包裹），不会阻塞订阅协程。
	OnReconnect(fn func())
}

// Message 统一消息协议
type Message struct {
	Topic     string          `json:"topic"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"timestamp"`
	SenderID  string          `json:"senderId,omitempty"` // 发布节点 ID，driver 接收层据此过滤本节点回环
	// Meta 携带跨 goroutine / 跨节点传递的上下文元数据（如 request_id）。
	// 发布方在 Publish 时从 ctx 提取 request_id 写入 Meta[requestid.MetaKey]；
	// 订阅方在 dispatch 时从 Meta 恢复到子 ctx，让 handler 内的日志 / Sentry 上报
	// 能与原始请求关联。nil Meta 表示无元数据（向后兼容旧消息）。
	Meta map[string]string `json:"meta,omitempty"`
}

// GetMeta 安全读取 Meta[key]，nil Meta 返回空串（向后兼容旧消息）。
func (m *Message) GetMeta(key string) string {
	if m == nil || m.Meta == nil {
		return ""
	}
	return m.Meta[key]
}

// NewMessage 创建消息
func NewMessage(topic string, payload interface{}) (*Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload failed: %w", err)
	}
	return &Message{
		Topic:     topic,
		Payload:   data,
		Timestamp: time.Now().Unix(),
	}, nil
}

// baseBus 基础实现，包含公共逻辑
type baseBus struct {
	nodeID   string // 本节点唯一标识，用于过滤自身广播回环
	handlers map[string][]func(ctx context.Context, msg []byte)
	mu       sync.RWMutex

	// onReconnect 由 RedisDriver.subscribeLoop 在断连恢复后触发（Task 13.2）。
	// MemoryDriver 经由嵌入获得 OnReconnect 方法但永不触发（无重连概念）。
	// 回调可选：未注册时 fireReconnect 是 no-op，subscribeLoop 正常运行。
	onReconnectMu sync.RWMutex
	onReconnect   func()

	// Worker pool (Task 23)
	//
	// dispatchQueue 是 dispatch 阶段的缓冲队列：消费 loop 收到消息后通过 dispatch
	// 投递到该队列，N 个 worker 从队列消费并调用 handler。替代原本 per-event
	// goroutine 的方式，避免高吞吐场景下 goroutine 数量爆炸。
	//
	// 队列满时 dispatch 阻塞（backpressure），让消费 loop 反压到上游 Publish；
	// 队列空时 worker 阻塞在 <-dispatchQueue 等待新消息。
	dispatchQueue chan dispatchJob

	// dispatchStop 在 Close 时被 close，用于解除 dispatch 的阻塞（队列满时）。
	// close 后 dispatch 不会向 dispatchQueue 发送新消息，从而让消费 loop 能
	// 检查 stopChan 并退出。
	dispatchStop chan struct{}

	// dispatchWG 跟踪所有 worker goroutine，shutdownWorkerPool 通过 Wait 等待
	// worker 排空 dispatchQueue 后退出。
	dispatchWG sync.WaitGroup

	// shutdownOnce 保证 shutdownWorkerPool 幂等（多次调用安全）。
	shutdownOnce sync.Once
}

// dispatchJob 是 dispatchQueue 的元素，携带消息和派生自 msg.Meta 的 ctx。
type dispatchJob struct {
	ctx context.Context
	msg *Message
}

func newBaseBus(nodeID string) *baseBus {
	return &baseBus{
		nodeID:   nodeID,
		handlers: make(map[string][]func(ctx context.Context, msg []byte)),
	}
}

// initWorkerPool 初始化 dispatch 队列并启动 N 个 worker goroutine。
// 必须在构造 driver 时调用一次（worker pool 在构造时创建，而非首次 Publish）。
// workers / queueSize 为零值时使用默认值（16 / 1024）。
func (b *baseBus) initWorkerPool(workers, queueSize int) {
	if workers <= 0 {
		workers = 16
	}
	if queueSize <= 0 {
		queueSize = 1024
	}
	b.dispatchQueue = make(chan dispatchJob, queueSize)
	b.dispatchStop = make(chan struct{})
	for i := 0; i < workers; i++ {
		b.dispatchWG.Add(1)
		// 每个 worker 用 GoSafe 启动：worker 内部已有 per-handler defer/recover
		// 防止 handler panic 杀死 worker；GoSafe 是兜底安全网，捕获非 handler
		// panic（理论上不应发生，但作为 belt-and-suspenders 防御性编程）。
		recovery.GoSafe("pubsub:worker", b.workerLoop)
	}
}

// workerLoop 是 worker goroutine 的主循环：从 dispatchQueue 消费消息并调用 handler。
// 当 dispatchQueue 被 close 时，range 循环结束（队列排空后退出）。
//
// 注意：GoSafe 启动 worker 后，GoSafe 的 defer recover 仅在 workerLoop 自身
// panic 时触发（且会让 worker 退出）；handler panic 由 invokeHandlers 内部的
// per-handler defer/recover 捕获，不会传播到 workerLoop，worker 继续运行。
func (b *baseBus) workerLoop() {
	defer b.dispatchWG.Done()
	for job := range b.dispatchQueue {
		b.invokeHandlers(job.ctx, job.msg)
	}
}

// invokeHandlers 调用消息对应的所有订阅者，使用 per-handler defer/recover
// 隔离 panic：单个 handler panic 不会跳过同消息的后续 handler，也不会杀死 worker。
// recover 行为与 recovery.GoSafe 一致（slog.Error + Sentry CaptureException），
// 但不退出 worker，保证 worker pool 容量稳定。
func (b *baseBus) invokeHandlers(ctx context.Context, msg *Message) {
	b.mu.RLock()
	handlers := make([]func(ctx context.Context, msg []byte), len(b.handlers[msg.Topic]))
	copy(handlers, b.handlers[msg.Topic])
	b.mu.RUnlock()

	for _, h := range handlers {
		// Go 1.22+ per-iteration 循环变量语义保证闭包捕获当次迭代的 h，
		// 此处显式参数化仅为可读性与旧版本兼容。
		func(h func(context.Context, []byte)) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("pubsub:handler panic",
						"topic", msg.Topic,
						"panic", r,
						"stack", string(debug.Stack()),
					)
					pkgSentry.CaptureException(fmt.Errorf("%v", r))
				}
			}()
			h(ctx, msg.Payload)
		}(h)
	}
}

// shutdownWorkerPool 优雅关闭 worker pool：
//  1. close(dispatchStop) —— 让正在阻塞的 dispatch 立即返回（不再投递新消息）。
//  2. close(dispatchQueue) —— 通知 worker 排空后退出（range 循环结束）。
//  3. 带超时等待 worker 退出 —— dispatchWG.Wait 加 5s 超时，避免某个 worker
//     卡死（如 handler 内死循环）导致优雅关闭永久阻塞。超时后仅记录告警，
//     不强制中断 worker（进程退出时会强制回收 goroutine）。
//
// 调用方必须确保调用前已停止消费 loop（loopWG.Wait() 已返回），否则可能
// 触发 "send on closed channel" panic（dispatch 与 close 并发）。
//
// 幂等：通过 sync.Once 保护，多次调用安全。
func (b *baseBus) shutdownWorkerPool() {
	b.shutdownOnce.Do(func() {
		close(b.dispatchStop)
		close(b.dispatchQueue)
	})
	// 带超时的等待：worker 卡死时不能永久阻塞优雅关闭流程。
	// 用 GoSafe 启动内部 goroutine（RULES.md §8.3：所有 go func() 必须用 GoSafe）；
	// 超时后该 goroutine 仍阻塞在 Wait，进程退出时由 runtime 强制回收。
	done := make(chan struct{})
	recovery.GoSafe("pubsub:shutdown_wait", func() {
		b.dispatchWG.Wait()
		close(done)
	})

	select {
	case <-done:
		// 所有 worker 已退出
	case <-time.After(5 * time.Second):
		// slog.Warn 而非 Error：drain 超时是可恢复问题，进程即将退出，
		// worker 卡死不会影响数据一致性（与 taskManager.Stop / logBus.Stop 的 drain 超时一致，
		// 见 RULES.md §8.5）。
		slog.Warn("pubsub: shutdownWorkerPool timed out after 5s, some workers may be stuck")
	}
}

// OnReconnect 注册重连回调（由 baseBus 提供默认实现，两个 Driver 经由嵌入获得）。
// 调用方在 SetEventBus 时注册：cacheMgr.SetEventBus(bus) 内部会 bus.OnReconnect(m.reloadL1All)。
func (b *baseBus) OnReconnect(fn func()) {
	b.onReconnectMu.Lock()
	defer b.onReconnectMu.Unlock()
	b.onReconnect = fn
}

// fireReconnect 触发已注册的重连回调。在独立 goroutine 中执行（GoSafe 包裹），
// 防止回调内的 IO（如清空 L1）阻塞 subscribeLoop；回调 panic 也会被 GoSafe 捕获上报。
func (b *baseBus) fireReconnect() {
	b.onReconnectMu.RLock()
	fn := b.onReconnect
	b.onReconnectMu.RUnlock()
	if fn == nil {
		return
	}
	recovery.GoSafe("pubsub:on_reconnect", fn)
}

// buildMessage 构造带 SenderID + Meta 的消息。
// Meta 中至少携带 request_id（若 ctx 中存在），用于跨 goroutine / 跨节点传播。
func (b *baseBus) buildMessage(ctx context.Context, topic string, payload interface{}) (*Message, error) {
	m, err := NewMessage(topic, payload)
	if err != nil {
		return nil, err
	}
	m.SenderID = b.nodeID
	if rid := requestid.FromContext(ctx); rid != "" {
		m.Meta = map[string]string{requestid.MetaKey: rid}
	}
	return m, nil
}

// isFromSelf 判断消息是否由本节点发出（用于过滤回环，避免双重处理）
// 兼容旧版本：若 SenderID 为空（旧节点发布），返回 false 不误过滤
func (b *baseBus) isFromSelf(m *Message) bool {
	return b.nodeID != "" && m.SenderID == b.nodeID
}

func (b *baseBus) Subscribe(topic string, handler func(ctx context.Context, msg []byte)) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], handler)
	return nil
}

// dispatch 将消息投递到 dispatchQueue，由 worker pool 异步消费。
// 从 msg.Meta 恢复 request_id 到 ctx，worker 内 handler 通过 ctx 关联原始请求。
// 派发使用 background context 作为基础（异步执行，原始请求早已返回），
// request_id 是从 Meta 恢复的，而非依赖原始请求 ctx 的生命周期。
//
// 队列满时 dispatch 阻塞（backpressure，默认行为），让消费 loop 反压到上游 Publish；
// 若 dispatchStop 已 close（bus 关闭中），dispatch 立即返回丢弃消息。
func (b *baseBus) dispatch(ctx context.Context, msg *Message) {
	if rid := msg.GetMeta(requestid.MetaKey); rid != "" {
		ctx = requestid.WithRequestID(ctx, rid)
	}
	select {
	case b.dispatchQueue <- dispatchJob{ctx: ctx, msg: msg}:
	case <-b.dispatchStop:
		// Bus 关闭中，丢弃消息（消费 loop 即将退出，无需处理）。
	}
}

// MemoryDriver 单机模式：基于内存 channel 实现
type MemoryDriver struct {
	*baseBus
	stopChan chan struct{}
	msgChan  chan *Message
	closed   bool
	closeMu  sync.Mutex
	loopWG   sync.WaitGroup // 跟踪 loop goroutine，Close 时 Wait 保证 loop 退出后再关 worker pool
}

// NewMemoryDriver 构造内存模式事件总线。
// workers / queueSize 控制 dispatch worker pool 容量，零值使用默认值（16 / 1024）。
func NewMemoryDriver(nodeID string, workers, queueSize int) EventBus {
	d := &MemoryDriver{
		baseBus:  newBaseBus(nodeID),
		stopChan: make(chan struct{}),
		msgChan:  make(chan *Message, 1000),
	}
	d.baseBus.initWorkerPool(workers, queueSize)
	// 异步启动内存驱动循环（GoSafe 包裹 recover + Sentry 上报，防止 panic 导致事件总线静默退出）
	d.loopWG.Add(1)
	recovery.GoSafe("pubsub:memory_loop", func() {
		defer d.loopWG.Done()
		d.loop()
	})
	return d
}

func (d *MemoryDriver) Publish(ctx context.Context, topic string, msg interface{}) error {
	d.closeMu.Lock()
	if d.closed {
		d.closeMu.Unlock()
		return fmt.Errorf("event bus is closed")
	}
	d.closeMu.Unlock()

	m, err := d.buildMessage(ctx, topic, msg)
	if err != nil {
		return err
	}

	select {
	case d.msgChan <- m:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *MemoryDriver) loop() {
	ctx := context.Background()
	for {
		select {
		case <-d.stopChan:
			return
		case msg, ok := <-d.msgChan:
			if !ok {
				return
			}
			// 过滤本节点发出的回环消息，避免双重处理（与 RedisDriver 行为一致）
			if d.isFromSelf(msg) {
				continue
			}
			// 派发时使用 background ctx + msg.Meta 恢复 request_id 到子 ctx，
			// 让订阅者 handler 通过 ctx 关联到原始请求。
			d.dispatch(ctx, msg)
		}
	}
}

func (d *MemoryDriver) Close() error {
	d.closeMu.Lock()
	if d.closed {
		d.closeMu.Unlock()
		return nil
	}
	d.closed = true
	d.closeMu.Unlock()

	// 1. close stopChan 信号 loop 退出；同时 dispatchStop 在 shutdownWorkerPool 内 close
	//    以解除 dispatch 阻塞（队列满时 loop 会卡在 dispatch，需 dispatchStop 唤醒）。
	// 2. loopWG.Wait 等 loop 完全退出后再 close dispatchQueue，避免 dispatch 与 close 并发
	//    触发 "send on closed channel" panic。
	close(d.stopChan)
	d.loopWG.Wait()
	d.shutdownWorkerPool()
	return nil
}

// RedisDriver 集群模式：基于 Redis Pub/Sub
type RedisDriver struct {
	*baseBus
	redisClient *redis.Client
	prefix      string
	channel     string
	stopChan    chan struct{}
	wg          sync.WaitGroup
	closed      bool
	closeMu     sync.Mutex
}

// NewRedisDriver 构造 Redis 模式事件总线。
// workers / queueSize 控制 dispatch worker pool 容量，零值使用默认值（16 / 1024）。
func NewRedisDriver(redisClient *redis.Client, prefix string, nodeID string, workers, queueSize int) EventBus {
	if prefix == "" {
		prefix = "netyadmin"
	}

	d := &RedisDriver{
		baseBus:     newBaseBus(nodeID),
		redisClient: redisClient,
		prefix:      prefix,
		channel:     fmt.Sprintf("%s:channel:system_bus", prefix),
		stopChan:    make(chan struct{}),
	}
	d.baseBus.initWorkerPool(workers, queueSize)

	if redisClient != nil {
		d.wg.Add(1)
		// 异步启动 Redis 订阅循环（GoSafe 包裹 recover + Sentry 上报，防止 panic 导致订阅静默退出）
		recovery.GoSafe("pubsub:subscribe_loop", d.subscribeLoop)
	}

	return d
}

func (d *RedisDriver) Publish(ctx context.Context, topic string, msg interface{}) error {
	d.closeMu.Lock()
	if d.closed {
		d.closeMu.Unlock()
		return fmt.Errorf("event bus is closed")
	}
	d.closeMu.Unlock()

	if d.redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}

	m, err := d.buildMessage(ctx, topic, msg)
	if err != nil {
		return err
	}

	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal message failed: %w", err)
	}

	return d.redisClient.Publish(ctx, d.channel, data).Err()
}

func (d *RedisDriver) subscribeLoop() {
	defer d.wg.Done()

	const reconnectDelay = 3 * time.Second
	// hasDisconnected 区分「首次连接」与「断连后重连」：仅断连恢复后才触发 OnReconnect，
	// 避免应用启动时（首次 Subscribe 成功）误触发 L1 全量 reload。
	hasDisconnected := false

	for {
		d.closeMu.Lock()
		if d.closed {
			d.closeMu.Unlock()
			return
		}
		d.closeMu.Unlock()

		ctx := context.Background()
		sub := d.redisClient.Subscribe(ctx, d.channel)
		ch := sub.Channel()

		// 重连成功（Subscribe 返回新句柄）：若此前发生过断连，触发 OnReconnect 回调。
		// 回调在独立 goroutine 中执行（fireReconnect 内部用 GoSafe 包裹），不阻塞订阅循环。
		// 在进入消息循环前触发，确保 L1 清空先于后续 invalidation 消息处理。
		if hasDisconnected {
			d.fireReconnect()
		}

		func() {
			defer sub.Close()
			for {
				select {
				case <-d.stopChan:
					return
				case msg, ok := <-ch:
					if !ok {
						// channel 关闭，Redis 断连，尝试重连
						hasDisconnected = true
						return
					}
					var m Message
					if err := json.Unmarshal([]byte(msg.Payload), &m); err != nil {
						continue
					}
					// 过滤本节点发出的回环消息（Redis Pub/Sub 不区分发布者）
					if d.isFromSelf(&m) {
						continue
					}
					// 派发时使用 background ctx + msg.Meta 恢复 request_id 到子 ctx，
					// 让订阅者 handler 通过 ctx 关联到原始请求。
					d.dispatch(ctx, &m)
				}
			}
		}()

		// 检查是否应该退出
		select {
		case <-d.stopChan:
			return
		case <-time.After(reconnectDelay):
			// 重连延迟后继续循环
		}
	}
}

func (d *RedisDriver) Close() error {
	d.closeMu.Lock()
	if d.closed {
		d.closeMu.Unlock()
		return nil
	}
	d.closed = true
	d.closeMu.Unlock()

	// 1. close stopChan 信号 subscribeLoop 退出；dispatchStop 在 shutdownWorkerPool 内 close
	//    以解除 dispatch 阻塞（队列满时 subscribeLoop 会卡在 dispatch，需 dispatchStop 唤醒）。
	// 2. wg.Wait 等 subscribeLoop 完全退出后再 close dispatchQueue，避免 dispatch 与 close 并发
	//    触发 "send on closed channel" panic。
	close(d.stopChan)
	d.wg.Wait()
	d.shutdownWorkerPool()
	return nil
}
