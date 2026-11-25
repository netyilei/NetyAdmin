// alerts.go：观测页阈值评估（展示型告警——请求时评估，不落库/不推送；
// 后台告警引擎不在本版范围，外部 Prometheus 告警用同一批 na_* 指标）。
package observability

import "sort"

// Alert 是阈值命中项（观测页横幅）。
type Alert struct {
	Key       string  `json:"key"`
	Severity  string  `json:"severity"` // critical | warning
	Message   string  `json:"message"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
}

// AlertSeverity 枚举。
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
)

// DefaultThresholds 是告警最小集默认阈值（docs/rules/observability.md §6）。
// 0 值语义=“大于 0 即告警”（DLQ 类）；config.observability.alerts 可覆盖。
var DefaultThresholds = map[string]float64{
	"http_5xx_rate":            0.05, // 累计 5xx / 总请求
	"bus_dlq_depth":            0,
	"scheduler_dlq_depth":      0,
	"bus_outbox_pending_depth": 1000,
	"message_queue_depth":      48, // 进程内队列容量 64 的 75%
	"message_pending_records":  1000,
}

// MergeThresholds 合并配置覆盖值；返回未知键（启动期告警——避免“配置了
// 但阈值不生效”的静默项）。
func MergeThresholds(overrides map[string]float64) (map[string]float64, []string) {
	merged := make(map[string]float64, len(DefaultThresholds)+len(overrides))
	for k, v := range DefaultThresholds {
		merged[k] = v
	}
	var unknown []string
	for k, v := range overrides {
		if _, ok := DefaultThresholds[k]; !ok {
			unknown = append(unknown, k)
			continue
		}
		merged[k] = v
	}
	sort.Strings(unknown)
	return merged, unknown
}

// Evaluate 评估阈值命中项（按严重度+键排序，稳定输出）。
func Evaluate(s Summary, thresholds map[string]float64) []Alert {
	var alerts []Alert
	add := func(key, severity, msg string, value float64) {
		alerts = append(alerts, Alert{
			Key: key, Severity: severity, Message: msg,
			Value: value, Threshold: thresholds[key],
		})
	}

	if s.StateMode != "" && s.StateMode != "NORMAL" {
		add("state_mode", SeverityCritical, "子系统状态降级: "+s.StateMode, 1)
	}
	if s.HTTPRequestsTotal > 0 && s.HTTP5xxRate > thresholds["http_5xx_rate"] {
		add("http_5xx_rate", SeverityWarning, "HTTP 5xx 比例超阈值", s.HTTP5xxRate)
	}
	if s.BusDLQDepth > thresholds["bus_dlq_depth"] {
		add("bus_dlq_depth", SeverityCritical, "总线死信积压（需 ReplayDLQ 或人工处理）", s.BusDLQDepth)
	}
	if s.SchedulerDLQDepth > thresholds["scheduler_dlq_depth"] {
		add("scheduler_dlq_depth", SeverityCritical, "任务失败死信积压（taskcenter 可重放）", s.SchedulerDLQDepth)
	}
	if s.BusOutboxPending > thresholds["bus_outbox_pending_depth"] {
		add("bus_outbox_pending_depth", SeverityWarning, "Outbox 待发布积压（relay 跟不上/Redis 不可用）", s.BusOutboxPending)
	}
	if s.MessageQueueDepth > thresholds["message_queue_depth"] {
		add("message_queue_depth", SeverityWarning, "消息发送队列积压（worker 跟不上）", s.MessageQueueDepth)
	}
	if s.MessagePending > thresholds["message_pending_records"] {
		add("message_pending_records", SeverityWarning, "消息待发送/可重试记录积压", s.MessagePending)
	}

	sort.SliceStable(alerts, func(i, j int) bool {
		if alerts[i].Severity != alerts[j].Severity {
			return alerts[i].Severity == SeverityCritical
		}
		return alerts[i].Key < alerts[j].Key
	})
	return alerts
}
