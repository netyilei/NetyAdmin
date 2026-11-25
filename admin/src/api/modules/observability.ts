// 观测面 API（cmd/netyadmin/observability.go：指标概览 + 指标浏览器；
// 数据源=进程内 Prometheus registry——含基座 na_* 与下游自注册 na_<module>_*。
// 信封由 axios 拦截器解包，调用方取 data）。
import http from "@/api";

/** 指标样本（counter/gauge 用 value；histogram 带 count/sum/buckets）。 */
export interface MetricSample {
  labels?: Record<string, string>;
  value: number;
  count?: number;
  sum?: number;
  buckets?: { le: number; count: number }[];
}

/** 指标族（Prometheus family 投影）。 */
export interface MetricFamily {
  name: string;
  help: string;
  type: string;
  samples: MetricSample[];
}

/** 概览（累计口径 + 运行期状态模式）。 */
export interface ObservabilitySummary {
  http_requests_total: number;
  http_5xx_total: number;
  http_5xx_rate: number;
  http_p95_ms: number;
  http_in_flight: number;
  bus_dlq_depth: number;
  bus_outbox_pending_depth: number;
  scheduler_dlq_depth: number;
  realtime_connections: number;
  message_queue_depth: number;
  message_pending_records: number;
  message_send_failed_total: number;
  state_mode?: string;
}

/** 阈值告警项（展示型——页面请求时评估）。 */
export interface ObservabilityAlert {
  key: string;
  severity: string; // critical | warning
  message: string;
  value: number;
  threshold: number;
}

export interface ObservabilityOverview {
  /** 本节点标识（多机部署下页面=本节点视图）。 */
  node_id: string;
  summary: ObservabilitySummary;
  window: { qps: number; http_5xx_rate: number; windowed: boolean };
  alerts: ObservabilityAlert[];
  thresholds: Record<string, number>;
}

export interface ObservabilityMetrics {
  families: MetricFamily[];
  total: number;
  truncated: boolean;
}

export const observabilityApi = {
  overview: () => http.get<ObservabilityOverview>("/admin/v1/system/observability/overview", undefined, { loading: false }),
  metrics: (family: string, limit = 200) =>
    http.get<ObservabilityMetrics>("/admin/v1/system/observability/metrics", { family, limit }, { loading: false })
};
