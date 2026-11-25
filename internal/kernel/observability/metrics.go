// metrics.go：Prometheus 指标——非默认 Registry（不污染全局），
// 命名遵循 docs/rules/namespaces.md（na_<subsystem>_<name>_<unit>；标签低基数）。
package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics 是 HTTP 接入面与 Bus 的必备指标最低集（docs/rules/observability.md §3）。
type Metrics struct {
	registry   *prometheus.Registry
	registerer prometheus.Registerer // 含 node_id ConstLabel 的注册面（Register 共用）

	HTTPDuration  *prometheus.HistogramVec // na_http_request_duration_seconds[module,type,method,code]
	RequestsTotal *prometheus.CounterVec   // na_http_requests_total[module,type,method,code]
	InFlight      prometheus.Gauge         // na_http_in_flight_requests

	BusPublished        *prometheus.CounterVec   // na_bus_published_total[topic,domain]
	BusDelivered        *prometheus.CounterVec   // na_bus_delivered_total[topic,domain,result]（success/error/dropped/discarded）
	BusDeliveryDuration *prometheus.HistogramVec // na_bus_delivery_duration_seconds[topic,domain]
	BusE2ELatency       *prometheus.HistogramVec // na_bus_e2e_latency_seconds[domain]（handler 开始-OccurredAt；observability.md §3）
	BusDLQDepth         prometheus.Gauge         // na_bus_dlq_depth（；§3 Bus 行 DLQ 深度，>0 告警）
	BusOutboxPending    prometheus.Gauge         // na_bus_outbox_pending_depth（outbox 待发布积压，审计 O1）
	BusBroadcastLost    *prometheus.CounterVec   // na_bus_broadcast_lost_total[source]（缺口检测：断连/重启窗口丢失，审计 A12-1）

	SchedulerTicks       *prometheus.CounterVec   // na_scheduler_ticks_total[job]
	SchedulerStarted     *prometheus.CounterVec   // na_scheduler_started_total[job]
	SchedulerSucceeded   *prometheus.CounterVec   // na_scheduler_succeeded_total[job]
	SchedulerFailed      *prometheus.CounterVec   // na_scheduler_failed_total[job]（重试穷尽）
	SchedulerRetrying    *prometheus.CounterVec   // na_scheduler_retrying_total[job]
	SchedulerSkipped     *prometheus.CounterVec   // na_scheduler_skipped_total[job,reason]（paused/not-leader/overlap/worker-full）
	SchedulerRunDuration *prometheus.HistogramVec // na_scheduler_run_duration_seconds[job]
	SchedulerStartLag    *prometheus.HistogramVec // na_scheduler_start_lag_seconds[job]
	SchedulerDLQDepth    prometheus.Gauge         // na_scheduler_dlq_depth（failed 穷尽行数=可重放死信量）

	// —— 补齐（observability.md §3 最低集：DB / Cache / Coordination / Session） ——

	DBQueryDuration prometheus.Histogram     // na_db_query_duration_seconds（连接级 SQL：Query/QueryRow/Exec）
	DBErrors        *prometheus.CounterVec   // na_db_errors_total[op]（query|commit|rollback——begin 失败=事务未生效，归 rollback）
	DBTxDuration    *prometheus.HistogramVec // na_db_tx_duration_seconds[result]（commit/rollback）

	CacheHits    *prometheus.CounterVec // na_cache_hits_total[entity]
	CacheMisses  *prometheus.CounterVec // na_cache_misses_total[entity]
	CacheDemoted *prometheus.CounterVec // na_cache_demoted_total[entity]（命中率<70% 自动降级 shared）

	// generation 失效收敛面（L2 广播是拉模式：本节点下次 gen 快照刷新才收敛，
	// 窗口=genSnapshotTTL 1s——两计数对照观测收敛是否发生/频率）。
	CacheInvalidate *prometheus.CounterVec // na_cache_invalidate_total[entity]（L2 generation INCR 广播计数）
	CacheGenDrift   *prometheus.CounterVec // na_cache_generation_drift_total[entity]（拉到 gen 与本地版本不一致=外部失效收敛事件计数）

	CoordFallbacks  *prometheus.CounterVec // na_coordination_fallback_total[primitive]
	CoordFailClosed *prometheus.CounterVec // na_coordination_fail_closed_total[name]（name=原语名或 ratelimit:<policy>；multi 拒绝）
	CoordFailOpen   *prometheus.CounterVec // na_coordination_fail_open_total[policy]（限流 fail_open 放行——告警面）

	SessionDBFallbacks *prometheus.CounterVec // na_session_db_fallback_total[face]（revocation/session_store 读回退）
	SessionFailClosed  *prometheus.CounterVec // na_session_fail_closed_total[face]（multi 拒绝——10010 语义）

	// ——（observability.md §3 RPC 行；na_rpc_* 兑现）——

	RPCCallsTotal   *prometheus.CounterVec   // na_rpc_calls_total[module,port,method,side,code]（side=server|client；code=HTTP 码组）
	RPCDuration     *prometheus.HistogramVec // na_rpc_duration_seconds[module,port,method,side]
	RPCCircuitState *prometheus.GaugeVec     // na_rpc_circuit_state[base_url]（0=closed 1=half-open 2=open；reliability §3 熔断态必须出指标）

	// ——（ws-protocol §11 六指标；na_realtime_*）——

	RealtimeConns         *prometheus.GaugeVec   // na_realtime_connections[surface]（当前连接数）
	RealtimeMessages      *prometheus.CounterVec // na_realtime_messages_total[surface,direction,type]（direction=in|out）
	RealtimeDropped       *prometheus.CounterVec // na_realtime_dropped_total[surface,reason]（slow_consumer|send_blocked）
	RealtimeClose         *prometheus.CounterVec // na_realtime_close_total[surface,code]（关闭原因分布）
	RealtimeSubscribeDeny *prometheus.CounterVec // na_realtime_subscribe_denied_total[surface]（ACL 拒绝）
	RealtimeTickets       *prometheus.CounterVec // na_realtime_ticket_issued_total[surface]（ticket 签发）
}

// NewMetrics 创建并注册指标集。
//
// nodeID 非空时作为 ConstLabel `node_id` 注入全部指标（含后续 Register 的
// 模块/下游自定义 collector）——multi-node-scaling.md §8「每节点指标带
// node_id」：多副本抓取时可按节点区分/聚合；空=不注入（单元测试面）。
func NewMetrics(nodeID string) *Metrics {
	reg := prometheus.NewRegistry()
	registerer := prometheus.Registerer(reg)
	if nodeID != "" {
		registerer = prometheus.WrapRegistererWith(prometheus.Labels{"node_id": nodeID}, reg)
	}
	m := &Metrics{registry: reg, registerer: registerer}
	m.HTTPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "na_http_request_duration_seconds",
		Help:    "HTTP 请求时长（服务端，含中间件全链路；slo.md §3 口径）",
		Buckets: []float64{.005, .01, .025, .05, .1, .2, .5, 1, 2.5, 5, 10},
	}, []string{"module", "type", "method", "code"})
	m.RequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_http_requests_total",
		Help: "HTTP 请求计数",
	}, []string{"module", "type", "method", "code"})
	m.InFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "na_http_in_flight_requests",
		Help: "在途请求数",
	})
	m.BusPublished = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_bus_published_total",
		Help: "Bus 发布计数（按 topic 与发布域）",
	}, []string{"topic", "domain"})
	m.BusDelivered = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_bus_delivered_total",
		Help: "Bus 投递计数（success/error/dropped=队列满丢弃/discarded=重试上限丢弃）",
	}, []string{"topic", "domain", "result"})
	m.BusDeliveryDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "na_bus_delivery_duration_seconds",
		Help:    "Bus handler 执行时长",
		Buckets: []float64{.005, .01, .025, .05, .1, .2, .5, 1, 2, 5},
	}, []string{"topic", "domain"})
	m.BusE2ELatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "na_bus_e2e_latency_seconds",
		Help:    "Bus 端到端延迟（handler 开始-OccurredAt；跨节点含时钟偏差，趋势观测口径）",
		Buckets: []float64{.005, .01, .025, .05, .1, .2, .5, 1, 2, 5, 10},
	}, []string{"domain"})
	m.BusDLQDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "na_bus_dlq_depth",
		Help: "Bus 死信表深度（>0 需人工处理：修复后 ReplayDLQ）",
	})
	m.BusOutboxPending = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "na_bus_outbox_pending_depth",
		Help: "Bus Outbox 待发布积压行数（持续增长=relay 跟不上或 Redis 不可用）",
	})
	m.BusBroadcastLost = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_bus_broadcast_lost_total",
		Help: "Broadcast 缺口检测丢失计数（per-publisher 序列号断档；§5.4 可丢弃并计数）",
	}, []string{"source"})
	m.SchedulerTicks = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_scheduler_ticks_total",
		Help: "Scheduler 调度触达计数",
	}, []string{"job"})
	m.SchedulerStarted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_scheduler_started_total",
		Help: "Scheduler 执行开始计数",
	}, []string{"job"})
	m.SchedulerSucceeded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_scheduler_succeeded_total",
		Help: "Scheduler 执行成功计数",
	}, []string{"job"})
	m.SchedulerFailed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_scheduler_failed_total",
		Help: "Scheduler 重试穷尽计数（TriggerNow 可重放）",
	}, []string{"job"})
	m.SchedulerRetrying = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_scheduler_retrying_total",
		Help: "Scheduler 进入重试等待计数",
	}, []string{"job"})
	m.SchedulerSkipped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_scheduler_skipped_total",
		Help: "Scheduler 触发跳过计数（paused/not-leader/overlap/worker-full）",
	}, []string{"job", "reason"})
	m.SchedulerRunDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "na_scheduler_run_duration_seconds",
		Help:    "Scheduler 单次执行时长",
		Buckets: []float64{.05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60, 300},
	}, []string{"job"})
	m.SchedulerStartLag = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "na_scheduler_start_lag_seconds",
		Help:    "Scheduler 计划到实际开始的延迟",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"job"})
	m.SchedulerDLQDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "na_scheduler_dlq_depth",
		Help: "Scheduler 失败穷尽运行数（运行记录即死信；TriggerNow 重放）",
	})
	m.DBQueryDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "na_db_query_duration_seconds",
		Help:    "DB 单查询时长（pgx tracer；reliability.md 基线 5s）",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 5},
	})
	m.DBErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_db_errors_total",
		Help: "DB 查询/事务错误计数",
	}, []string{"op"})
	m.DBTxDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "na_db_tx_duration_seconds",
		Help:    "事务总时长（begin→commit/rollback；reliability.md 事务 5s 基线）",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"result"})
	m.CacheHits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_cache_hits_total",
		Help: "Cache 命中计数（按 entity；admission.auto_demote 评估输入）",
	}, []string{"entity"})
	m.CacheMisses = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_cache_misses_total",
		Help: "Cache 未命中计数（含负缓存命中）",
	}, []string{"entity"})
	m.CacheDemoted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_cache_demoted_total",
		Help: "hot_local→shared 自动降级计数（命中率<70%，state-policies admission）",
	}, []string{"entity"})
	m.CacheInvalidate = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_cache_invalidate_total",
		Help: "Cache 失效广播计数（L2 generation INCR；无 L2 本地失效不计数）",
	}, []string{"entity"})
	m.CacheGenDrift = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_cache_generation_drift_total",
		Help: "Cache generation 漂移收敛计数（拉取到与本地不一致的 gen=外部失效已收敛；恒 0=广播未到达/未失效）",
	}, []string{"entity"})
	m.CoordFallbacks = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_coordination_fallback_total",
		Help: "Coordination 原语 single 拓扑本地回退计数（恢复路径告警输入）",
	}, []string{"primitive"})
	m.CoordFailClosed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_coordination_fail_closed_total",
		Help: "Coordination 原语/限流策略 fail_closed 拒绝计数（multi Redis 不可用）",
	}, []string{"name"})
	m.CoordFailOpen = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_coordination_fail_open_total",
		Help: "限流策略 fail_open 放行计数（Redis 不可用仍放行——告警面）",
	}, []string{"policy"})
	m.SessionDBFallbacks = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_session_db_fallback_total",
		Help: "Session 平面 DB 读回退计数（single 拓扑 Redis 不可用/未命中）",
	}, []string{"face"})
	m.SessionFailClosed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_session_fail_closed_total",
		Help: "Session 平面 fail_closed 拒绝计数（multi Redis 不可用→调用方 10010）",
	}, []string{"face"})
	m.RPCCallsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_rpc_calls_total",
		Help: "内部 RPC 调用计数（server=服务端受理 / client=客户端发起；code=2xx/4xx/5xx/net 组）",
	}, []string{"module", "port", "method", "side", "code"})
	m.RPCDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "na_rpc_duration_seconds",
		Help:    "内部 RPC 执行时长（server=handler 全链路 / client=单次调用含网络）",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"module", "port", "method", "side"})
	m.RPCCircuitState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "na_rpc_circuit_state",
		Help: "RPC 客户端熔断器状态（0=closed 1=half-open 2=open；reliability.md §3）",
	}, []string{"base_url"})
	m.RealtimeConns = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "na_realtime_connections",
		Help: "WS 当前连接数（按 surface；ws-protocol §11）",
	}, []string{"surface"})
	m.RealtimeMessages = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_realtime_messages_total",
		Help: "WS 消息计数（in=客户端上行 / out=服务端下行；type=帧类型）",
	}, []string{"surface", "direction", "type"})
	m.RealtimeDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_realtime_dropped_total",
		Help: "WS 丢弃/慢消费者计数（reason=slow_consumer/send_blocked）",
	}, []string{"surface", "reason"})
	m.RealtimeClose = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_realtime_close_total",
		Help: "WS 关闭计数（code=关闭码分布；ws-protocol §5）",
	}, []string{"surface", "code"})
	m.RealtimeSubscribeDeny = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_realtime_subscribe_denied_total",
		Help: "WS 订阅 ACL 拒绝计数（ws-protocol §11）",
	}, []string{"surface"})
	m.RealtimeTickets = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "na_realtime_ticket_issued_total",
		Help: "WS ticket 签发计数（按 surface）",
	}, []string{"surface"})
	registerer.MustRegister(m.HTTPDuration, m.RequestsTotal, m.InFlight,
		m.RPCCallsTotal, m.RPCDuration, m.RPCCircuitState,
		m.BusPublished, m.BusDelivered, m.BusDeliveryDuration, m.BusE2ELatency, m.BusDLQDepth, m.BusOutboxPending, m.BusBroadcastLost,
		m.SchedulerTicks, m.SchedulerStarted, m.SchedulerSucceeded, m.SchedulerFailed,
		m.SchedulerRetrying, m.SchedulerSkipped, m.SchedulerRunDuration, m.SchedulerStartLag, m.SchedulerDLQDepth,
		m.DBQueryDuration, m.DBErrors, m.DBTxDuration,
		m.CacheHits, m.CacheMisses, m.CacheDemoted, m.CacheInvalidate, m.CacheGenDrift,
		m.CoordFallbacks, m.CoordFailClosed, m.CoordFailOpen,
		m.SessionDBFallbacks, m.SessionFailClosed,
		m.RealtimeConns, m.RealtimeMessages, m.RealtimeDropped, m.RealtimeClose,
		m.RealtimeSubscribeDeny, m.RealtimeTickets)
	return m
}

// Registry 返回指标注册表（/metrics 端点挂载用）。
func (m *Metrics) Registry() *prometheus.Registry { return m.registry }
