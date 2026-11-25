<script setup lang="ts" name="observability">
import { computed, onActivated, onDeactivated, onMounted, onUnmounted, ref } from "vue";
import {
  observabilityApi,
  type MetricFamily,
  type ObservabilityAlert,
  type ObservabilityOverview
} from "@/api/modules/observability";

const overview = ref<ObservabilityOverview>();
const families = ref<MetricFamily[]>([]);
const loadingOverview = ref(false);
const loadingMetrics = ref(false);
const metricsTruncated = ref(false);
const family = ref("");
const autoRefresh = ref(false);
let timer: ReturnType<typeof setInterval> | undefined;

// silent=true 用于 30s 自动刷新：不闪 v-loading（避免页面每半分钟抖动）。
const loadOverview = async (silent = false) => {
  if (!silent) loadingOverview.value = true;
  try {
    const { data } = await observabilityApi.overview();
    overview.value = data;
  } finally {
    if (!silent) loadingOverview.value = false;
  }
};

const loadMetrics = async (silent = false) => {
  if (!silent) loadingMetrics.value = true;
  try {
    const { data } = await observabilityApi.metrics(family.value.trim());
    families.value = data.families ?? [];
    metricsTruncated.value = Boolean(data.truncated);
  } finally {
    if (!silent) loadingMetrics.value = false;
  }
};

const refreshAll = async (silent = false) => {
  await Promise.all([loadOverview(silent), loadMetrics(silent)]);
};

// 轮询启停：动态视图恒开 keep-alive（tab 切换不触发 onUnmounted），
// 必须用 onActivated/onDeactivated 管定时器生命周期；自动刷新同时覆盖
// 概览与指标浏览器（pending 采集侧带 15s 缓存，不会放大 DB 压力）。
const startTimer = () => {
  stopTimer();
  timer = setInterval(() => {
    if (autoRefresh.value) refreshAll(true);
  }, 30000);
};
const stopTimer = () => {
  if (timer) clearInterval(timer);
  timer = undefined;
};
// el-switch change 回调入参为 string | number | boolean（Element Plus 类型定义）。
const onAutoRefreshChange = (val: string | number | boolean) => {
  const on = Boolean(val);
  autoRefresh.value = on;
  if (on) {
    loadOverview();
    startTimer();
  } else {
    stopTimer();
  }
};

onMounted(refreshAll);
onActivated(() => {
  if (autoRefresh.value) startTimer();
});
onDeactivated(stopTimer);
onUnmounted(stopTimer);

const numText = (v: number): string => {
  if (!Number.isFinite(v)) return "—";
  if (Math.abs(v) >= 1_000_000) return `${(v / 1_000_000).toFixed(2)}M`;
  if (Math.abs(v) >= 1_000) return `${(v / 1_000).toFixed(2)}K`;
  return String(Math.round(v * 100) / 100);
};

const labelText = (labels?: Record<string, string>): string => {
  if (!labels) return "—";
  return Object.keys(labels)
    .sort()
    .map(k => `${k}=${labels[k]}`)
    .join(", ");
};

const alertSeverity = (key: string): string => {
  const hit = overview.value?.alerts?.find(a => a.key === key);
  if (!hit) return "";
  return hit.severity === "critical" ? "danger" : "warning";
};

// 告警文案：数值型阈值附“当前/阈值”；状态类（state_mode）无阈值语义，只显示消息。
const alertText = (alert: ObservabilityAlert): string =>
  alert.key === "state_mode"
    ? alert.message
    : `${alert.message}（当前 ${numText(alert.value)}，阈值 ${numText(alert.threshold)}）`;

const cards = computed(() => {
  const s = overview.value?.summary;
  if (!s) return [];
  const windowed = Boolean(overview.value?.window.windowed);
  return [
    { key: "qps", label: "QPS（本节点·窗口）", value: windowed ? overview.value!.window.qps.toFixed(2) : "—", level: "" },
    {
      key: "http_5xx_rate",
      label: windowed ? "5xx 比例（本节点·窗口）" : "5xx 比例（本节点·累计）",
      // 告警用窗口口径（有流量时）——展示同口径，避免“卡片低但已告警”的错位。
      value: `${((windowed ? overview.value!.window.http_5xx_rate : s.http_5xx_rate) * 100).toFixed(2)}%`,
      level: alertSeverity("http_5xx_rate")
    },
    { key: "p95", label: "P95 延迟（本节点）", value: `${s.http_p95_ms.toFixed(0)} ms`, level: "" },
    { key: "inflight", label: "在途请求（本节点）", value: numText(s.http_in_flight), level: "" },
    { key: "bus_dlq_depth", label: "Bus 死信（集群）", value: numText(s.bus_dlq_depth), level: alertSeverity("bus_dlq_depth") },
    {
      key: "bus_outbox_pending_depth",
      label: "Outbox 积压（集群）",
      value: numText(s.bus_outbox_pending_depth),
      level: alertSeverity("bus_outbox_pending_depth")
    },
    {
      key: "scheduler_dlq_depth",
      label: "任务死信（集群）",
      value: numText(s.scheduler_dlq_depth),
      level: alertSeverity("scheduler_dlq_depth")
    },
    { key: "realtime", label: "实时连接（本节点）", value: numText(s.realtime_connections), level: "" },
    {
      key: "message_queue_depth",
      label: "消息队列（本节点）",
      value: numText(s.message_queue_depth),
      level: alertSeverity("message_queue_depth")
    },
    {
      key: "message_pending_records",
      label: "消息待发（集群）",
      value: numText(s.message_pending_records),
      level: alertSeverity("message_pending_records")
    },
    { key: "message_failed", label: "发送失败（本节点·累计）", value: numText(s.message_send_failed_total), level: "" },
    {
      key: "state_mode",
      label: "状态模式（本节点）",
      value: s.state_mode || "—",
      level: s.state_mode && s.state_mode !== "NORMAL" ? "danger" : ""
    }
  ];
});

type MetricRow = { family: string; type: string; labels: string; value: string };

// 指标样本扁平化（上限 500 行防大标签族卡顿；可用 family 前缀过滤收敛）。
const metricRows = computed<MetricRow[]>(() => {
  const rows: MetricRow[] = [];
  for (const fam of families.value) {
    for (const smp of fam.samples) {
      rows.push({ family: fam.name, type: fam.type, labels: labelText(smp.labels), value: numText(smp.value) });
      if (rows.length >= 500) return rows;
    }
  }
  return rows;
});

// 样本总数（与表格行数对照——超过 500 行时提示用前缀过滤，避免“以为看全了”）。
const sampleTotal = computed(() => families.value.reduce((n, fam) => n + fam.samples.length, 0));
const rowsTruncated = computed(() => sampleTotal.value > metricRows.value.length);

const cardTagType = (level: string): "primary" | "success" | "warning" | "danger" | "info" => {
  if (level === "danger") return "danger";
  if (level === "warning") return "warning";
  return "info";
};
</script>

<template>
  <div class="observability-page">
    <el-card shadow="never">
      <div class="page-header">
        <div class="page-title">
          可观测
          <span class="node-tip"> 节点：{{ overview?.node_id || "—" }}（本页=本节点视图；标「集群」的为数据库全局值） </span>
        </div>
        <div class="page-actions">
          <el-input
            v-model="family"
            placeholder="指标名前缀，如 na_message"
            clearable
            style="width: 240px"
            @keyup.enter="loadMetrics()"
          />
          <el-button :loading="loadingMetrics" @click="loadMetrics()">查询指标</el-button>
          <span class="auto-refresh-label">自动刷新（30s）</span>
          <el-switch :model-value="autoRefresh" @change="onAutoRefreshChange" />
          <el-button type="primary" :loading="loadingOverview" @click="loadOverview()">刷新概览</el-button>
        </div>
      </div>

      <div v-if="overview?.alerts?.length" class="alert-list">
        <el-alert
          v-for="alert in overview.alerts"
          :key="alert.key"
          :type="alert.severity === 'critical' ? 'error' : 'warning'"
          :closable="false"
          show-icon
          :title="alertText(alert)"
        />
      </div>
      <el-alert v-else type="success" :closable="false" show-icon title="无阈值告警" class="alert-list" />
    </el-card>

    <el-card v-loading="loadingOverview" shadow="never" class="section-card">
      <el-row :gutter="12">
        <el-col v-for="card in cards" :key="card.key" :xs="12" :sm="8" :md="4">
          <div class="stat-card">
            <div class="stat-label">{{ card.label }}</div>
            <div class="stat-value">{{ card.value }}</div>
            <el-tag v-if="card.level" :type="cardTagType(card.level)" size="small">
              {{ card.level === "danger" ? "超阈值" : "注意" }}
            </el-tag>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <el-card v-loading="loadingMetrics" shadow="never" class="section-card">
      <template #header>
        <div class="metrics-header">
          <span>
            指标浏览器（{{ families.length }} 个指标族，含下游自注册 na_*；展示 {{ metricRows.length }}/{{ sampleTotal }} 行）
          </span>
          <span v-if="metricsTruncated || rowsTruncated" class="truncated-tip">样本已截断，可用前缀过滤查看</span>
        </div>
      </template>
      <el-table :data="metricRows" border stripe max-height="520" empty-text="无数据（检查指标名前缀）">
        <el-table-column prop="family" label="指标" min-width="220" show-overflow-tooltip />
        <el-table-column prop="type" label="类型" width="100" />
        <el-table-column prop="labels" label="标签" min-width="240" show-overflow-tooltip />
        <el-table-column prop="value" label="值" width="120" align="right" />
      </el-table>
    </el-card>
  </div>
</template>

<style scoped lang="scss">
.observability-page {
  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 8px;

    .page-title {
      font-size: 16px;
      font-weight: 600;
      display: flex;
      align-items: baseline;
      gap: 8px;
      flex-wrap: wrap;

      .node-tip {
        font-size: 12px;
        font-weight: 400;
        color: var(--el-text-color-secondary);
      }
    }

    .page-actions {
      display: flex;
      align-items: center;
      gap: 8px;
      flex-wrap: wrap;
    }

    .auto-refresh-label {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }
  }

  .alert-list {
    margin-top: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .section-card {
    margin-top: 12px;
  }

  .stat-card {
    padding: 12px 8px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 6px;
    margin-bottom: 12px;

    .stat-label {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    .stat-value {
      font-size: 20px;
      font-weight: 600;
      margin: 6px 0;
      word-break: break-all;
    }
  }

  .metrics-header {
    display: flex;
    align-items: center;
    justify-content: space-between;

    .truncated-tip {
      font-size: 12px;
      color: var(--el-color-warning);
    }
  }
}
</style>
