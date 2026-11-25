<script setup lang="ts" name="menus">
// 菜单管理（URL/路径/component/icon 管理——route-tree 数据源，
// 保存后 reload 即生效=动态路由实时更新）。
import { ElMessage, ElMessageBox } from "element-plus";
import { onMounted, reactive, ref } from "vue";

import { rbacApi, type MenuRow } from "@/api/modules/rbac";

const loading = ref(false);
const tree = ref<MenuRow[]>([]);

const load = async () => {
  loading.value = true;
  try {
    const resp: any = await rbacApi.menus();
    tree.value = resp.data;
  } finally {
    loading.value = false;
  }
};
onMounted(load);

const editVisible = ref(false);
const editMode = ref<"create" | "update">("create");
const editForm = reactive({
  id: "",
  parentId: "",
  name: "",
  path: "",
  component: "",
  icon: "",
  sortOrder: 0,
  hidden: false,
  status: 1
});

const openCreate = (parentId = "") => {
  editMode.value = "create";
  Object.assign(editForm, {
    id: "",
    parentId,
    name: "",
    path: "",
    component: "",
    icon: "",
    sortOrder: 0,
    hidden: false,
    status: 1
  });
  editVisible.value = true;
};
const openEdit = (row: MenuRow) => {
  editMode.value = "update";
  Object.assign(editForm, { ...row });
  editVisible.value = true;
};
const onSubmit = async () => {
  const payload = { ...editForm };
  if (editMode.value === "create") {
    await rbacApi.createMenu(payload);
    ElMessage.success("菜单已创建（route-tree 即时更新）");
  } else {
    await rbacApi.updateMenu(editForm.id, payload);
    ElMessage.success("菜单已更新");
  }
  editVisible.value = false;
  await load();
};

const onDelete = async (row: MenuRow) => {
  await ElMessageBox.confirm(`确认删除菜单「${row.name}」？`, "删除确认", { type: "warning" });
  await rbacApi.deleteMenu(row.id);
  ElMessage.success("已删除");
  await load();
};
</script>

<template>
  <div class="table-box">
    <el-card shadow="never" class="content-card">
      <div class="page-header">
        <span class="page-title">菜单管理</span>
        <div class="page-actions">
          <el-button v-auth="'admin.menus.create'" type="primary" @click="openCreate()">新建根菜单</el-button>
        </div>
      </div>

      <el-table v-loading="loading" :data="tree" row-key="id" border default-expand-all>
        <el-table-column prop="name" label="名称" min-width="160" />
        <el-table-column prop="path" label="路由路径" min-width="150" />
        <el-table-column prop="component" label="视图路径" min-width="180" show-overflow-tooltip />
        <el-table-column prop="icon" label="图标" width="110" />
        <el-table-column prop="module" label="所属模块" width="110" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.module" size="small" type="info">{{ row.module }}</el-tag>
            <span v-else>常显</span>
          </template>
        </el-table-column>
        <el-table-column prop="sortOrder" label="排序" width="70" align="center" />
        <el-table-column label="隐藏" width="70" align="center">
          <template #default="{ row }">{{ row.hidden ? "是" : "否" }}</template>
        </el-table-column>
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? "启用" : "禁用" }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="215" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openCreate(row.id)">加子项</el-button>
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="onDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="editVisible" :title="editMode === 'create' ? '新建菜单' : '编辑菜单'" width="520" destroy-on-close>
      <el-form label-width="90">
        <el-form-item label="名称" required>
          <el-input v-model="editForm.name" />
        </el-form-item>
        <el-form-item label="路由路径">
          <el-input v-model="editForm.path" placeholder="如 /system/admins" />
        </el-form-item>
        <el-form-item label="视图路径">
          <el-input v-model="editForm.component" placeholder="如 /system/admins/index（src/views 下）" />
        </el-form-item>
        <el-form-item label="图标">
          <el-input v-model="editForm.icon" placeholder="Element Plus 图标名" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="editForm.sortOrder" />
        </el-form-item>
        <el-form-item label="隐藏">
          <el-switch v-model="editForm.hidden" />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="editForm.status" :active-value="1" :inactive-value="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="onSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>
