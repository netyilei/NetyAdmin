<script setup lang="ts" name="buttons">
// 按钮权限管理（rbac_buttons——菜单归属按钮码；授权随角色 role_buttons）。
import { ElMessage, ElMessageBox } from "element-plus";
import { onMounted, reactive, ref } from "vue";

import { rbacApi, type ButtonRow, type MenuRow } from "@/api/modules/rbac";

const loading = ref(false);
const rows = ref<ButtonRow[]>([]);
const menus = ref<MenuRow[]>([]);
const filterMenu = ref("");

const load = async () => {
  loading.value = true;
  try {
    const resp: any = await rbacApi.buttons();
    rows.value = resp.data;
    const m: any = await rbacApi.menus();
    menus.value = m.data;
  } finally {
    loading.value = false;
  }
};
onMounted(load);

const menuName = (menuId: string): string => menus.value.find(m => m.id === menuId)?.name ?? menuId;

const createVisible = ref(false);
const createForm = reactive({ menuId: "", code: "", name: "" });

const onCreate = async () => {
  await rbacApi.createButton({ ...createForm });
  ElMessage.success("按钮已创建");
  createVisible.value = false;
  await load();
};

const onDelete = async (row: ButtonRow) => {
  await ElMessageBox.confirm(`确认删除按钮「${row.name}（${row.code}）」？`, "删除确认", { type: "warning" });
  await rbacApi.deleteButton(row.id);
  ElMessage.success("已删除");
  await load();
};

const filtered = (): ButtonRow[] => (filterMenu.value ? rows.value.filter(r => r.menuId === filterMenu.value) : rows.value);
</script>

<template>
  <div class="table-box">
    <el-card shadow="never" class="content-card">
      <div class="page-header">
        <span class="page-title">按钮权限</span>
        <div class="page-actions">
          <el-select v-model="filterMenu" placeholder="按菜单过滤" clearable style="width: 200px">
            <el-option v-for="m in menus" :key="m.id" :value="m.id" :label="m.name" />
          </el-select>
          <el-button v-auth="'admin.buttons.create'" type="primary" @click="createVisible = true">新建按钮</el-button>
        </div>
      </div>

      <el-table v-loading="loading" :data="filtered()" border stripe>
        <el-table-column label="归属菜单" min-width="140">
          <template #default="{ row }">{{ menuName(row.menuId) }}</template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column prop="code" label="权限码" min-width="220" show-overflow-tooltip />
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button v-auth="'admin.buttons.delete'" size="small" type="danger" @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="createVisible" title="新建按钮权限" width="440" destroy-on-close>
      <el-form label-width="80">
        <el-form-item label="归属菜单" required>
          <el-select v-model="createForm.menuId" style="width: 100%">
            <el-option v-for="m in menus" :key="m.id" :value="m.id" :label="m.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="名称" required>
          <el-input v-model="createForm.name" placeholder="如新建管理员" />
        </el-form-item>
        <el-form-item label="权限码" required>
          <el-input v-model="createForm.code" placeholder="admin.xxx.yyy" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" @click="onCreate">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>
