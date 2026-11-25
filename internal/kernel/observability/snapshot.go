// snapshot.go：指标抓取面的 JSON 投影（admin 观测页数据源；不改变
// Prometheus 文本曝光面——/metrics 由 promhttp 独立提供）。
//
// 设计：Snapshot 走 prometheus.Gatherer（非默认 Registry），把 MetricFamily
// 平铺为 name/help/type/samples——admin 页面与下游自定义指标（na_<module>_*）
// 同一视图，无需引入 Prometheus 服务。
package observability

import (
	"math"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// MetricFamily 是一个指标族（JSON 投影）。
type MetricFamily struct {
	Name    string         `json:"name"`
	Help    string         `json:"help"`
	Type    string         `json:"type"` // counter|gauge|histogram|summary|untyped
	Samples []MetricSample `json:"samples"`
}

// MetricSample 是单个样本：Labels 为标签集；counter/gauge 用 Value；
// histogram/summary 额外带 Count/Sum（Value=Count，便于统一排序展示）；
// histogram 带 Buckets（p95 计算面）。
type MetricSample struct {
	Labels  map[string]string `json:"labels,omitempty"`
	Value   float64           `json:"value"`
	Count   uint64            `json:"count,omitempty"`
	Sum     float64           `json:"sum,omitempty"`
	Buckets []HistogramBucket `json:"buckets,omitempty"`
}

// Snapshot 抓取当前全部指标（按 name 排序，稳定输出）。
func (m *Metrics) Snapshot() ([]MetricFamily, error) {
	families, err := m.registry.Gather()
	if err != nil {
		return nil, err
	}
	out := make([]MetricFamily, 0, len(families))
	for _, mf := range families {
		fam := MetricFamily{
			Name: mf.GetName(),
			Help: mf.GetHelp(),
			Type: strings.ToLower(mf.GetType().String()),
		}
		for _, metric := range mf.GetMetric() {
			fam.Samples = append(fam.Samples, sampleOf(metric))
		}
		out = append(out, fam)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Register 注册自定义 Collector（模块/下游业务指标入口；替代裸
// Registry().MustRegister——重复注册返回错误而非 panic）。
//
// 下游用法（跨模块消费面）：kernel.Metrics 端口经 Registry Expose，
// ResolveAs[*observability.Metrics] 后 Register 自有 na_<module>_* 指标。
// 注册经统一 registerer——自动继承 node_id ConstLabel（多机可区分）。
func (m *Metrics) Register(c prometheus.Collector) error {
	return m.registerer.Register(c)
}

// sampleOf 取单条样本（dto.Metric 的 oneof 判定）。
func sampleOf(metric *dto.Metric) MetricSample {
	s := MetricSample{Labels: labelMap(metric.GetLabel())}
	switch {
	case metric.GetCounter() != nil:
		s.Value = metric.GetCounter().GetValue()
	case metric.GetGauge() != nil:
		s.Value = metric.GetGauge().GetValue()
	case metric.GetUntyped() != nil:
		s.Value = metric.GetUntyped().GetValue()
	case metric.GetHistogram() != nil:
		h := metric.GetHistogram()
		s.Count = h.GetSampleCount()
		s.Sum = h.GetSampleSum()
		s.Value = float64(s.Count)
		for _, b := range h.GetBucket() {
			le := b.GetUpperBound()
			// +Inf/NaN 上界不可 JSON 序列化（json 拒绝 Inf——曾致观测端点空响应）；
			// 总量由 Count/Sum 承载，桶只保留有限上界（+Inf 桶语义=无穷，无展示价值）。
			if math.IsInf(le, 0) || math.IsNaN(le) {
				continue
			}
			s.Buckets = append(s.Buckets, HistogramBucket{UpperBound: le, Count: b.GetCumulativeCount()})
		}
	case metric.GetSummary() != nil:
		sm := metric.GetSummary()
		s.Count = sm.GetSampleCount()
		s.Sum = sm.GetSampleSum()
		s.Value = float64(s.Count)
	}
	return s
}

// labelMap 标签对 → map（空标签返回 nil，JSON 省略）。
func labelMap(pairs []*dto.LabelPair) map[string]string {
	if len(pairs) == 0 {
		return nil
	}
	out := make(map[string]string, len(pairs))
	for _, p := range pairs {
		out[p.GetName()] = p.GetValue()
	}
	return out
}
