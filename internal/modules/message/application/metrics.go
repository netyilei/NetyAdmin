// metrics.go：message 模块发送链路指标（admin 观测页与 /metrics 同源）。
//
// 三个指标（命名 observability.md §3：na_<subsystem>_<name>_<unit>）：
//   - na_message_queue_depth：进程内发送队列深度（单队列，无 channel 维度；
//     满=worker 跟不上，记录已落 pending 由 retry Job 重投）；
//   - na_message_pending_records{channel}：待发送/可重试记录数（scrape 时查库，
//     DB 为真相源——进程重启不漂移；走 (channel,status) 索引）；
//   - na_message_send_failed_total{channel}：投递失败计数（驱动错误/无可用驱动；
//     含可重试与重试穷尽）。
package application

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/modules/message/domain"
)

// 指标名常量（观测页/告警/外部采集共用；改名单点）。
const (
	MetricQueueDepth      = "na_message_queue_depth"
	MetricPendingRecords  = "na_message_pending_records"
	MetricSendFailedTotal = "na_message_send_failed_total"
)

// 观测页/采集面已知通道（0 值也输出——缺族比 0 值更容易误判；取值同 domain 常量）。
var knownChannels = []string{string(domain.ChannelEmail), string(domain.ChannelSms)}

// Metrics 是发送链路指标集（nil=零开销——测试/模块禁用）。
type Metrics struct {
	queueDepth prometheus.Gauge
	sendFailed *prometheus.CounterVec
}

// NewMetrics 注册队列深度与失败计数（pending 由 Attach 期挂 Collector——
// 依赖 repo）。重复注册返回错误（进程内单实例）。
func NewMetrics(reg *observability.Metrics) (*Metrics, error) {
	queue := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricQueueDepth,
		Help: "消息发送队列深度（进程内单队列；满=worker 跟不上，记录由 retry Job 重投）",
	})
	failed := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: MetricSendFailedTotal,
		Help: "消息投递失败计数（驱动错误或无可用驱动；含可重试与重试穷尽）",
	}, []string{"channel"})
	if err := reg.Register(queue); err != nil {
		return nil, fmt.Errorf("message: register %s: %w", MetricQueueDepth, err)
	}
	if err := reg.Register(failed); err != nil {
		return nil, fmt.Errorf("message: register %s: %w", MetricSendFailedTotal, err)
	}
	// 预创建已知通道 0 值序列（CounterVec 无子项时不出现在 /metrics——
	// 采集侧 rate() 需要起始序列；首次失败后同样命中该子项）。
	for _, channel := range knownChannels {
		failed.WithLabelValues(channel)
	}
	return &Metrics{queueDepth: queue, sendFailed: failed}, nil
}

// SetQueueDepth 更新队列深度（enqueue/出队两处调用）。
func (m *Metrics) SetQueueDepth(n int) {
	if m != nil {
		m.queueDepth.Set(float64(n))
	}
}

// IncSendFailed 记录一次投递失败（channel 维度）。
func (m *Metrics) IncSendFailed(channel string) {
	if m != nil {
		m.sendFailed.WithLabelValues(channel).Inc()
	}
}

// PendingCounter 是待发送/可重试记录计数源（adapters 实现；测试可注入）。
type PendingCounter func(ctx context.Context) (map[string]int64, error)

// pendingCollector 在 scrape 时查询待发送记录数（DB 为真相源；15s 结果缓存）。
// DB 故障=本周期不输出并记 error 日志（不伪造 0——缺失比假 0 诚实）。
type pendingCollector struct {
	count   PendingCounter
	log     *slog.Logger
	desc    *prometheus.Desc
	timeout time.Duration

	mu        sync.Mutex
	cached    map[string]int64
	fetchedAt time.Time
	now       func() time.Time // 时钟注入（测试用；nil=time.Now）
}

// pendingCacheTTL 是待发计数缓存有效期（Prometheus 抓取间隔通常 15-60s；
// 观测页自动刷新 30s——缓存把 DB 查询压到 ≤1 次/TTL）。
const pendingCacheTTL = 15 * time.Second

// NewPendingCollector 创建挂载到共享 registry 的 pending 采集器。
func NewPendingCollector(count PendingCounter, log *slog.Logger) prometheus.Collector {
	return &pendingCollector{
		count:   count,
		log:     log,
		timeout: 3 * time.Second,
		desc: prometheus.NewDesc(MetricPendingRecords,
			"消息待发送/可重试记录数（status=pending 或 failed 且未达重试上限）",
			[]string{"channel"}, nil),
	}
}

// Describe 实现 prometheus.Collector。
func (c *pendingCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

// Collect 实现 prometheus.Collector。
func (c *pendingCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	counts, err := c.counts(ctx)
	if err != nil {
		if c.log != nil {
			c.log.Error("message: pending records count failed (metric skipped)", "err", err)
		}
		return
	}
	// 已知通道补 0 + 排序（稳定输出；未知通道（历史数据）也输出）。
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, float64(counts[name]), name)
	}
}

// counts 取待发计数（带 TTL 缓存：/metrics 抓取与观测页高频刷新不放大 DB
// 压力；陈旧上界=pendingCacheTTL。缓存只存成功结果——故障下不伪造 0）。
// 查询在锁外执行（DB 慢时不阻塞其他 scrape 读缓存；并发重复查可接受）。
func (c *pendingCollector) counts(ctx context.Context) (map[string]int64, error) {
	now := time.Now
	if c.now != nil {
		now = c.now
	}
	c.mu.Lock()
	if c.cached != nil && now().Sub(c.fetchedAt) < pendingCacheTTL {
		cached := c.cached
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	counts, err := c.count(ctx)
	if err != nil {
		return nil, err
	}
	// 已知通道补 0（缺族比 0 值更易误判）。
	for _, name := range knownChannels {
		if _, ok := counts[name]; !ok {
			counts[name] = 0
		}
	}
	c.mu.Lock()
	c.cached = counts
	c.fetchedAt = now()
	c.mu.Unlock()
	return counts, nil
}
