// bus.go：全局总线核心——消息信封、订阅注册表、三域发布路由。
// 语义（§5.3，发布方选择投递域；订阅方按 topic 收取到达消息）：
//   - Local：进程内同步执行（per-handler 超时），错误返回发布方。
//   - Broadcast：本地异步投递 + Redis Pub/Sub relay 到所有节点（at-most-once，
//     有界队列满丢弃并计数）；WithRemoteOnly 跳过本地段（发布方已自行处理的场景）。
//   - Cluster：Redis Streams 消费组竞争消费一次（at-least-once，失败重投；
//     达到 MaxAttempts 入 DLQ，需 DB，无 DB 退回丢弃计数）。
//
// 单机无 Redis：Broadcast/Cluster 降级为本地投递（AGENTS §3 仅 topology=single
// 允许回退；multi 无 Redis 时构造直接报错）。
package bus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
)

// Kind 消息类别。
type Kind string

const (
	KindDomain      Kind = "domain"
	KindPlatform    Kind = "platform"
	KindIntegration Kind = "integration"
)

// Domain 发布域。
type Domain string

const (
	DomainLocal     Domain = "local"
	DomainBroadcast Domain = "broadcast"
	DomainCluster   Domain = "cluster"
)

// Event 类型化事件：值类型实现，Topic() 不依赖字段（订阅侧取零值 topic）。
// Topic 常量在模块 api/events 定义（§5.5）。
type Event interface{ Topic() string }

// Message 消息信封（§5.2）。Headers 约定键见下方常量（principal 由 buildHeaders
// 按 ctx 自动补齐——authn.Principal.String() 格式 type:id）。
type Message struct {
	ID         string            `json:"id"` // ULID，全局唯一，幂等判定依据
	Topic      string            `json:"topic"`
	Kind       Kind              `json:"kind"`
	SchemaVer  int               `json:"schema_ver"`
	Payload    json.RawMessage   `json:"payload,omitempty"` // JSON
	Headers    map[string]string `json:"headers,omitempty"`
	OccurredAt time.Time         `json:"occurred_at"`
}

// Handler 消息处理器。错误=投递失败：Broadcast 计数、Cluster 不 ack 待重投。
// 必须幂等：Broadcast at-most-once、Cluster at-least-once，不承诺 exactly-once（§5.4）。
type Handler func(ctx context.Context, msg Message) error

// Headers 键约定（§5.2）。
const (
	HeaderTraceID   = "trace_id"
	HeaderRequestID = "request_id"
	HeaderSource    = "source"
	HeaderPrincipal = "principal" // ：发布方身份（type:id；有损传播，非凭证）
)

// 哨兵错误（调用方 errors.Is 判定）。
var (
	ErrClosed        = errors.New("bus: closed")
	ErrInvalidTopic  = errors.New("bus: invalid topic")
	ErrInvalidOption = errors.New("bus: invalid publish option")
	ErrAlreadyStart  = errors.New("bus: already started")
	ErrNoRedis       = errors.New("bus: redis required")
)

// 默认值（具名；handler 超时 2s/排空 5s 与/ 生命周期契约同口径）。
// 容量/并发/裁剪长度无具名消费者（G1-15），固定为常量不设 Options 项。
const (
	defaultQueueSize       = 1024
	defaultWorkers         = 4
	defaultHandlerTimeout  = 2 * time.Second
	defaultReclaimInterval = 30 * time.Second
	defaultMaxAttempts     = 3
	defaultStreamMaxLen    = 10000
	defaultShutdownDrain   = 5 * time.Second
	defaultKeyRoot         = "na"

	reclaimIdleMargin = time.Second // PEL 回收最小空闲 = HandlerTimeout + margin（防在途双处理）
	clusterPoll       = 200 * time.Millisecond
	subscribeOpTO     = 2 * time.Second // Subscribe 建流/建组的单次 Redis 操作超时
	topicMaxLen       = 128
	schemaVerDefault  = 1

	prefixDomain      = "domain."
	prefixPlatform    = "platform."
	prefixIntegration = "integration."

	modeNormal    = "normal"
	modeLocalOnly = "local-only"

	deliverResultSuccess   = "success"
	deliverResultError     = "error"
	deliverResultDropped   = "dropped"
	deliverResultDiscarded = "discarded"
)

// Options 构造选项。RedisAddr 为空时仅允许 AllowLocalFallback=true（单机降级）。
type Options struct {
	NodeID             string                 // 必填：集群内消费者标识 + Headers.source（须全集群唯一）
	Logger             *slog.Logger           // 必填
	Metrics            *observability.Metrics // 必填（na_bus_* 指标）
	Clock              primitives.Clock       // 可空（默认 SystemClock）
	RedisAddr          string                 // 空 → 仅本地（须 AllowLocalFallback）
	AllowLocalFallback bool                   // 单机无 Redis 时允许本地回退（仅 topology=single 传 true）
	KeyRoot            string                 // Redis 键 root（namespaces.md；默认 na，来自 cfg.Redis.Prefix）
	Env                string                 // 环境段（铁律 6：key 必须含环境段；来自 cfg.Server.Env，必填）
	DB                 *database.Pool         // 可空（Outbox/DLQ 依赖；装配后段 AttachDB 延迟注入亦可）

	HandlerTimeout  time.Duration // 单 handler 超时（默认 2s）
	ReclaimInterval time.Duration // Streams PEL 回收周期（默认 30s）
	MaxAttempts     int           // Cluster 重试上限（默认 3；超过 XAck+丢弃计数）

	injectRedis redisFace // 测试注入面（同包测试用；置非 nil 时代替 NewRedisFace）
	// outboxRetryBase 测试注入面（行级失败退避基数；0=默认 outboxRetryBase 常量
	// ——生产恒默认值，G1-15 不设配置项）。
	outboxRetryBase time.Duration
}

// Bus 全局总线门面。
type Bus struct {
	instID  string // 实例标识（防 Pub/Sub 自环双执行；与 NodeID 无关）
	cfg     Options
	keys    keySet
	logger  *slog.Logger
	clock   primitives.Clock
	metrics *observability.Metrics

	mu       sync.RWMutex
	subs     map[string][]subEntry
	bcastSeq atomic.Uint64 // Broadcast 发布序列（缺口检测）
	seq      uint64

	q        *queue
	redis    redisFace      // nil = local-only
	db       *database.Pool // Outbox/DLQ（nil = 不可用，走降级路径）
	bcast    *broadcastTransport
	cluster  *clusterTransport
	startCtx context.Context
	startCan context.CancelFunc
	runWG    sync.WaitGroup
	started  atomic.Bool
	closed   atomic.Bool
}

type subEntry struct {
	id       uint64
	fn       Handler
	filter   func(Message) bool // nil=全收
	priority int                // 同 topic 投递序（升序；相同保持注册序）
	blocking bool               // Local 域发布方等待 handler 实际返回（不看 HandlerTimeout）
	gate     chan struct{}      // 并发闸门（cap=concurrency；单订阅跨消息在途上限）
}

// New 构造总线（不连 Redis；连接在 Start 验证）。
func New(opts Options) (*Bus, error) {
	if opts.NodeID == "" {
		return nil, fmt.Errorf("bus: node id required")
	}
	if opts.Logger == nil {
		return nil, fmt.Errorf("bus: logger required")
	}
	if opts.Metrics == nil {
		return nil, fmt.Errorf("bus: metrics required")
	}
	if opts.Env == "" {
		return nil, fmt.Errorf("bus: env required (namespaces env segment)")
	}
	if opts.RedisAddr == "" && !opts.AllowLocalFallback {
		return nil, fmt.Errorf("%w: redis addr empty without local fallback (multi topology must configure redis)", ErrNoRedis)
	}
	if opts.Clock == nil {
		opts.Clock = primitives.SystemClock{}
	}
	if opts.HandlerTimeout <= 0 {
		opts.HandlerTimeout = defaultHandlerTimeout
	}
	if opts.ReclaimInterval <= 0 {
		opts.ReclaimInterval = defaultReclaimInterval
	}
	normalizeInt(&opts.MaxAttempts, defaultMaxAttempts)
	root := opts.KeyRoot
	if root == "" {
		root = defaultKeyRoot
	}
	b := &Bus{
		instID:  primitives.NewULID(opts.Clock),
		cfg:     opts,
		keys:    newKeySet(root, opts.Env),
		logger:  opts.Logger,
		clock:   opts.Clock,
		metrics: opts.Metrics,
		db:      opts.DB,
		subs:    map[string][]subEntry{},
	}
	b.q = newQueue(defaultQueueSize, defaultWorkers, b.runItem)
	switch {
	case opts.injectRedis != nil:
		b.redis = opts.injectRedis // 测试注入面优先
	case opts.RedisAddr != "":
		b.redis = NewRedisFace(opts.RedisAddr)
	}
	return b, nil
}

func normalizeInt(v *int, def int) {
	if *v <= 0 {
		*v = def
	}
}

// Start 启动远端段（relay/消费/回收循环）。Redis 不可达直接报错（fail-fast；
// 单机降级场景由构造方传 AllowLocalFallback，不走 Start）。
func (b *Bus) Start(ctx context.Context) error {
	if b.closed.Load() {
		return ErrClosed
	}
	if !b.started.CompareAndSwap(false, true) {
		return ErrAlreadyStart
	}
	b.startCtx, b.startCan = context.WithCancel(context.Background())
	if b.redis == nil {
		b.logger.Warn("bus local-only mode: broadcast/cluster degrade to local delivery")
		return nil
	}
	if err := b.redis.Ping(ctx); err != nil {
		b.started.Store(false)
		b.startCan()
		return fmt.Errorf("bus: redis ping: %w", err)
	}
	ps, err := b.redis.Subscribe(ctx, b.keys.broadcastChannel)
	if err != nil {
		b.started.Store(false)
		b.startCan()
		return fmt.Errorf("bus: subscribe broadcast channel: %w", err)
	}
	b.bcast = &broadcastTransport{
		face:    b.redis,
		ps:      ps,
		ch:      b.keys.broadcastChannel,
		instID:  b.instID,
		logger:  b.logger,
		seq:     &b.bcastSeq,
		enqueue: b.enqueueBroadcast,
		lost:    b.recordBroadcastLost,
	}
	b.cluster = &clusterTransport{
		face:         b.redis,
		keys:         b.keys,
		logger:       b.logger,
		nodeID:       b.cfg.NodeID,
		maxLen:       defaultStreamMaxLen,
		maxAttempts:  int64(b.cfg.MaxAttempts),
		minIdle:      b.cfg.HandlerTimeout + reclaimIdleMargin,
		reclaimEvery: b.cfg.ReclaimInterval,
		topics:       b.clusterTopics,
		enqueue:      b.enqueueCluster,
		ack:          b.ackEntry,
		storeDead:    b.storeDeadEntry,
		discarded:    b.recordDiscarded,
	}
	b.runWG.Add(3)
	go func() { defer b.runWG.Done(); b.bcast.run(b.startCtx) }()
	go func() { defer b.runWG.Done(); b.cluster.run(b.startCtx) }()
	go func() { defer b.runWG.Done(); b.cluster.reclaim(b.startCtx) }()
	if b.db != nil {
		b.runWG.Add(1)
		go func() { defer b.runWG.Done(); b.runRelay(b.startCtx) }() // Outbox 中继
	}
	b.logger.Info("bus started", "node", b.cfg.NodeID, "outbox", b.db != nil)
	return nil
}

// Close 停止循环、排空队列（ShutdownDrain 上限，超时残留交由后台继续到处理完）并关闭连接。
// 幂等；Close 后 Publish/Subscribe 返回 ErrClosed。
func (b *Bus) Close() error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}
	if b.startCan != nil {
		b.startCan()
	}
	// 先关订阅连接：relay 循环经 msgs 通道关闭退出，pubsubImpl 转发 goroutine
	// 随源通道关闭退出（顺序在 Wait 之前，保证泄漏面收敛）
	if b.bcast != nil {
		_ = b.bcast.ps.Close()
	}
	b.runWG.Wait()
	if residual := b.q.close(defaultShutdownDrain); residual > 0 {
		b.logger.Warn("bus shutdown drain exceeded", "residual_items", residual)
	}
	if b.redis != nil {
		_ = b.redis.Close()
	}
	return nil
}

// publishConfig 发布选项集合。
type publishConfig struct {
	domain      Domain
	kind        Kind
	schemaVer   int
	remoteOnly  bool
	aggregateID string
}

// PublishOption 发布选项。
type PublishOption func(*publishConfig)

// WithDomain 指定发布域（缺省 Local：不显式声明不产生跨节点副作用）。
func WithDomain(d Domain) PublishOption {
	return func(c *publishConfig) { c.domain = d }
}

// WithKind 指定消息类别（缺省 domain）。
func WithKind(k Kind) PublishOption {
	return func(c *publishConfig) { c.kind = k }
}

// WithSchemaVer 指定 schema 版本（缺省 1；§5.5 版本规则随事件目录落地）。
func WithSchemaVer(v int) PublishOption {
	return func(c *publishConfig) { c.schemaVer = v }
}

// WithRemoteOnly 跳过本地投递，仅远端（仅 Broadcast 有意义；发布方已完成本地处理，
// 如缓存失效先改本地再广播远端）。
func WithRemoteOnly() PublishOption {
	return func(c *publishConfig) { c.remoteOnly = true }
}

func (c *publishConfig) validate() error {
	switch c.domain {
	case DomainLocal, DomainBroadcast, DomainCluster:
	default:
		return fmt.Errorf("%w: unknown domain %q", ErrInvalidOption, c.domain)
	}
	switch c.kind {
	case KindDomain, KindPlatform, KindIntegration:
	default:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidOption, c.kind)
	}
	if c.schemaVer < 1 {
		return fmt.Errorf("%w: schema ver must be >= 1", ErrInvalidOption)
	}
	if c.remoteOnly && c.domain != DomainBroadcast {
		return fmt.Errorf("%w: remote only applies to broadcast domain", ErrInvalidOption)
	}
	return nil
}

// Publish 发布消息（信封缺省字段就地补齐：ID/OccurredAt/Kind/SchemaVer/Headers）。
// Broadcast：先 relay 后本地（relay 失败返回错误，本地未投递，重试语义干净）。
func (b *Bus) Publish(ctx context.Context, msg Message, opts ...PublishOption) error {
	if b.closed.Load() {
		return ErrClosed
	}
	cfg := publishConfig{domain: DomainLocal, kind: KindDomain, schemaVer: schemaVerDefault}
	for _, o := range opts {
		o(&cfg)
	}
	if err := cfg.validate(); err != nil {
		return err
	}
	prepared, err := b.prepare(ctx, msg, cfg)
	if err != nil {
		return err
	}
	msg = prepared
	b.metrics.BusPublished.WithLabelValues(msg.Topic, string(cfg.domain)).Inc()

	switch cfg.domain {
	case DomainLocal:
		return b.deliverLocal(ctx, msg)
	case DomainBroadcast:
		if b.redis == nil {
			if cfg.remoteOnly {
				// 本地模式无远端可投：静默丢弃会违背发布方语义，显式报错由调用方决策
				return fmt.Errorf("%w: remote-only publish in local-only mode", ErrNoRedis)
			}
			b.enqueueBroadcast(msg) // 降级：本地异步投递
			return nil
		}
		if err := b.requireStarted(); err != nil {
			return err
		}
		if err := b.bcast.relay(ctx, msg); err != nil {
			return err
		}
		if !cfg.remoteOnly {
			b.enqueueBroadcast(msg)
		}
		return nil
	case DomainCluster:
		if b.redis == nil {
			b.enqueueCluster(msg, nil) // 降级：单节点本地异步执行一次
			return nil
		}
		if err := b.requireStarted(); err != nil {
			return err
		}
		return b.cluster.publish(ctx, msg)
	}
	return nil // 不可达（validate 已穷举域）
}

// requireStarted 远端域发布前置：Redis 已配置但未 Start（transport 未建）时
// 拒绝，防 nil 解引用（本地域/降级模式不依赖 Start）。
func (b *Bus) requireStarted() error {
	if !b.started.Load() {
		return fmt.Errorf("bus: not started (call Start before remote-domain publish)")
	}
	return nil
}

// prepare 信封缺省补齐与校验（Publish 与 PublishToOutbox 共用；G1-06）。
func (b *Bus) prepare(ctx context.Context, msg Message, cfg publishConfig) (Message, error) {
	if err := validateTopic(msg.Topic); err != nil {
		return Message{}, err
	}
	if msg.ID == "" {
		msg.ID = primitives.NewULID(b.clock)
	}
	if msg.OccurredAt.IsZero() {
		msg.OccurredAt = b.clock.Now()
	}
	if msg.Kind == "" {
		msg.Kind = cfg.kind
	}
	if msg.SchemaVer == 0 {
		msg.SchemaVer = cfg.schemaVer
	}
	msg.Headers = b.buildHeaders(ctx, msg.Headers)
	return msg, nil
}

// SubscribeOption 订阅选项（过滤/优先级/并发/阻塞——Local/Broadcast/Cluster
// 三域一致语义：分发路径共用 deliver）。
type SubscribeOption func(*subscribeConfig)

// subscribeConfig 订阅选项集合。
type subscribeConfig struct {
	filter      func(Message) bool
	priority    int
	concurrency int
	blocking    bool
}

// WithFilter 消息过滤：返回 false 的消息不投递到本订阅（过滤在 handler 前
// 执行；filter panic 按不匹配处理并告警——隔离毒过滤器，不击穿 worker）。
func WithFilter(fn func(Message) bool) SubscribeOption {
	return func(c *subscribeConfig) { c.filter = fn }
}

// WithPriority 投递优先级：同 topic 多订阅按 priority **升序**投递（小者先）；
// 相同值保持注册顺序（稳定排序）。默认 0。
func WithPriority(n int) SubscribeOption {
	return func(c *subscribeConfig) { c.priority = n }
}

// WithConcurrency 单订阅并发处理上限（默认 1=串行；>1 才允许跨消息并行）。
// 语义=同一订阅同时最多 n 个消息在 handler 内（跨 Local 并发发布方/异步
// 队列 worker）；闸门按订阅隔离，不阻塞同 topic 其他订阅。0/负数按默认 1。
func WithConcurrency(n int) SubscribeOption {
	return func(c *subscribeConfig) { c.concurrency = n }
}

// WithBlocking 本地阻塞模式：Local 域发布方等待本订阅 handler **实际返回**
// 后才返回（不受 HandlerTimeout 提前返回约束；仅 ctx 取消可放弃等待）。
// 默认（不加本选项）保持语义：HandlerTimeout 到点即返回，在途
// handler 在后台继续。Broadcast/Cluster 异步域不适用（分发本身异步）。
func WithBlocking() SubscribeOption {
	return func(c *subscribeConfig) { c.blocking = true }
}

// Subscribe 订阅 topic（同 topic 多订阅依次 fan-out；选项见 SubscribeOption）。
// Unsubscribe 不删除 Redis 消费组（组持久化，避免删组丢消息）。
func (b *Bus) Subscribe(topic string, fn Handler, opts ...SubscribeOption) (Subscription, error) {
	if b.closed.Load() {
		return Subscription{}, ErrClosed
	}
	if err := validateTopic(topic); err != nil {
		return Subscription{}, err
	}
	if fn == nil {
		return Subscription{}, fmt.Errorf("bus: nil handler for topic %s", topic)
	}
	cfg := subscribeConfig{concurrency: 1}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.concurrency < 1 {
		cfg.concurrency = 1 // 0/负数=默认串行（保序语义）
	}
	if b.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), subscribeOpTO)
		defer cancel()
		if err := b.redis.XGroupCreateMkStream(ctx, b.keys.streamKey(topic), b.keys.consumerGroup); err != nil {
			return Subscription{}, fmt.Errorf("bus: create stream group %s: %w", topic, err)
		}
	}
	entry := subEntry{
		fn:       fn,
		filter:   cfg.filter,
		priority: cfg.priority,
		blocking: cfg.blocking,
		gate:     make(chan struct{}, cfg.concurrency),
	}
	b.mu.Lock()
	b.seq++
	entry.id = b.seq
	b.subs[topic] = append(b.subs[topic], entry)
	b.mu.Unlock()
	return Subscription{b: b, topic: topic, id: entry.id}, nil
}

// Subscription 订阅句柄；零值安全（Unsubscribe 无操作）。
type Subscription struct {
	b     *Bus
	topic string
	id    uint64
}

// Unsubscribe 移除本订阅（幂等）。
func (s Subscription) Unsubscribe() {
	if s.b == nil {
		return
	}
	s.b.mu.Lock()
	list := s.b.subs[s.topic]
	for i, e := range list {
		if e.id == s.id {
			rest := append(list[:i], list[i+1:]...)
			if len(rest) == 0 {
				delete(s.b.subs, s.topic)
			} else {
				s.b.subs[s.topic] = rest
			}
			break
		}
	}
	s.b.mu.Unlock()
}

// PublishEvent 类型化发布：ev JSON 序列化为 Payload，topic 取 ev.Topic()。
func PublishEvent[E Event](b *Bus, ctx context.Context, ev E, opts ...PublishOption) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("bus: marshal event %T: %w", ev, err)
	}
	return b.Publish(ctx, Message{Topic: ev.Topic(), Payload: json.RawMessage(data)}, opts...)
}

// SubscribeEvent 类型化订阅：Payload 反序列化为 E 后调用 fn（选项透传
// Subscribe——过滤/优先级/并发/阻塞语义同裸订阅）。
// E 必须为值类型（topic 取自零值，指针类型会在 Topic() 处空指针）。
func SubscribeEvent[E Event](b *Bus, fn func(context.Context, E) error,
	opts ...SubscribeOption) (Subscription, error) {

	var zero E
	return b.Subscribe(zero.Topic(), func(ctx context.Context, msg Message) error {
		var ev E
		if len(msg.Payload) > 0 {
			if err := json.Unmarshal(msg.Payload, &ev); err != nil {
				return fmt.Errorf("bus: unmarshal event %s: %w", msg.Topic, err)
			}
		}
		return fn(ctx, ev)
	}, opts...)
}

// Snapshot 诊断面（diagnostics 数据源）。
type Snapshot struct {
	Mode   string         `json:"mode"`   // normal | local-only
	Topics map[string]int `json:"topics"` // topic → 订阅数
}

func (b *Bus) Snapshot() Snapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()
	topics := make(map[string]int, len(b.subs))
	for t, list := range b.subs {
		topics[t] = len(list)
	}
	mode := modeNormal
	if b.redis == nil {
		mode = modeLocalOnly
	}
	return Snapshot{Mode: mode, Topics: topics}
}

// deliverLocal 本地段同步执行（Local 域）：按优先级逐订阅投递，错误聚合返回
// 发布方（blocking 订阅等待 handler 实际返回；默认订阅受 HandlerTimeout 约束）。
func (b *Bus) deliverLocal(ctx context.Context, msg Message) error {
	return b.deliver(ctx, msg, DomainLocal)
}

// deliver 是分发统一入口（Local/Broadcast/Cluster 三域共用——选项语义一致）：
// 订阅按 priority 升序（稳定=注册序）；filter 不匹配跳过；单订阅 gate 控制在途。
func (b *Bus) deliver(ctx context.Context, msg Message, domain Domain) error {
	var errs []error
	for _, e := range b.snapshot(msg.Topic) {
		if b.skip(e, msg) {
			continue
		}
		if err := b.dispatch(ctx, msg, e, domain); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// skip 执行订阅过滤器（nil=全收）。filter panic 按不匹配处理并告警——毒过滤器
// 不击穿 worker/发布方。
func (b *Bus) skip(e subEntry, msg Message) (skip bool) {
	if e.filter == nil {
		return false
	}
	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("bus subscribe filter panic (treated as no match)", "topic", msg.Topic, "panic", r)
			skip = true
		}
	}()
	return !e.filter(msg)
}

// dispatch 单订阅投递：并发闸门（cap=concurrency）内执行 handler；
// blocking 仅 Local 域生效（异步域分发语义即不等待）。
func (b *Bus) dispatch(ctx context.Context, msg Message, e subEntry, domain Domain) error {
	select {
	case e.gate <- struct{}{}:
	case <-ctx.Done():
		return fmt.Errorf("bus subscribe concurrency wait: %w", ctx.Err())
	}
	defer func() { <-e.gate }()
	if domain == DomainLocal && e.blocking {
		return b.runBlockingHandler(ctx, msg, e.fn, domain)
	}
	return b.runHandler(ctx, msg, e.fn, domain)
}

// runItem 队列 worker 入口（Broadcast 本地/远端段与 Cluster 降级/消费段共用）：
// 执行全部订阅；Cluster 条目成功后 ack，失败留 PEL 待回收重投。
func (b *Bus) runItem(item deliveryItem) {
	ctx := context.Background()
	b.observeE2E(item.msg, item.domain)
	execErr := b.deliver(ctx, item.msg, item.domain)
	if item.ack == nil {
		return
	}
	if execErr != nil {
		return // 不 ack：回收循环重投（at-least-once）
	}
	if err := b.ackEntry(item.ack.stream, item.ack.entryID); err != nil {
		// ack 失败留 PEL 重投；消费方幂等兜底
		b.logger.Warn("bus cluster ack failed", "topic", item.msg.Topic, "id", item.ack.entryID, "err", err)
	}
}

// ackEntry 带超时的条目确认（runItem 与毒消息路径共用）。
func (b *Bus) ackEntry(stream, entryID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), subscribeOpTO)
	defer cancel()
	return b.redis.XAck(ctx, stream, b.keys.consumerGroup, entryID)
}

// observeE2E 端到端延迟（observability.md §3 Bus 行）：handler 开始时刻 -
// OccurredAt（发布时刻）。跨节点含时钟偏差，作趋势观测不做绝对承诺。
func (b *Bus) observeE2E(msg Message, domain Domain) {
	if msg.OccurredAt.IsZero() {
		return
	}
	b.metrics.BusE2ELatency.WithLabelValues(string(domain)).Observe(b.clock.Now().Sub(msg.OccurredAt).Seconds())
}

// runHandler 执行单个 handler：header 贯穿 ctx + 超时 + panic 隔离 + 指标。
func (b *Bus) runHandler(ctx context.Context, msg Message, fn Handler, domain Domain) error {
	ctx = headerCtx(ctx, msg.Headers)
	hctx, cancel := context.WithTimeout(ctx, b.cfg.HandlerTimeout)
	defer cancel()
	start := b.clock.Now()
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("bus handler panic: %v", r)
			}
		}()
		done <- fn(hctx, msg)
	}()
	var err error
	timer := time.NewTimer(b.cfg.HandlerTimeout)
	defer timer.Stop()
	select {
	case err = <-done:
	case <-timer.C:
		cancel() // 协作式中断在途 handler
		err = fmt.Errorf("bus handler timeout after %s", b.cfg.HandlerTimeout)
	}
	b.recordDelivery(msg.Topic, domain, start, err)
	return err
}

// runBlockingHandler 执行阻塞订阅 handler（Local 域 WithBlocking）：不设
// HandlerTimeout 弃等——发布方只在 handler 实际返回或 ctx 取消后继续。
// panic 隔离/指标与 runHandler 同口径。
func (b *Bus) runBlockingHandler(ctx context.Context, msg Message, fn Handler, domain Domain) error {
	ctx = headerCtx(ctx, msg.Headers)
	start := b.clock.Now()
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("bus handler panic: %v", r)
			}
		}()
		done <- fn(ctx, msg)
	}()
	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		// 调用方显式取消（请求 deadline/关停）：放弃等待，如实回错
		err = fmt.Errorf("bus blocking handler wait aborted: %w", ctx.Err())
	}
	b.recordDelivery(msg.Topic, domain, start, err)
	return err
}

// recordDelivery 投递指标（runHandler/runBlockingHandler 共用——G1-06）。
func (b *Bus) recordDelivery(topic string, domain Domain, start time.Time, err error) {
	b.metrics.BusDeliveryDuration.WithLabelValues(topic, string(domain)).Observe(b.clock.Now().Sub(start).Seconds())
	result := deliverResultSuccess
	if err != nil {
		result = deliverResultError
	}
	b.metrics.BusDelivered.WithLabelValues(topic, string(domain), result).Inc()
}

// enqueueBroadcast 本地段入队（Broadcast 域）；满丢弃并计数（§5.4 背压）。
func (b *Bus) enqueueBroadcast(msg Message) {
	if !b.q.enqueue(deliveryItem{msg: msg, domain: DomainBroadcast}) {
		b.metrics.BusDelivered.WithLabelValues(msg.Topic, string(DomainBroadcast), deliverResultDropped).Inc()
	}
}

// enqueueCluster 入队（Cluster 消费/降级段）；ack 联动随条目携带。
// 满丢弃不 ack → 条目留 PEL 由回收循环重投（不丢消息，受 MaxAttempts 约束）。
func (b *Bus) enqueueCluster(msg Message, ack *pendingAck) {
	if !b.q.enqueue(deliveryItem{msg: msg, domain: DomainCluster, ack: ack}) {
		b.metrics.BusDelivered.WithLabelValues(msg.Topic, string(DomainCluster), deliverResultDropped).Inc()
	}
}

// recordBroadcastLost 广播缺口丢失计数（接收侧断档；审计 A12-1）。
func (b *Bus) recordBroadcastLost(source string, n uint64) {
	b.metrics.BusBroadcastLost.WithLabelValues(source).Add(float64(n))
}

func (b *Bus) recordDiscarded(topic, entryID string) {
	b.metrics.BusDelivered.WithLabelValues(topic, string(DomainCluster), deliverResultDiscarded).Inc()
	b.logger.Warn("bus cluster message discarded after max attempts", "topic", topic, "id", entryID)
}

// buildHeaders 合并调用方自定义头并保证 request_id/trace_id 贯穿（缺失生成，
// G2 异步链路红线）；W3C traceparent 注入（OTel 与自定义 trace_id 双通道）；
// source 固定为本节点；principal 按 ctx 身份补齐（type:id 有损格式）。
func (b *Bus) buildHeaders(ctx context.Context, user map[string]string) map[string]string {
	_, reqID, traceID := observability.EnsureIDs(ctx, "")
	h := make(map[string]string, len(user)+5)
	for k, v := range user {
		h[k] = v
	}
	if reqID != "" {
		h[HeaderRequestID] = reqID
	}
	if traceID != "" {
		h[HeaderTraceID] = traceID
	}
	observability.InjectTraceHeaders(ctx, h)
	if p, ok := authn.PrincipalFrom(ctx); ok {
		h[HeaderPrincipal] = p.String()
	}
	h[HeaderSource] = b.cfg.NodeID
	return h
}

// headerCtx 将信封头注回 handler ctx（消费端日志/下游调用延续同一 trace；
// traceparent 提取—— 桥接语义见 observability.ExtractTraceContext；
// principal 回注还原传播身份——§11 全链路传递，有损对称只还原 Type/ID，
// 格式坏忽略不阻断：消息头是传播上下文不是凭证）。
func headerCtx(ctx context.Context, headers map[string]string) context.Context {
	if id := headers[HeaderRequestID]; id != "" {
		ctx = observability.WithRequestID(ctx, id)
	}
	if id := headers[HeaderTraceID]; id != "" {
		ctx = observability.WithTraceID(ctx, id)
	}
	ctx = observability.ExtractTraceContext(ctx, headers)
	if p, ok := authn.ParseString(headers[HeaderPrincipal]); ok {
		ctx = authn.WithPrincipal(ctx, p)
	}
	return ctx
}

// snapshot 返回 topic 订阅快照（按 priority 升序；相同 priority 保持注册序
// ——sort.SliceStable）。gate chan 随 entry 复制（同通道共享，闸门语义不变）。
func (b *Bus) snapshot(topic string) []subEntry {
	b.mu.RLock()
	list := b.subs[topic]
	if len(list) == 0 {
		b.mu.RUnlock()
		return nil
	}
	out := make([]subEntry, len(list))
	copy(out, list)
	b.mu.RUnlock()
	sort.SliceStable(out, func(i, j int) bool { return out[i].priority < out[j].priority })
	return out
}

// clusterTopics 当前有订阅的 topic 快照（消费/回收循环按轮取用；新 topic 拾取
// 延迟 ≤ 一个 BLOCK/轮询周期）。
func (b *Bus) clusterTopics() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	topics := make([]string, 0, len(b.subs))
	for t := range b.subs {
		topics = append(topics, t)
	}
	return topics
}

// validateTopic 校验 §5.5 命名：
// domain.<module>.<aggregate>.<action>（4 段）；
// platform.<subsystem>.<event>、integration.<system>.<event>（3 段）。
// 段字符 [a-z][a-z0-9_]*，总长 ≤ topicMaxLen。
func validateTopic(topic string) error {
	if topic == "" || len(topic) > topicMaxLen {
		return fmt.Errorf("%w: length must be 1..%d", ErrInvalidTopic, topicMaxLen)
	}
	var need int
	switch {
	case strings.HasPrefix(topic, prefixDomain):
		need = 4
	case strings.HasPrefix(topic, prefixPlatform), strings.HasPrefix(topic, prefixIntegration):
		need = 3
	default:
		return fmt.Errorf("%w: prefix must be %q/%q/%q", ErrInvalidTopic, prefixDomain, prefixPlatform, prefixIntegration)
	}
	segs := strings.Split(topic, ".")
	if len(segs) != need {
		return fmt.Errorf("%w: %s topics need %d segments, got %d", ErrInvalidTopic, segs[0], need, len(segs))
	}
	for _, s := range segs {
		if !topicSegRe.MatchString(s) {
			return fmt.Errorf("%w: segment %q must match %s", ErrInvalidTopic, s, topicSegRe.String())
		}
	}
	return nil
}

var topicSegRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
