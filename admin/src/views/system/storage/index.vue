<script setup lang="ts" name="storageConfig">
// 存储配置：配置 CRUD + 设默认 + 上传记录查看 + 过期记录手动清理。
// 端点=admin.storage.*（contracts/openapi/admin.v1.yaml）；权限码与 rbac_apis 同源。
import { ElMessage, ElMessageBox } from "element-plus";
import { onMounted, reactive, ref } from "vue";

import { storageApi, type StorageConfigRow, type StorageProvider, type StorageRecordRow } from "@/api/modules/storage";
import { fmtTime } from "@/utils/format";

// 实体状态枚举（domain.StorageConfig.Status / UploadRecord.Status——具名禁字面量）
const CFG_ENABLED = 1;
const REC_PENDING = 1;
const REC_COMPLETED = 2;
const REC_EXPIRED = 3;

const activeTab = ref("configs");

// —— 存储配置 tab ——
const cfgLoading = ref(false);
const cfgRows = ref<StorageConfigRow[]>([]);
const cfgTotal = ref(0);
const cfgQuery = reactive({ page: 1, page_size: 10 });

const loadConfigs = async () => {
  cfgLoading.value = true;
  try {
    const resp: any = await storageApi.listConfigs(cfgQuery.page, cfgQuery.page_size);
    cfgRows.value = resp.data.list;
    cfgTotal.value = resp.data.total;
  } finally {
    cfgLoading.value = false;
  }
};

// —— 新建/编辑弹窗（secret_key 响应掩码不回显——更新需重填） ——
const editVisible = ref(false);
const editMode = ref<"create" | "update">("create");
const editRow = ref<StorageConfigRow | null>(null);
const editForm = reactive({
  name: "",
  provider: "minio" as StorageProvider,
  endpoint: "",
  region: "",
  bucket_lookup: "" as "" | "path" | "virtual-host", // 空=默认 path
  bucket: "",
  access_key: "",
  secret_key: "",
  domain: "",
  path_prefix: ""
});

const openCreate = () => {
  editMode.value = "create";
  editRow.value = null;
  Object.assign(editForm, {
    name: "",
    provider: "minio",
    endpoint: "",
    region: "",
    bucket_lookup: "",
    bucket: "",
    access_key: "",
    secret_key: "",
    domain: "",
    path_prefix: ""
  });
  editVisible.value = true;
};
const openEdit = (row: StorageConfigRow) => {
  editMode.value = "update";
  editRow.value = row;
  Object.assign(editForm, {
    name: row.name,
    provider: row.provider,
    endpoint: row.endpoint,
    region: row.region,
    bucket_lookup: row.bucket_lookup || "",
    bucket: row.bucket,
    access_key: row.access_key,
    secret_key: "",
    domain: row.domain,
    path_prefix: row.path_prefix
  });
  editVisible.value = true;
};
const onSubmitEdit = async () => {
  // 写请求（adapters.configRequest）：更新保留响应面外字段（status/is_default/
  // max_file_size/allowed_types 沿用原行——列表响应不回显原文的字段不臆造）
  const base = {
    name: editForm.name,
    provider: editForm.provider,
    endpoint: editForm.endpoint,
    region: editForm.region,
    bucket_lookup: editForm.bucket_lookup || undefined,
    bucket: editForm.bucket,
    access_key: editForm.access_key,
    secret_key: editForm.secret_key,
    domain: editForm.domain,
    path_prefix: editForm.path_prefix
  };
  if (editMode.value === "create") {
    await storageApi.createConfig(base);
    ElMessage.success("存储配置已创建");
  } else if (editRow.value) {
    await storageApi.updateConfig(editRow.value.id, {
      ...base,
      status: editRow.value.status,
      is_default: editRow.value.is_default,
      max_file_size: editRow.value.max_file_size,
      allowed_types: editRow.value.allowed_types
    });
    ElMessage.success("已更新");
  }
  editVisible.value = false;
  await loadConfigs();
};

const onSetDefault = async (row: StorageConfigRow) => {
  await ElMessageBox.confirm(`确认将「${row.name}」设为默认存储配置？后续上传默认走该配置。`, "设默认确认", { type: "warning" });
  await storageApi.setDefault(row.id);
  ElMessage.success("已设为默认");
  await loadConfigs();
};

const onDelete = async (row: StorageConfigRow) => {
  await ElMessageBox.confirm(`确认删除存储配置「${row.name}」？删除后不可恢复。`, "删除确认", { type: "warning" });
  await storageApi.deleteConfig(row.id);
  ElMessage.success("已删除");
  await loadConfigs();
};

// —— 上传记录 tab ——
const recLoading = ref(false);
const recRows = ref<StorageRecordRow[]>([]);
const recTotal = ref(0);
const recQuery = reactive({ page: 1, page_size: 10 });
let recLoaded = false;

const loadRecords = async () => {
  recLoading.value = true;
  try {
    const resp: any = await storageApi.listRecords(recQuery.page, recQuery.page_size);
    recRows.value = resp.data.list;
    recTotal.value = resp.data.total;
    recLoaded = true;
  } finally {
    recLoading.value = false;
  }
};

const onTabChange = (tab: string | number) => {
  if (tab === "records" && !recLoaded) loadRecords();
};

const onCleanup = async () => {
  await ElMessageBox.confirm("确认清理所有过期上传记录？此操作不可恢复。", "清理确认", { type: "warning" });
  const resp: any = await storageApi.cleanup();
  ElMessage.success(`已清理 ${resp.data?.expired ?? 0} 条过期记录`);
  recQuery.page = 1;
  await loadRecords();
};

// 记录状态展示（1=pending 2=completed 3=expired）
const recStatusTag = (status: number): { label: string; type: "info" | "success" | "warning" } => {
  if (status === REC_COMPLETED) return { label: "已完成", type: "success" };
  if (status === REC_EXPIRED) return { label: "已过期", type: "warning" };
  if (status === REC_PENDING) return { label: "待确认", type: "info" };
  return { label: `未知(${status})`, type: "info" };
};

const formatSize = (bytes: number): string => {
  if (!bytes) return "—";
  const kb = bytes / 1024;
  if (kb < 1024) return `${kb.toFixed(1)} KB`;
  const mb = kb / 1024;
  if (mb < 1024) return `${mb.toFixed(1)} MB`;
  return `${(mb / 1024).toFixed(2)} GB`;
};

onMounted(loadConfigs);
</script>

<template>
  <div class="table-box">
    <el-card shadow="never" class="content-card">
      <el-tabs v-model="activeTab" @tab-change="onTabChange">
        <el-tab-pane label="存储配置" name="configs">
          <div class="page-header">
            <span class="page-title">存储配置</span>
            <div class="page-actions">
              <el-button v-auth="'admin.storage.configs.write'" type="primary" @click="openCreate">新建配置</el-button>
            </div>
          </div>

          <el-table v-loading="cfgLoading" :data="cfgRows" border stripe>
            <el-table-column prop="name" label="名称" min-width="120" show-overflow-tooltip />
            <el-table-column prop="provider" label="提供商" width="100" align="center" />
            <el-table-column prop="endpoint" label="端点" min-width="180" show-overflow-tooltip />
            <el-table-column prop="bucket" label="桶" min-width="120" show-overflow-tooltip />
            <el-table-column label="默认" width="80" align="center">
              <template #default="{ row }">
                <el-tag v-if="row.is_default" type="primary">默认</el-tag>
                <span v-else>—</span>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === CFG_ENABLED ? 'success' : 'danger'">{{
                  row.status === CFG_ENABLED ? "启用" : "禁用"
                }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="更新时间" min-width="150">
              <template #default="{ row }">{{ fmtTime(row.updated_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="210" fixed="right">
              <template #default="{ row }">
                <el-button v-auth="'admin.storage.configs.write'" size="small" @click="openEdit(row)">编辑</el-button>
                <el-button
                  v-auth="'admin.storage.configs.write'"
                  size="small"
                  :disabled="row.is_default"
                  @click="onSetDefault(row)"
                  >设默认</el-button
                >
                <el-button v-auth="'admin.storage.configs.write'" size="small" type="danger" plain @click="onDelete(row)"
                  >删除</el-button
                >
              </template>
            </el-table-column>
          </el-table>
          <div class="pagination-wrap">
            <el-pagination
              layout="total, sizes, prev, pager, next"
              :total="cfgTotal"
              :page-sizes="[10, 20, 50]"
              :page-size="cfgQuery.page_size"
              :current-page="cfgQuery.page"
              @current-change="
                (p: number) => {
                  cfgQuery.page = p;
                  loadConfigs();
                }
              "
              @size-change="
                (s: number) => {
                  cfgQuery.page_size = s;
                  cfgQuery.page = 1;
                  loadConfigs();
                }
              "
            />
          </div>
        </el-tab-pane>

        <el-tab-pane label="上传记录" name="records">
          <div class="page-header">
            <span class="page-title">上传记录</span>
            <div class="page-actions">
              <el-button v-auth="'admin.storage.cleanup'" type="danger" plain @click="onCleanup">清理过期记录</el-button>
            </div>
          </div>

          <el-table v-loading="recLoading" :data="recRows" border stripe>
            <el-table-column prop="file_name" label="文件名" min-width="150" show-overflow-tooltip />
            <el-table-column prop="object_key" label="对象键" min-width="200" show-overflow-tooltip />
            <el-table-column label="大小" width="100" align="right">
              <template #default="{ row }">{{ formatSize(row.file_size) }}</template>
            </el-table-column>
            <el-table-column prop="mime_type" label="MIME" min-width="120" show-overflow-tooltip>
              <template #default="{ row }">{{ row.mime_type || "—" }}</template>
            </el-table-column>
            <el-table-column label="状态" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="recStatusTag(row.status).type">{{ recStatusTag(row.status).label }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="上传时间" min-width="150">
              <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="完成时间" min-width="150">
              <template #default="{ row }">{{ fmtTime(row.completed_at) }}</template>
            </el-table-column>
          </el-table>
          <div class="pagination-wrap">
            <el-pagination
              layout="total, sizes, prev, pager, next"
              :total="recTotal"
              :page-sizes="[10, 20, 50]"
              :page-size="recQuery.page_size"
              :current-page="recQuery.page"
              @current-change="
                (p: number) => {
                  recQuery.page = p;
                  loadRecords();
                }
              "
              @size-change="
                (s: number) => {
                  recQuery.page_size = s;
                  recQuery.page = 1;
                  loadRecords();
                }
              "
            />
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <el-dialog
      v-model="editVisible"
      :title="editMode === 'create' ? '新建存储配置' : '编辑存储配置'"
      width="520"
      destroy-on-close
    >
      <el-form label-width="100">
        <el-form-item label="名称" required>
          <el-input v-model="editForm.name" placeholder="配置名称" />
        </el-form-item>
        <el-form-item label="提供商" required>
          <el-select v-model="editForm.provider" style="width: 100%">
            <el-option label="MinIO" value="minio" />
            <el-option label="S3 兼容" value="s3" />
          </el-select>
        </el-form-item>
        <el-form-item label="端点" required>
          <el-input v-model="editForm.endpoint" placeholder="如 http:/7.0.0.1:9000" />
        </el-form-item>
        <el-form-item label="地域">
          <el-input v-model="editForm.region" placeholder="可空" />
        </el-form-item>
        <el-form-item label="寻址风格">
          <el-select v-model="editForm.bucket_lookup" style="width: 100%" clearable placeholder="默认 path">
            <el-option label="path（路径风格）" value="path" />
            <el-option label="virtual-host（虚拟主机）" value="virtual-host" />
          </el-select>
        </el-form-item>
        <el-form-item label="桶" required>
          <el-input v-model="editForm.bucket" placeholder="bucket 名称" />
        </el-form-item>
        <el-form-item label="AccessKey" required>
          <el-input v-model="editForm.access_key" />
        </el-form-item>
        <el-form-item label="SecretKey" required>
          <el-input
            v-model="editForm.secret_key"
            type="password"
            show-password-on="click"
            :placeholder="editMode === 'update' ? '掩码不回显——更新需重填' : '访问密钥'"
          />
        </el-form-item>
        <el-form-item label="自定义域名">
          <el-input v-model="editForm.domain" placeholder="公开读场景 URL 前缀；空=端点" />
        </el-form-item>
        <el-form-item label="路径前缀">
          <el-input v-model="editForm.path_prefix" placeholder="可空；仅 [a-zA-Z0-9/_-]，≤64" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="onSubmitEdit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>
