<script setup lang="ts" name="users">
// 尾项：users 用户管理页（消费 US-01 Port——admin.users.list/disable；
// 用户 2026-11-19四 API：注册/登录/改用户名/改手机号——管理面=列表+禁用）。
import { ElMessage, ElMessageBox } from "element-plus";
import { onMounted, reactive, ref } from "vue";

import http from "@/api";

interface UserRow {
  id: string;
  username: string;
  email: string;
  phone: string;
  status: number;
  token_version: number; // 后端 users 域 json tag snake_case
  created_at: string;
}

const loading = ref(false);
const rows = ref<UserRow[]>([]);
const total = ref(0);
const query = reactive({ page: 1, page_size: 10 });

const load = async () => {
  loading.value = true;
  try {
    const resp: any = await http.get(`/admin/v1/users?page=${query.page}&page_size=${query.page_size}`);
    rows.value = resp.data.list;
    total.value = resp.data.total;
  } finally {
    loading.value = false;
  }
};
onMounted(load);

const onToggleStatus = async (row: UserRow) => {
  const next = row.status === 1 ? 2 : 1;
  await ElMessageBox.confirm(
    next === 2 ? `确认禁用「${row.username}」？（TokenVersion++ 即时下线）` : `确认启用「${row.username}」？`,
    "状态变更",
    { type: "warning" }
  );
  // 后端注册 u.PUT /:id/status（此前 PATCH 方法错位必 405）
  await http.put(`/admin/v1/users/${row.id}/status`, { status: next });
  ElMessage.success(next === 2 ? "已禁用" : "已启用");
  await load();
};
</script>

<template>
  <div class="table-box">
    <div class="card content-box">
      <div class="page-header">
        <span class="page-title">用户管理</span>
        <div class="page-actions">
          <span class="text-xs" style="color: var(--el-text-color-secondary)"
            >C 端演示模块（US-01——users.enabled=true 时有数据）</span
          >
        </div>
      </div>
      <el-table v-loading="loading" :data="rows" border stripe>
        <el-table-column prop="username" label="用户名" min-width="140" show-overflow-tooltip />
        <el-table-column prop="email" label="邮箱" min-width="180" show-overflow-tooltip />
        <el-table-column prop="phone" label="手机号" min-width="130" show-overflow-tooltip />
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? "启用" : "禁用" }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="token_version" label="TV" width="70" align="center" />
        <el-table-column label="操作" width="110" fixed="right">
          <template #default="{ row }">
            <el-button
              v-auth="'admin.users.disable'"
              size="small"
              :type="row.status === 1 ? 'warning' : 'success'"
              @click="onToggleStatus(row)"
            >
              {{ row.status === 1 ? "禁用" : "启用" }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pagination-wrap">
        <el-pagination
          layout="total, prev, pager, next"
          :total="total"
          :page-size="query.page_size"
          :current-page="query.page"
          @current-change="
            (p: number) => {
              query.page = p;
              load();
            }
          "
        />
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.pagination-wrap {
  display: flex;
  justify-content: center;
  margin-top: 12px;
}
</style>
