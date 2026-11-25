<script setup lang="ts" name="taskcenter">
// 任务中心：任务目录（暂停/恢复/立即触发）+ 运行记录抽屉。
// 权限码=admin.taskcenter.view/operate（与 rbac_apis 同源——12566 种子）。
// 注：JobInfo 无 next-run 字段（scheduler 快照面只有 schedule）——下次执行
// 以调度表达式呈现，不虚构数据。
import { ElMessage } from "element-plus";
import { onMounted, ref } from "vue";

import { taskcenterApi, type JobInfo, type RunRecord } from "@/api/modules/taskcenter";
import { fmtTime } from "@/utils/format";

const PERM_OPERATE = "admin.taskcenter.operate";

const loading = ref(false);
const jobs = ref<JobInfo[]>([]);

const load = async () => {
  loading.value = true;
  try {
    const resp: any = await taskcenterApi.jobs();
    jobs.value = resp.data.jobs ?? [];
  } finally {
    loading.value = false;
  }
};
onMounted(load);

// —— 暂停/恢复/立即触发（operate） ——
const onTogglePause = async (row: JobInfo) => {
  if (row.paused) {
    await taskcenterApi.resume(row.name);
    ElMessage.success(`任务「${row.name}」已恢复`);
  } else {
    await taskcenterApi.pause(row.name);
    ElMessage.success(`任务「${row.name}」已暂停`);
  }
  await load();
};

const onTrigger = async (row: JobInfo) => {
  const resp: any = await taskcenterApi.trigger(row.name);
  const data = resp.data ?? {};
  if (data.triggered) {
    ElMessage.success(`任务「${row.name}」已触发执行`);
  } else {
    ElMessage.warning(`未触发：${data.reason === "paused" ? "任务已暂停" : "当前节点非 leader"}`);
  }
  await load();
};

// —— 运行记录抽屉 ——
const runsVisible = ref(false);
const runsLoading = ref(false);
const runsTarget = ref<JobInfo | null>(null);
const runs = ref<RunRecord[]>([]);

type TagType = "primary" | "success" | "warning" | "info" | "danger";
const STATUS_TAG: Record<string, TagType> = { running: "warning", succeeded: "success", failed: "danger" };
const STATUS_TEXT: Record<string, string> = {
  running: "进行中",
  succeeded: "成功",
  failed: "失败",
  unknown: "未知"
};
const statusTag = (s: string): TagType => STATUS_TAG[s] ?? "info";
const statusText = (s: string): string => STATUS_TEXT[s] ?? s;

const durationText = (r: RunRecord) => {
  if (!r.finished_at || !r.started_at) return "—";
  const ms = new Date(r.finished_at).getTime() - new Date(r.started_at).getTime();
  return ms >= 1000 ? `${(ms / 1000).toFixed(1)}s` : `${ms}ms`;
};

const onRuns = async (row: JobInfo) => {
  runsTarget.value = row;
  runsVisible.value = true;
  runsLoading.value = true;
  try {
    const resp: any = await taskcenterApi.runs(row.name);
    runs.value = resp.data.runs ?? [];
  } finally {
    runsLoading.value = false;
  }
};
</script>

<template>
  <div class="table-box">
    <el-card shadow="never" class="content-card">
      <div class="page-header">
        <span class="page-title">任务中心</span>
        <div class="page-actions">
          <el-button @click="load">刷新</el-button>
        </div>
      </div>

      <el-table v-loading="loading" :data="jobs" border stripe>
        <el-table-column prop="name" label="任务名" min-width="180" show-overflow-tooltip />
        <el-table-column prop="trigger" label="类型" width="110" align="center" />
        <el-table-column label="调度" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.schedule || "—" }}
            <span v-if="row.timezone" class="tz">{{ row.timezone }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="scope" label="Scope" width="110" align="center" />
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.paused" type="danger">已暂停</el-tag>
            <el-tag v-else-if="row.running" type="primary">运行中</el-tag>
            <el-tag v-else type="success">空闲</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button v-auth="PERM_OPERATE" size="small" type="primary" plain @click="onTrigger(row)"> 立即触发 </el-button>
            <el-button v-auth="PERM_OPERATE" size="small" @click="onTogglePause(row)">
              {{ row.paused ? "恢复" : "暂停" }}
            </el-button>
            <el-button size="small" @click="onRuns(row)">运行记录</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-drawer v-model="runsVisible" :title="`运行记录：${runsTarget?.name ?? ''}`" size="55%" destroy-on-close>
      <el-table v-loading="runsLoading" :data="runs" border stripe size="small">
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="耗时" width="90" align="center">
          <template #default="{ row }">{{ durationText(row) }}</template>
        </el-table-column>
        <el-table-column prop="trigger_kind" label="触发方式" width="100" align="center" />
        <el-table-column prop="node" label="节点" min-width="140" show-overflow-tooltip />
        <el-table-column label="错误" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">{{ row.error || "—" }}</template>
        </el-table-column>
        <el-table-column label="开始时间" width="160">
          <template #default="{ row }">{{ fmtTime(row.started_at) }}</template>
        </el-table-column>
        <el-table-column label="结束时间" width="160">
          <template #default="{ row }">{{ fmtTime(row.finished_at) }}</template>
        </el-table-column>
      </el-table>
    </el-drawer>
  </div>
</template>

<style scoped lang="scss">
.tz {
  margin-left: 6px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
