<script setup lang="ts" name="gwapi">
// API 登记（网关管理一级下独立页，2026-11-21 用户从接口权限页拆出）：
// gw_apis 全量登记台账（admin 面+client 面，grp=业务模块）——登记是开放的
// 目录面，API 归入哪些 scope 组（授权分组）在「接口权限」页维护。
import { ElMessage, ElMessageBox } from "element-plus";
import { onMounted, reactive, ref } from "vue";

import { scopeApi, type GwApiRow } from "@/api/modules/gwAdmin";

const PERM_APIS_CREATE = "admin.gw.apis.create";
const PERM_APIS_UPDATE = "admin.gw.apis.update";
const PERM_APIS_DELETE = "admin.gw.apis.delete";

const loading = ref(false);
const rows = ref<GwApiRow[]>([]);

const load = async () => {
  loading.value = true;
  try {
    const resp: any = await scopeApi.apis();
    rows.value = (resp.data ?? []) as GwApiRow[];
  } finally {
    loading.value = false;
  }
};
onMounted(load);

const apiVisible = ref(false);
const apiMode = ref<"create" | "update">("create");
const apiRow = ref<GwApiRow | null>(null);
const apiForm = reactive({ method: "GET", path: "", name: "", grp: "", description: "", status: 1 });

const openCreate = () => {
  apiMode.value = "create";
  apiRow.value = null;
  Object.assign(apiForm, { method: "GET", path: "", name: "", grp: "", description: "", status: 1 });
  apiVisible.value = true;
};
const openEdit = (row: GwApiRow) => {
  apiMode.value = "update";
  apiRow.value = row;
  Object.assign(apiForm, {
    method: row.method,
    path: row.path,
    name: row.name,
    grp: row.grp,
    description: row.description,
    status: row.status
  });
  apiVisible.value = true;
};
const onSubmit = async () => {
  const payload = {
    method: apiForm.method,
    path: apiForm.path,
    name: apiForm.name,
    grp: apiForm.grp || undefined,
    description: apiForm.description || undefined,
    status: apiForm.status
  };
  if (apiMode.value === "create") {
    await scopeApi.createApi(payload);
    ElMessage.success("网关 API 已登记");
  } else if (apiRow.value) {
    await scopeApi.updateApi(apiRow.value.id, payload);
    ElMessage.success("网关 API 已更新");
  }
  apiVisible.value = false;
  await load();
};
const onDelete = async (row: GwApiRow) => {
  await ElMessageBox.confirm(`确认删除登记 API「${row.method} ${row.path}」？引用它的 scope 白名单行将一并清理。`, "删除确认", {
    type: "warning"
  });
  await scopeApi.deleteApi(row.id);
  ElMessage.success("已删除");
  await load();
};
</script>

<template>
  <div class="table-box">
    <el-card shadow="never" class="content-card">
      <div class="page-header">
        <span class="page-title">API 登记</span>
        <div class="page-actions">
          <el-button v-auth="PERM_APIS_CREATE" type="primary" @click="openCreate">登记 API</el-button>
        </div>
      </div>
      <div class="ledger-tip">
        开放 API 的登记台账（admin 面+client 面，grp=业务模块）。API 归入哪些 scope
        组（授权分组）在「接口权限」页维护；路径/说明与各模块真实路由同源。
      </div>
      <el-table v-loading="loading" :data="rows" border stripe>
        <el-table-column prop="method" label="方法" width="90" align="center" />
        <el-table-column prop="path" label="路径" min-width="260" show-overflow-tooltip />
        <el-table-column prop="name" label="名称" min-width="150" show-overflow-tooltip />
        <el-table-column prop="grp" label="模块" width="110" show-overflow-tooltip>
          <template #default="{ row }">{{ row.grp || "—" }}</template>
        </el-table-column>
        <el-table-column label="说明" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">{{ row.description || "—" }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? "启用" : "停用" }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button v-auth="PERM_APIS_UPDATE" size="small" @click="openEdit(row)">编辑</el-button>
            <el-button v-auth="PERM_APIS_DELETE" size="small" type="danger" plain @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="apiVisible" :title="apiMode === 'create' ? '登记网关 API' : '编辑网关 API'" width="520" destroy-on-close>
      <el-form label-width="90">
        <el-form-item label="方法" required>
          <el-select v-model="apiForm.method" style="width: 140px">
            <el-option v-for="m in ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']" :key="m" :value="m" :label="m" />
          </el-select>
        </el-form-item>
        <el-form-item label="路径" required>
          <el-input v-model="apiForm.path" placeholder="/gateway/v1/... 或 /client/v1/..." maxlength="255" />
        </el-form-item>
        <el-form-item label="名称" required>
          <el-input v-model="apiForm.name" placeholder="API 名称（≤100 字）" maxlength="100" />
        </el-form-item>
        <el-form-item label="模块">
          <el-input v-model="apiForm.grp" placeholder="业务分组（如 users/message/storage）" maxlength="50" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="apiForm.description" type="textarea" :rows="2" maxlength="255" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="apiForm.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="2">停用</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="apiVisible = false">取消</el-button>
        <el-button type="primary" @click="onSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.ledger-tip {
  padding: 10px 14px;
  margin-bottom: 14px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--el-text-color-regular);
  background: var(--el-fill-color-light);
  border-radius: 4px;
}
</style>
