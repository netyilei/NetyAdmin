<script setup lang="ts" name="gwscope">
// 接口权限（网关管理一级下，与 IP 访问控制对称——IP 管「谁能来」，本页管
// 「能访问什么」）：scope 组字典 CRUD + 选中组 API 白名单勾选（gw_scope_apis
// 全量替换）。API 登记台账在「API 登记」页维护，
// 本页白名单勾选只消费登记清单。应用侧在「应用平台」页多选 scope 组；
// scope 组被应用授权/API 白名单引用中=拒删（后端 409 守卫）。
import { ElMessage, ElMessageBox } from "element-plus";
import { computed, onMounted, reactive, ref } from "vue";

import { scopeApi, type GwApiRow, type ScopeGroupRow } from "@/api/modules/gwAdmin";

const PERM_SCOPES_WRITE = "admin.gw.scope-groups.write";
const PERM_APIS_BIND = "admin.gw.apis.bind";

// —— scope 组列表（左栏；点击行查看/维护 API 白名单） ——
const loading = ref(false);
const groups = ref<ScopeGroupRow[]>([]);
const selected = ref<ScopeGroupRow | null>(null);
const groupRowClass = ({ row }: { row: ScopeGroupRow }) =>
  selected.value && row.id === selected.value.id ? "scope-selected" : "";

const loadGroups = async () => {
  loading.value = true;
  try {
    const resp: any = await scopeApi.groups();
    groups.value = (resp.data ?? []) as ScopeGroupRow[];
    if (selected.value) {
      selected.value = groups.value.find(g => g.id === selected.value!.id) ?? null;
      if (!selected.value) grantedApiIDs.value = [];
    }
  } finally {
    loading.value = false;
  }
};
const onSelectGroup = (row: ScopeGroupRow) => {
  selected.value = row;
  loadGrantedApis();
};
onMounted(loadGroups);

// —— 选中组的 API 白名单（右栏 checkbox 全量替换） ——
const apis = ref<GwApiRow[]>([]);
const apisLoading = ref(false);
const grantedApiIDs = ref<string[]>([]);
const grantedSet = computed(() => new Set(grantedApiIDs.value));

const loadApis = async () => {
  apisLoading.value = true;
  try {
    const resp: any = await scopeApi.apis();
    apis.value = (resp.data ?? []) as GwApiRow[];
  } finally {
    apisLoading.value = false;
  }
};
// 白名单请求的选中态守卫（快速连点 A/B 组时响应乱序会把 A 的授权
// 集灌给 B——响应落地前校验 selected 仍是发起时的组，漂移即丢弃）
const loadGrantedApis = async () => {
  const target = selected.value;
  if (!target) {
    grantedApiIDs.value = [];
    return;
  }
  const resp: any = await scopeApi.scopeApis(target.code);
  if (!selected.value || selected.value.id !== target.id) {
    return; // 选中组已漂移——丢弃过期响应
  }
  grantedApiIDs.value = ((resp.data ?? []) as GwApiRow[]).map(a => a.id);
};
const onToggleApi = (apiID: string) => {
  if (grantedSet.value.has(apiID)) {
    grantedApiIDs.value = grantedApiIDs.value.filter(i => i !== apiID);
  } else {
    grantedApiIDs.value = [...grantedApiIDs.value, apiID];
  }
};
const savingGrant = ref(false);
const onSaveGrant = async () => {
  if (!selected.value) return;
  savingGrant.value = true;
  try {
    await scopeApi.setScopeApis(selected.value.code, grantedApiIDs.value);
    ElMessage.success("白名单已保存");
    await loadGrantedApis();
  } finally {
    savingGrant.value = false;
  }
};

// —— scope 组新建/编辑 ——
const groupVisible = ref(false);
const groupMode = ref<"create" | "update">("create");
const groupRow = ref<ScopeGroupRow | null>(null);
const groupForm = reactive({ code: "", name: "", description: "", status: 1 });
const openGroupCreate = () => {
  groupMode.value = "create";
  groupRow.value = null;
  Object.assign(groupForm, { code: "", name: "", description: "", status: 1 });
  groupVisible.value = true;
};
const openGroupEdit = (row: ScopeGroupRow) => {
  groupMode.value = "update";
  groupRow.value = row;
  Object.assign(groupForm, { code: row.code, name: row.name, description: row.description, status: row.status });
  groupVisible.value = true;
};
const onSubmitGroup = async () => {
  if (groupMode.value === "create") {
    await scopeApi.createGroup({
      code: groupForm.code,
      name: groupForm.name,
      description: groupForm.description || undefined,
      status: groupForm.status
    });
    ElMessage.success("scope 组已创建");
  } else if (groupRow.value) {
    // code 随编辑提交（后端 required——P0-2；编辑弹窗已回填原值，组唯一键不可改）
    await scopeApi.updateGroup(groupRow.value.id, {
      code: groupForm.code,
      name: groupForm.name,
      description: groupForm.description || undefined,
      status: groupForm.status
    });
    ElMessage.success("scope 组已更新");
  }
  groupVisible.value = false;
  await loadGroups();
};
const onDeleteGroup = async (row: ScopeGroupRow) => {
  await ElMessageBox.confirm(
    `确认删除 scope 组「${row.name}」（${row.code}）？应用授权或 API 白名单引用中将被拒绝（需先解绑）。`,
    "删除确认",
    { type: "warning" }
  );
  try {
    await scopeApi.deleteGroup(row.id);
  } catch (e: any) {
    // 占用守卫（后端 409 scope group in use by apps）——拦截器 reject 信封
    // {code, message_key, details:{reason}}；命中即本地引导，不叠加全局 toast
    const reason = String(e?.details?.reason ?? "");
    if (reason.includes("in use")) {
      ElMessage.warning("该组仍被应用授权或 API 白名单引用，请先解绑后再删除");
      return;
    }
    if (reason) return; // 其余业务错已由全局拦截器 toast——吞掉避免未处理 rejection
    throw e;
  }
  ElMessage.success("已删除");
  await loadGroups();
};

onMounted(loadApis);
</script>

<template>
  <div class="table-box">
    <el-card shadow="never" class="content-card">
      <div class="page-header">
        <span class="page-title">接口权限</span>
        <div class="page-actions">
          <el-button v-auth="PERM_SCOPES_WRITE" type="primary" @click="openGroupCreate">新建 scope 组</el-button>
        </div>
      </div>

      <div class="semantic-tip">
        scope 组=开放 API 的授权分组：组内勾选的 API 对持有该 scope 的调用方放行（edge scope 白名单）。
        应用在「应用平台」页多选授权组；组被应用授权或白名单引用中时不可删除。
      </div>

      <el-row :gutter="12">
        <!-- 左：scope 组列表（点击行维护白名单） -->
        <el-col :span="8">
          <div class="pane-title">scope 组（点击行维护 API 白名单）</div>
          <el-table
            v-loading="loading"
            :data="groups"
            border
            stripe
            highlight-current-row
            :row-class-name="groupRowClass"
            @row-click="onSelectGroup"
          >
            <el-table-column label="名称" min-width="100" show-overflow-tooltip>
              <template #default="{ row }">{{ row.name }}</template>
            </el-table-column>
            <el-table-column label="Code" min-width="100" show-overflow-tooltip>
              <template #default="{ row }">{{ row.code }}</template>
            </el-table-column>
            <el-table-column label="状态" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? "启用" : "停用" }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="130" fixed="right">
              <template #default="{ row }">
                <el-button v-auth="PERM_SCOPES_WRITE" size="small" @click.stop="openGroupEdit(row)">编辑</el-button>
                <el-button v-auth="PERM_SCOPES_WRITE" size="small" type="danger" plain @click.stop="onDeleteGroup(row)">
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-col>

        <!-- 右：选中组的 API 白名单 -->
        <el-col :span="16">
          <div class="pane-title">
            API 白名单{{ selected ? `：${selected.name}（${selected.code}）` : "" }}
            <el-button
              v-if="selected"
              v-auth="PERM_APIS_BIND"
              size="small"
              type="primary"
              :loading="savingGrant"
              @click="onSaveGrant"
            >
              保存白名单
            </el-button>
          </div>
          <div v-if="selected" class="form-tip" style="margin-bottom: 8px">
            勾选 = 该 scope 组放行对应 API；保存即全量替换生效。
          </div>
          <el-table v-if="selected" v-loading="apisLoading" :data="apis" border stripe max-height="360">
            <el-table-column label="授权" width="60" align="center">
              <template #default="{ row }">
                <el-checkbox :model-value="grantedSet.has(row.id)" @change="onToggleApi(row.id)" />
              </template>
            </el-table-column>
            <el-table-column prop="method" label="方法" width="80" align="center" />
            <el-table-column prop="path" label="路径" min-width="200" show-overflow-tooltip />
            <el-table-column prop="name" label="名称" min-width="120" show-overflow-tooltip />
            <el-table-column label="分组" width="100" show-overflow-tooltip>
              <template #default="{ row }">{{ row.grp || "—" }}</template>
            </el-table-column>
          </el-table>
          <el-empty v-else description="请选择左侧 scope 组" :image-size="80" />
        </el-col>
      </el-row>
    </el-card>

    <el-dialog
      v-model="groupVisible"
      :title="groupMode === 'create' ? '新建 scope 组' : '编辑 scope 组'"
      width="480"
      destroy-on-close
    >
      <el-form label-width="90">
        <el-form-item label="Code" required>
          <el-input
            v-model="groupForm.code"
            :disabled="groupMode === 'update'"
            placeholder="scope code（如 user_base，创建后不可改）"
            maxlength="50"
          />
        </el-form-item>
        <el-form-item label="名称" required>
          <el-input v-model="groupForm.name" placeholder="组名称（≤100 字）" maxlength="100" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="groupForm.description" type="textarea" :rows="2" maxlength="255" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="groupForm.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="2">停用</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupVisible = false">取消</el-button>
        <el-button type="primary" @click="onSubmitGroup">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
}
.pane-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.semantic-tip {
  padding: 10px 14px;
  margin-bottom: 14px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--el-text-color-regular);
  background: var(--el-fill-color-light);
  border-radius: 4px;
}
/* 选中组行高亮 */
:deep(.scope-selected td) {
  background-color: var(--el-color-primary-light-9) !important;
}
</style>
