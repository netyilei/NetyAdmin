// summary.go：指标快照 → 观测页概览（纯函数，无 IO；admin 端点与测试共用）。
package observability

import (
	"math"
	"sort"
	"strconv"
)

// HistogramBucket 是 histogram 桶（累计计数；UpperBound=le，含 +Inf）。
type HistogramBucket struct {
	UpperBound float64 `json:"le"`
	Count      uint64  `json:"count"`
}

// Summary 是观测页概览（计数器口径=进程启动以累计；页面标注"累计"）。
// StateMode 由调用方注入（health.ResolveMode——指标本身不含）。
type Summary struct {
	HTTPRequestsTotal float64 `json:"http_requests_total"`
	HTTP5xxTotal      float64 `json:"http_5xx_total"`
	HTTP5xxRate       float64 `json:"http_5xx_rate"` // 累计 5xx / 总请求（分母 0=0）
	HTTPP95MS         float64 `json:"http_p95_ms"`
	HTTPInFlight      float64 `json:"http_in_flight"`

	BusDLQDepth       float64 `json:"bus_dlq_depth"`
	BusOutboxPending  float64 `json:"bus_outbox_pending_depth"`
	SchedulerDLQDepth float64 `json:"scheduler_dlq_depth"`
	RealtimeConns     float64 `json:"realtime_connections"`

	MessageQueueDepth float64 `json:"message_queue_depth"`
	MessagePending    float64 `json:"message_pending_records"`
	MessageFailed     float64 `json:"message_send_failed_total"`

	StateMode string `json:"state_mode,omitempty"`
}

// Summarize 从指标快照汇总概览（缺族=0，不报错——模块禁用/零流量即零值）。
func Summarize(families []MetricFamily) Summary {
	idx := make(map[string]*MetricFamily, len(families))
	for i := range families {
		idx[families[i].Name] = &families[i]
	}
	var s Summary

	if fam := idx["na_http_requests_total"]; fam != nil {
		for _, smp := range fam.Samples {
			s.HTTPRequestsTotal += smp.Value
			if is5xx(smp.Labels["code"]) {
				s.HTTP5xxTotal += smp.Value
			}
		}
	}
	if s.HTTPRequestsTotal > 0 {
		s.HTTP5xxRate = s.HTTP5xxTotal / s.HTTPRequestsTotal
	}
	if fam := idx["na_http_in_flight_requests"]; fam != nil && len(fam.Samples) > 0 {
		s.HTTPInFlight = fam.Samples[0].Value
	}
	if fam := idx["na_http_request_duration_seconds"]; fam != nil {
		s.HTTPP95MS = p95Millis(fam.Samples)
	}
	s.BusDLQDepth = firstValue(idx["na_bus_dlq_depth"])
	s.BusOutboxPending = firstValue(idx["na_bus_outbox_pending_depth"])
	s.SchedulerDLQDepth = firstValue(idx["na_scheduler_dlq_depth"])
	s.RealtimeConns = sumValues(idx["na_realtime_connections"])
	s.MessageQueueDepth = firstValue(idx["na_message_queue_depth"])
	s.MessagePending = sumValues(idx["na_message_pending_records"])
	s.MessageFailed = sumValues(idx["na_message_send_failed_total"])
	return s
}

// is5xx HTTP 状态标签判定（标签值=数字状态码串）。
func is5xx(code string) bool {
	n, err := strconv.Atoi(code)
	return err == nil && n >= 500
}

// firstValue 单值族（gauge/单样本 counter）取值。
func firstValue(fam *MetricFamily) float64 {
	if fam == nil || len(fam.Samples) == 0 {
		return 0
	}
	return fam.Samples[0].Value
}

// sumValues 多标签族求和（realtime 按 surface / message 按 channel）。
func sumValues(fam *MetricFamily) float64 {
	if fam == nil {
		return 0
	}
	var sum float64
	for _, smp := range fam.Samples {
		sum += smp.Value
	}
	return sum
}

// p95Millis 从 histogram 桶计算 p95（毫秒；线性近似=取命中桶上界）。
// 历史为空（所有 count=0）=0；p95 落在 +Inf 桶（观测值超过最大有限桶）=
// 饱和为最大有限桶上界（页面显示"≥ 该值"，且不产生 +Inf——JSON 不可序列化）。
func p95Millis(samples []MetricSample) float64 {
	var total uint64
	var buckets []HistogramBucket
	for _, smp := range samples {
		total += smp.Count
		for _, b := range smp.Buckets {
			if math.IsInf(b.UpperBound, 0) || math.IsNaN(b.UpperBound) {
				continue
			}
			buckets = append(buckets, b)
		}
	}
	if total == 0 || len(buckets) == 0 {
		return 0
	}
	rank := 0.95 * float64(total)
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].UpperBound < buckets[j].UpperBound })
	for _, b := range buckets {
		if float64(b.Count) >= rank {
			return b.UpperBound * 1000
		}
	}
	// 排名落在 +Inf 桶：饱和到最大有限上界（不再返回 +Inf）。
	return buckets[len(buckets)-1].UpperBound * 1000
}
