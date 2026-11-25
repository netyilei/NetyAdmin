<script setup lang="ts" name="apis">
// API 权限登记管理（rbac_apis——method/path/act 权限码；判定面与
// 登记面单一语义——check-rbac-perms 门禁保证路由声明↔登记对偶）。
import { ElMessage, ElMessageBox } from "element-plus";
import { onMounted, reactive, ref } from "vue";

import { rbacApi, type ApiRow } from "@/api/modules/rbac";

const loading = ref(false);
const rows = ref<ApiRow[]>([]);
const total = ref(0);
const query = reactive({ page: 1, page_size: 10, act: "" });

const allRows = ref<ApiRow[]>([]);
const load = async () => {
  loading.value = true;
  try {
    const resp: any = await rbacApi.apis(1, 500);
    allRows.value = resp.data;
    onSearch();
  } finally {
    loading.value = false;
  }
};
// act 前端过滤（后端 list 全量返回无 act query——复：搜索框语义兑现）
const onSearch = () => {
  const kw = query.act.trim();
  const filtered = kw ? allRows.value.filter(r => r.act.includes(kw)) : allRows.value;
  total.value = filtered.length;
  const start = (query.page - 1) * query.page_size;
  rows.value = filtered.slice(start, start + query.page_size);
};
onMounted(load);

const createVisible = ref(false);
const createForm = reactive({ method: "GET", path: "", act: "", summary: "" });

const onCreate = async () => {
  await rbacApi.createApi({ ...createForm });
  ElMessage.success("API 登记已创建");
  createVisible.value = false;
  await load();
};

const onDelete = async (row: ApiRow) => {
  await ElMessageBox.confirm(`确认删除登记「${row.method} ${row.path}（${row.act}）」？`, "删除确认", { type: "warning" });
  await rbacApi.deleteApi(row.id);
  ElMessage.success("已删除");
  await load();
};
</script>

<template>
  <div class="table-box">
    <el-card shadow="never" class="content-card">
      <div class="page-header">
        <span class="page-title">API 权限登记</span>
        <div class="page-actions">
          <el-input
            v-model="query.act"
            placeholder="按权限码搜索"
            clearable
            style="width: 200px"
            @keyup.enter="onSearch"
            @clear="onSearch"
          />
          <el-button @click="onSearch">搜索</el-button>
          <el-button v-auth="'admin.apis.create'" type="primary" @click="createVisible = true">新建登记</el-button>
        </div>
      </div>

      <el-table v-loading="loading" :data="rows" border stripe>
        <el-table-column prop="method" label="方法" width="90" align="center">
          <template #default="{ row }">
            <el-tag size="small">{{ row.method }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="path" label="路由" min-width="240" show-overflow-tooltip />
        <el-table-column prop="act" label="权限码" min-width="220" show-overflow-tooltip />
        <el-table-column prop="summary" label="描述" min-width="160" show-overflow-tooltip />
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button v-auth="'admin.apis.delete'" size="small" type="danger" @click="onDelete(row)">删除</el-button>
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

    <el-dialog v-model="createVisible" title="新建 API 登记" width="480" destroy-on-close>
      <el-form label-width="80">
        <el-form-item label="方法" required>
          <el-select v-model="createForm.method" style="width: 120px">
            <el-option v-for="m in ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']" :key="m" :value="m" :label="m" />
          </el-select>
        </el-form-item>
        <el-form-item label="路由" required>
          <el-input v-model="createForm.path" placeholder="/admin/v1/xxx/:id" />
        </el-form-item>
        <el-form-item label="权限码" required>
          <el-input v-model="createForm.act" placeholder="admin.xxx.yyy（与路由声明同源）" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="createForm.summary" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" @click="onCreate">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.mt-2 {
  margin-top: 12px;
}
.flex-center {
  display: flex;
  justify-content: center;
}
</style>
