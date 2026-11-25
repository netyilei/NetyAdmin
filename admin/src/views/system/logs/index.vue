<script setup lang="ts" name="operation-logs">
// 操作日志：日志列表（type 过滤）+ security 子面页签 + 详情抽屉 +
// CSV 导出（GET blob）。端点=/admin/v1/operation-logs（contracts/openapi 权威源）；
// 无删除面——logging.md §3 追加写不可篡改。
import { ElMessage } from "element-plus";
import { computed, onMounted, reactive, ref } from "vue";

import {
  auditApi,
  LOG_TYPE_LABELS,
  LOG_TYPE_OPERATION,
  LOG_TYPE_SECURITY,
  type AuditLogQuery,
  type AuditRecord
} from "@/api/modules/audit";
import { fmtTime } from "@/utils/format";

const activeTab = ref("logs");
const loading = ref(false);
const rows = ref<AuditRecord[]>([]);
const total = ref(0);

// 共享过滤（security 页签固定 type=3；logs 页签按筛选）。
const query = reactive({ page: 1, page_size: 10, type: undefined as number | undefined, action: "", result: "", request_id: "" });

const currentType = computed<number | undefined>(() => (activeTab.value === "security" ? LOG_TYPE_SECURITY : query.type));

const buildQuery = (withRequestId: boolean): AuditLogQuery => ({
  type: currentType.value,
  action: query.action || undefined,
  result: query.result || undefined,
  request_id: withRequestId ? query.request_id || undefined : undefined
});

const load = async () => {
  loading.value = true;
  try {
    const resp: any = await auditApi.logs(query.page, query.page_size, buildQuery(true));
    rows.value = resp.data.list;
    total.value = resp.data.total;
  } finally {
    loading.value = false;
  }
};
onMounted(load);

const onTabChange = () => {
  query.page = 1;
  load();
};
const onSearch = () => {
  query.page = 1;
  load();
};

const typeTag = (t: number) => (t === LOG_TYPE_SECURITY ? "danger" : t === LOG_TYPE_OPERATION ? "primary" : "info");
const resultTag = (r?: string) => (r === "success" ? "success" : r === "deny" ? "danger" : r === "error" ? "warning" : "info");

// —— 导出（CSV——当前页签过滤条件；导出端点不支持 request_id）——
const exporting = ref(false);
const onExport = async () => {
  exporting.value = true;
  try {
    const blob = (await auditApi.export(buildQuery(false))) as unknown as Blob;
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `operation-logs-${new Date().toISOString().slice(0, 19).replace(/[T:]/g, "")}.csv`;
    a.click();
    URL.revokeObjectURL(url);
    ElMessage.success("导出已下载（超 10000 行请缩小过滤范围）");
  } finally {
    exporting.value = false;
  }
};

// —— 详情抽屉（全字段展示）——
const drawerVisible = ref(false);
const detailLoading = ref(false);
const detail = ref<AuditRecord | null>(null);
const detailJson = computed(() => (detail.value?.detail ? JSON.stringify(detail.value.detail, null, 2) : ""));

const onDetail = async (row: AuditRecord) => {
  drawerVisible.value = true;
  detailLoading.value = true;
  detail.value = null;
  try {
    const resp: any = await auditApi.get(row.id);
    detail.value = resp.data;
  } finally {
    detailLoading.value = false;
  }
};
</script>

<template>
  <div class="table-box">
    <el-card shadow="never" class="content-card">
      <div class="page-header">
        <el-tabs v-model="activeTab" class="log-tabs" @tab-change="onTabChange">
          <el-tab-pane label="日志" name="logs" />
          <el-tab-pane label="安全事件" name="security" />
        </el-tabs>
        <div class="page-actions">
          <el-select
            v-if="activeTab === 'logs'"
            v-model="query.type"
            placeholder="全部类型"
            clearable
            style="width: 130px"
            @change="onSearch"
          >
            <el-option v-for="(label, t) in LOG_TYPE_LABELS" :key="t" :label="label" :value="t" />
          </el-select>
          <el-select v-model="query.result" placeholder="全部结果" clearable style="width: 130px" @change="onSearch">
            <el-option label="成功" value="success" />
            <el-option label="拒绝" value="deny" />
            <el-option label="错误" value="error" />
          </el-select>
          <el-input
            v-model="query.action"
            placeholder="动作前缀（如 login.）"
            clearable
            style="width: 180px"
            @keyup.enter="onSearch"
            @clear="onSearch"
          />
          <el-input
            v-if="activeTab === 'logs'"
            v-model="query.request_id"
            placeholder="request_id"
            clearable
            style="width: 200px"
            @keyup.enter="onSearch"
            @clear="onSearch"
          />
          <el-button @click="onSearch">搜索</el-button>
          <el-button :loading="exporting" @click="onExport">导出 CSV</el-button>
        </div>
      </div>

      <el-alert
        v-if="activeTab === 'security'"
        title="安全事件=登录失败/越权/限流（type=3 security 子面——同一记录模型按类型过滤）"
        type="warning"
        :closable="false"
        class="mb-2"
      />

      <el-table v-loading="loading" :data="rows" border stripe>
        <el-table-column label="时间" min-width="150">
          <template #default="{ row }">{{ fmtTime(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="操作者" min-width="150" show-overflow-tooltip>
          <template #default="{ row }"
            >{{ row.actorType || "—" }}<template v-if="row.actorId"> / {{ row.actorId }}</template></template
          >
        </el-table-column>
        <el-table-column label="类型" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="typeTag(row.type)">{{ LOG_TYPE_LABELS[row.type] ?? row.type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="内容摘要" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="mono">{{ row.action || "—" }}</span>
            <template v-if="row.resource"> · {{ row.resource }}</template>
          </template>
        </el-table-column>
        <el-table-column label="结果" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="resultTag(row.result)">{{ row.result || "—" }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="requestId" label="request_id" min-width="200" show-overflow-tooltip>
          <template #default="{ row }"
            ><span class="mono">{{ row.requestId || "—" }}</span></template
          >
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="onDetail(row)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pagination-wrap">
        <el-pagination
          layout="total, sizes, prev, pager, next"
          :total="total"
          :page-sizes="[10, 20, 50]"
          :page-size="query.page_size"
          :current-page="query.page"
          @current-change="
            (p: number) => {
              query.page = p;
              load();
            }
          "
          @size-change="
            (s: number) => {
              query.page_size = s;
              query.page = 1;
              load();
            }
          "
        />
      </div>
    </el-card>

    <el-drawer v-model="drawerVisible" title="日志详情" size="480">
      <div v-loading="detailLoading">
        <template v-if="detail">
          <el-descriptions :column="1" border>
            <el-descriptions-item label="ID"
              ><span class="mono">{{ detail.id }}</span></el-descriptions-item
            >
            <el-descriptions-item label="类型">
              <el-tag :type="typeTag(detail.type)">{{ LOG_TYPE_LABELS[detail.type] ?? detail.type }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="动作"
              ><span class="mono">{{ detail.action || "—" }}</span></el-descriptions-item
            >
            <el-descriptions-item label="资源"
              ><span class="mono">{{ detail.resource || "—" }}</span></el-descriptions-item
            >
            <el-descriptions-item label="结果">
              <el-tag :type="resultTag(detail.result)">{{ detail.result || "—" }}</el-tag>
              <template v-if="detail.bizCode">
                · <span class="mono">{{ detail.bizCode }}</span></template
              >
            </el-descriptions-item>
            <el-descriptions-item label="操作者"
              >{{ detail.actorType || "—"
              }}<template v-if="detail.actorId">
                / <span class="mono">{{ detail.actorId }}</span></template
              ></el-descriptions-item
            >
            <el-descriptions-item label="IP">{{ detail.ip || "—" }}</el-descriptions-item>
            <el-descriptions-item label="User-Agent">{{ detail.userAgent || "—" }}</el-descriptions-item>
            <el-descriptions-item label="request_id"
              ><span class="mono">{{ detail.requestId || "—" }}</span></el-descriptions-item
            >
            <el-descriptions-item label="trace_id"
              ><span class="mono">{{ detail.traceId || "—" }}</span></el-descriptions-item
            >
            <el-descriptions-item label="node_id">{{ detail.nodeId || "—" }}</el-descriptions-item>
            <el-descriptions-item label="时间">{{ fmtTime(detail.createdAt) }}</el-descriptions-item>
          </el-descriptions>
          <template v-if="detailJson">
            <div class="detail-section-title">结构化上下文（已脱敏）</div>
            <pre class="mono detail-json">{{ detailJson }}</pre>
          </template>
        </template>
      </div>
    </el-drawer>
  </div>
</template>

<style scoped lang="scss">
.log-tabs {
  flex: 1;
  :deep(.el-tabs__header) {
    margin-bottom: 0;
  }
}
.mb-2 {
  margin-bottom: 8px;
}
.mono {
  font-family: "JetBrains Mono", Consolas, monospace;
  font-size: 12px;
}
.detail-section-title {
  margin: 16px 0 8px;
  font-weight: 600;
}
.detail-json {
  margin: 0;
  padding: 12px;
  max-height: 320px;
  overflow: auto;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
