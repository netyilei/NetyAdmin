<script setup lang="ts" name="gw">
// 应用平台（网关管理一级下）：开放平台应用管理（CRUD/重置密钥/IP 组集+
// scope 授权组绑定）+ 调用统计卡片区。权限码=admin.gw.apps.*（与 rbac_apis
// 同源——12558 种子）。IP 组字典在「IP 访问控制」页、scope 组与 API 白名单
// 字典在「接口权限」页维护，本页只做应用侧绑定（表单单一保存入口）。
// 密钥面：create 由管理端生成明文（仅表单内一次性可见，丢失只能重置）；
// reset-secret 由后端生成（响应通道唯一），均不落日志。
import { ElMessage, ElMessageBox } from "element-plus";
import type { FormInstance, FormRules } from "element-plus";
import { computed, onMounted, reactive, ref } from "vue";

import { gwApi, ipacApi, type GwAppRow, type GwScopeGroup, type IPGroup } from "@/api/modules/gwAdmin";
import { fmtTime } from "@/utils/format";

const PERM_CREATE = "admin.gw.apps.create";
const PERM_UPDATE = "admin.gw.apps.update";
const PERM_DELETE = "admin.gw.apps.delete";
const PERM_RESET = "admin.gw.apps.reset_secret";

// —— 统计卡片区（overview 四指标——简化 el-statistic 卡片行） ——
const hours = ref(24);
const statsLoading = ref(false);
const stats = reactive({ total: 0, errors: 0, errorRate: 0, avgLatency: 0 });

const errorRatePercent = computed(() => Number((stats.errorRate * 100).toFixed(2)));

const loadStats = async () => {
  statsLoading.value = true;
  try {
    const resp: any = await gwApi.statsOverview(hours.value);
    const d = resp.data ?? {};
    stats.total = d.total ?? 0;
    stats.errors = d.errors ?? 0;
    stats.errorRate = d.errorRate ?? 0;
    stats.avgLatency = d.avgLatency ?? 0;
  } finally {
    statsLoading.value = false;
  }
};

// —— 应用列表 ——
const loading = ref(false);
const rows = ref<GwAppRow[]>([]);
const total = ref(0);
const query = reactive({ page: 1, page_size: 10 });

const load = async () => {
  loading.value = true;
  try {
    const resp: any = await gwApi.apps(query.page, query.page_size);
    rows.value = resp.data.list ?? [];
    total.value = resp.data.total ?? 0;
  } finally {
    loading.value = false;
  }
};

// —— IP 策略组/scope 组选项（绑定多选下拉 + 表格回显；字典归 IP 访问控制/
// 接口权限两页维护） ——
const ipGroups = ref<IPGroup[]>([]);
const groupNameOf = (id: string) => ipGroups.value.find(g => g.id === id)?.name ?? id;
const scopeGroups = ref<{ code: string; name: string }[]>([]);
const scopeNameOf = (code: string) => scopeGroups.value.find(g => g.code === code)?.name ?? code;
const loadGroupOptions = async () => {
  try {
    const [ipResp, scopeResp] = await Promise.all([ipacApi.groups(), gwApi.availableScopes()]);
    ipGroups.value = ((ipResp as any).data ?? []) as IPGroup[];
    scopeGroups.value = (((scopeResp as any).data ?? []) as GwScopeGroup[]).map(g => ({
      code: g.code,
      name: g.name
    }));
  } catch {
    ipGroups.value = []; // 字典读取失败不阻塞应用面（绑定校验在后端 fail-closed）
  }
};

onMounted(() => {
  loadStats();
  load();
  loadGroupOptions();
});

const randomHex = (bytes: number) => {
  const arr = new Uint8Array(bytes);
  crypto.getRandomValues(arr);
  return Array.from(arr, b => b.toString(16).padStart(2, "0")).join("");
};

// —— 应用启停（状态列此前只读；对照 users 页 onToggleStatus；
// 后端 PUT /open/apps/:id/status——权限码 admin.gw.apps.update） ——
const onToggleStatus = async (row: GwAppRow) => {
  const next = row.status === 1 ? 2 : 1;
  await ElMessageBox.confirm(
    next === 2 ? `确认禁用「${row.name}」？该应用调用方立即 403。` : `确认启用「${row.name}」？`,
    "状态变更",
    { type: "warning" }
  );
  await gwApi.setAppStatus(row.id, next);
  ElMessage.success(next === 2 ? "已禁用" : "已启用");
  await load();
};

// —— 新建/编辑 ——
const editVisible = ref(false);
const editMode = ref<"create" | "update">("create");
const editRow = ref<GwAppRow | null>(null);
const editFormRef = ref<FormInstance>();
const editForm = reactive({
  name: "",
  appKey: "",
  secret: "",
  remark: "",
  rateLimitEnabled: false,
  rate: 0, // 每秒请求数（0=不限——G3 契约语义）
  ipGroupIds: [] as string[], // 组集空=不限 IP
  scopes: [] as string[] // 授权组集空=无开放 API 权限
});
// 表单校验前置（②/G2：名称必填≤100；appKey 16-64、secret 16-128——
// 与后端 createAppRequest 校验同源，提交前拦截免 400 裸弹错）
const editRules = reactive<FormRules>({
  name: [
    { required: true, message: "请输入应用名称", trigger: "blur" },
    { max: 100, message: "名称不能超过 100 字", trigger: "blur" }
  ],
  appKey: [
    { required: true, message: "请输入 AppKey", trigger: "blur" },
    { min: 16, max: 64, message: "AppKey 长度须为 16-64 位", trigger: "blur" }
  ],
  secret: [
    { required: true, message: "请输入 AppSecret", trigger: "blur" },
    { min: 16, max: 128, message: "AppSecret 长度须为 16-128 位", trigger: "blur" }
  ]
});

const openCreate = () => {
  editMode.value = "create";
  editRow.value = null;
  Object.assign(editForm, {
    name: "",
    appKey: `ak_${randomHex(16)}`,
    secret: randomHex(32),
    remark: "",
    rateLimitEnabled: false,
    rate: 0,
    ipGroupIds: [],
    scopes: []
  });
  editVisible.value = true;
};
const openEdit = (row: GwAppRow) => {
  editMode.value = "update";
  editRow.value = row;
  Object.assign(editForm, {
    name: row.name,
    appKey: row.appKey,
    secret: "",
    remark: row.remark ?? "",
    rateLimitEnabled: Boolean(row.rateLimitEnabled),
    rate: Number(row.quota?.rate ?? 0),
    ipGroupIds: Array.isArray(row.ipGroupIds) ? row.ipGroupIds.slice() : [],
    scopes: Array.isArray(row.scopes) ? row.scopes.slice() : []
  });
  editVisible.value = true;
};
const onSubmitEdit = async () => {
  const ok = await editFormRef.value
    ?.validate()
    .then(() => true)
    .catch(() => false);
  if (!ok) return;
  if (editMode.value === "create") {
    await gwApi.createApp({
      name: editForm.name,
      appKey: editForm.appKey,
      secret: editForm.secret,
      remark: editForm.remark || undefined,
      rateLimitEnabled: editForm.rateLimitEnabled,
      quota: { rate: editForm.rateLimitEnabled ? editForm.rate : 0 }, // 关限流=清零
      ipGroupIds: editForm.ipGroupIds,
      scopes: editForm.scopes
    });
    // 密钥一次性展示（此前创建即关弹窗，toast 提示「请立即保存」时已不可保存）。
    // 后端 AppView 脱敏不含 secret（domain/gw.go）；创建密钥由前端生成、表单本就持有明文，
    // 直接带入展示弹窗即可——无需 reset-secret 语义（轮换刚建的密钥毫无意义）。
    secretDisplay.appKey = editForm.appKey;
    secretDisplay.secret = editForm.secret;
    secretVisible.value = true;
    ElMessage.success("应用已创建");
  } else if (editRow.value) {
    await gwApi.updateApp(editRow.value.id, {
      name: editForm.name,
      remark: editForm.remark || undefined,
      rateLimitEnabled: editForm.rateLimitEnabled,
      quota: { rate: editForm.rate },
      ipGroupIds: editForm.ipGroupIds,
      scopes: editForm.scopes
    });
    ElMessage.success("已更新");
  }
  editVisible.value = false;
  await Promise.all([load(), loadGroupOptions()]);
};

// —— AppSecret 一次性展示弹窗（创建成功后打开；关闭即清内存明文） ——
const secretVisible = ref(false);
const secretDisplay = reactive({ appKey: "", secret: "" });
const onCopySecret = async () => {
  await navigator.clipboard.writeText(secretDisplay.secret);
  ElMessage.success("AppSecret 已复制");
};

const onDelete = async (row: GwAppRow) => {
  await ElMessageBox.confirm(`确认删除应用「${row.name}」（${row.appKey}）？删除后调用方立即失效。`, "删除确认", {
    type: "warning"
  });
  await gwApi.deleteApp(row.id);
  ElMessage.success("已删除");
  await load();
};

const onResetSecret = async (row: GwAppRow) => {
  await ElMessageBox.confirm(`确认重置「${row.appKey}」的 AppSecret？旧密钥立即失效。`, "重置确认", {
    type: "warning"
  });
  const resp: any = await gwApi.resetSecret(row.id);
  const d = resp.data ?? {};
  ElMessageBox.alert(`AppKey：${d.appKey}\n新 AppSecret（一次性显示）：${d.secret}`, "重置成功", { type: "warning" });
};
</script>

<template>
  <div class="table-box">
    <!-- 统计卡片区（overview 四指标） -->
    <el-row v-loading="statsLoading" :gutter="12" class="stats-row">
      <el-col :span="6">
        <el-card shadow="never"><el-statistic title="调用总量" :value="stats.total" /></el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="never"><el-statistic title="错误数" :value="stats.errors" /></el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="never"><el-statistic title="错误率 (%)" :value="errorRatePercent" /></el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="never"><el-statistic title="平均延迟 (ms)" :value="stats.avgLatency" /></el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="content-card">
      <div class="page-header">
        <span class="page-title">开放平台应用</span>
        <div class="page-actions">
          <el-radio-group v-model="hours" size="small" @change="loadStats">
            <el-radio-button :value="24">24h</el-radio-button>
            <el-radio-button :value="72">72h</el-radio-button>
            <el-radio-button :value="168">7d</el-radio-button>
            <el-radio-button :value="720">30d</el-radio-button>
          </el-radio-group>
          <el-button v-auth="PERM_CREATE" type="primary" @click="openCreate">新建应用</el-button>
        </div>
      </div>

      <el-table v-loading="loading" :data="rows" border stripe>
        <el-table-column prop="name" label="名称" min-width="140" show-overflow-tooltip />
        <el-table-column prop="appKey" label="AppKey" min-width="220" show-overflow-tooltip />
        <el-table-column label="限流" width="110" align="center">
          <template #default="{ row }">
            <!-- rate≤0=不限（domain 语义）——enabled+0 也显示不限 -->
            <span v-if="row.rateLimitEnabled && (row.quota?.rate ?? 0) > 0">{{ row.quota?.rate }}/s</span>
            <span v-else style="color: var(--el-text-color-secondary)">不限</span>
          </template>
        </el-table-column>
        <el-table-column label="IP 策略组" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="(row.ipGroupIds ?? []).length">{{ row.ipGroupIds.map(groupNameOf).join("、") }}</span>
            <span v-else style="color: var(--el-text-color-secondary)">不限</span>
          </template>
        </el-table-column>
        <el-table-column label="scope 授权组" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="(row.scopes ?? []).length">{{ row.scopes.map(scopeNameOf).join("、") }}</span>
            <span v-else style="color: var(--el-text-color-secondary)">—</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? "启用" : "禁用" }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="备注" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ row.remark || "—" }}</template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="150">
          <template #default="{ row }">{{ fmtTime(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="300" fixed="right">
          <template #default="{ row }">
            <el-button v-auth="PERM_UPDATE" size="small" @click="openEdit(row)">编辑</el-button>
            <el-button
              v-auth="PERM_UPDATE"
              size="small"
              :type="row.status === 1 ? 'warning' : 'success'"
              plain
              @click="onToggleStatus(row)"
            >
              {{ row.status === 1 ? "禁用" : "启用" }}
            </el-button>
            <el-button v-auth="PERM_RESET" size="small" type="warning" plain @click="onResetSecret(row)"> 重置密钥 </el-button>
            <el-button v-auth="PERM_DELETE" size="small" type="danger" plain @click="onDelete(row)">删除</el-button>
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

    <el-dialog v-model="editVisible" :title="editMode === 'create' ? '新建应用' : '编辑应用'" width="520" destroy-on-close>
      <el-form ref="editFormRef" :model="editForm" :rules="editRules" label-width="100">
        <el-form-item label="名称" prop="name">
          <el-input v-model="editForm.name" placeholder="应用名称（≤100 字）" />
        </el-form-item>
        <el-form-item v-if="editMode === 'create'" label="AppKey" prop="appKey">
          <el-input v-model="editForm.appKey" placeholder="16-64 位，唯一">
            <template #append>
              <el-button @click="editForm.appKey = `ak_${randomHex(16)}`">生成</el-button>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item v-if="editMode === 'create'" label="AppSecret" prop="secret">
          <el-input v-model="editForm.secret" placeholder="16-128 位">
            <template #append>
              <el-button @click="editForm.secret = randomHex(32)">生成</el-button>
            </template>
          </el-input>
          <div class="form-tip">仅创建时可见，请立即保存；丢失只能重置密钥（旧密钥立即失效）。</div>
        </el-form-item>
        <el-form-item label="访问频率">
          <div style="width: 100%">
            <el-switch v-model="editForm.rateLimitEnabled" active-text="启用限流" style="margin-right: 12px" />
            <el-input-number v-model="editForm.rate" :min="0" :disabled="!editForm.rateLimitEnabled" />
            <span style="margin-left: 8px; color: var(--el-text-color-secondary)">次/秒（0=不限）</span>
          </div>
        </el-form-item>
        <el-form-item label="IP 策略组">
          <el-select v-model="editForm.ipGroupIds" multiple clearable placeholder="不选=不限 IP" style="width: 100%">
            <el-option v-for="g in ipGroups" :key="g.id" :value="g.id" :label="g.name" />
          </el-select>
          <div class="form-tip">
            组集严格合成「禁止 &gt; 开放」：任一黑名单命中即拒；组集非空无命中即拒；组集空=不限。 组/规则字典在「IP
            访问控制」页维护。
          </div>
        </el-form-item>
        <el-form-item label="scope 授权组">
          <el-select v-model="editForm.scopes" multiple clearable placeholder="不选=无开放 API 权限" style="width: 100%">
            <el-option v-for="g in scopeGroups" :key="g.code" :value="g.code" :label="g.name || g.code" />
          </el-select>
          <div class="form-tip">授权组内的 API 对该调用方放行（scope 白名单）；组字典与 API 白名单在「接口权限」页维护。</div>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="editForm.remark" type="textarea" :rows="2" maxlength="255" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="onSubmitEdit">确定</el-button>
      </template>
    </el-dialog>

    <!-- AppSecret 一次性展示 -->
    <el-dialog
      v-model="secretVisible"
      title="应用已创建——请立即保存密钥"
      width="520"
      destroy-on-close
      @closed="secretDisplay.secret = ''"
    >
      <el-alert type="warning" :closable="false" show-icon title="该密钥仅本次展示，关闭后无法再次查看" class="secret-alert" />
      <el-form label-width="100">
        <el-form-item label="AppKey">
          <span class="secret-value">{{ secretDisplay.appKey }}</span>
        </el-form-item>
        <el-form-item label="AppSecret">
          <span class="secret-value">{{ secretDisplay.secret }}</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button type="primary" @click="onCopySecret">复制 AppSecret</el-button>
        <el-button @click="secretVisible = false">我已保存，关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.stats-row {
  margin-bottom: 12px;
}
.mb-2 {
  margin-bottom: 8px;
}
.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
}
.secret-alert {
  margin-bottom: 12px;
}
.secret-value {
  font-family: monospace;
  font-size: 13px;
  word-break: break-all;
  line-height: 1.5;
}
.scope-desc {
  margin-left: 6px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
