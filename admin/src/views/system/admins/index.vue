<script setup lang="ts" name="admins">
// 管理员管理：列表/新建/编辑/删除/启停/重置密码/角色绑定。
// 按钮权限=v-auth 权限码（admin.admins.*——与 rbac_apis 同源）。
import { ElMessage, ElMessageBox } from "element-plus";
import type { FormInstance, FormRules } from "element-plus";
import { onMounted, reactive, ref } from "vue";

import { ArrowDown } from "@element-plus/icons-vue";
import { adminApi, type AdminRow } from "@/api/modules/identity";
import { rbacApi, type RoleRow } from "@/api/modules/rbac";
import { fmtTime } from "@/utils/format";

const loading = ref(false);
const rows = ref<AdminRow[]>([]);
const total = ref(0);
const query = reactive({ page: 1, page_size: 10, username: "" });

const load = async () => {
  loading.value = true;
  try {
    const resp: any = await adminApi.list(query.page, query.page_size, {
      username: query.username || undefined
    });
    rows.value = resp.data.list;
    total.value = resp.data.total;
  } finally {
    loading.value = false;
  }
};
onMounted(load);

const onSearch = () => {
  query.page = 1;
  load();
};

// —— 新建/编辑 ——
const editVisible = ref(false);
const editMode = ref<"create" | "update">("create");
const editRow = ref<AdminRow | null>(null);
const editFormRef = ref<FormInstance>();
const editForm = reactive({ username: "", password: "", name: "", email: "" });
// 编辑邮箱是否已取到原文（详情按 admin.admins.get 权限）——未取到时提交不带
// email（后端 nil=不改该列），防把列表掩码串回写（PII 掩码批）
const editEmailKnown = ref(false);
// 表单校验前置（①/G2：密码≥8/姓名必填/邮箱格式，提交前拦截免 400 裸弹错；
// 用户名 3-32 与后端 identity 契约同源）
const editRules = reactive<FormRules>({
  username: [
    { required: true, message: "请输入用户名", trigger: "blur" },
    { min: 3, max: 32, message: "用户名长度须为 3-32 位", trigger: "blur" }
  ],
  password: [
    { required: true, message: "请输入初始密码", trigger: "blur" },
    { min: 8, message: "密码至少 8 位", trigger: "blur" }
  ],
  name: [{ required: true, message: "请输入姓名", trigger: "blur" }],
  email: [{ type: "email", message: "邮箱格式不正确", trigger: "blur" }]
});

// 打开表单的请求序号守卫（编辑详情异步取回；快速连点不同行时响应
// 乱序会把 A 的详情/邮箱灌进 B 的表单，保存即交叉改写——发起即取号，落地前
// 比对序号，过期响应整段丢弃。对齐 gwscope loadGrantedApis 的漂移丢弃先例）
let editSeq = 0;
const openCreate = () => {
  editSeq++; // 作废可能在途的编辑详情响应
  editMode.value = "create";
  editRow.value = null;
  editEmailKnown.value = true; // 新建=用户输入值，非回写原文
  Object.assign(editForm, { username: "", password: "", name: "", email: "" });
  editVisible.value = true;
};
const openEdit = async (row: AdminRow) => {
  const seq = ++editSeq;
  // 列表 email 已掩码（PII）——编辑表单取详情原文（admin.admins.get 权限）；
  // 无权限时留空且提交不带 email（后端 nil=不改该列，防掩码串回写）
  let email = "";
  let known = false;
  try {
    const resp: any = await adminApi.get(row.id);
    if (typeof resp?.data?.email === "string") {
      email = resp.data.email;
      known = true;
    }
  } catch {
    /* 详情不可得：留空不改 */
  }
  if (seq !== editSeq) return; // 期间已发起其他编辑/新建——丢弃过期响应
  editMode.value = "update";
  editRow.value = row;
  editEmailKnown.value = known;
  Object.assign(editForm, { username: row.username, password: "", name: row.name, email });
  editVisible.value = true;
};
const onSubmitEdit = async () => {
  const ok = await editFormRef.value
    ?.validate()
    .then(() => true)
    .catch(() => false);
  if (!ok) return;
  if (editMode.value === "create") {
    await adminApi.create({ ...editForm });
    ElMessage.success("管理员已创建（首登强制修改密码）");
  } else if (editRow.value) {
    const payload: { name: string; email?: string } = { name: editForm.name };
    if (editEmailKnown.value || editForm.email !== "") payload.email = editForm.email;
    await adminApi.update(editRow.value.id, payload);
    ElMessage.success("已更新");
  }
  editVisible.value = false;
  await load();
};

const onToggleStatus = async (row: AdminRow) => {
  await adminApi.setStatus(row.id, row.status === 1 ? 2 : 1);
  ElMessage.success(row.status === 1 ? "已禁用（即时下线）" : "已启用");
  await load();
};

const onDelete = async (row: AdminRow) => {
  await ElMessageBox.confirm(`确认删除管理员「${row.username}」？`, "删除确认", { type: "warning" });
  await adminApi.delete(row.id);
  ElMessage.success("已删除");
  await load();
};

const onResetPassword = async (row: AdminRow) => {
  await ElMessageBox.confirm(`确认重置「${row.username}」的密码？旧密码立即失效。`, "重置确认", { type: "warning" });
  const resp: any = await adminApi.resetPassword(row.id);
  ElMessageBox.alert(`新密码（一次性显示）：${resp.data?.password}`, "重置成功", { type: "warning" });
};

const onRowCommand = async (cmd: string, row: AdminRow) => {
  if (cmd === "disable" || cmd === "enable") await onToggleStatus(row);
  else if (cmd === "reset") await onResetPassword(row);
  else if (cmd === "delete") await onDelete(row);
};

// —— 角色绑定 ——
const rolesVisible = ref(false);
const rolesTarget = ref<AdminRow | null>(null);
const allRoles = ref<RoleRow[]>([]);
const boundRoleIds = ref<string[]>([]);

// 角色弹窗同型竞态守卫（同型：两次 onRoles 并发时 boundRoleIds 是
// 全局响应落地——后到旧响应会把 A 的角色集灌给 B；序号漂移即丢弃，弹窗不打开）
let rolesSeq = 0;
const onRoles = async (row: AdminRow) => {
  const seq = ++rolesSeq;
  const [rolesResp, boundResp] = await Promise.all([
    rbacApi.roles(1, 100),
    adminApi.roles(row.id).catch(() => ({ data: { roles: [] } }))
  ]);
  if (seq !== rolesSeq) return; // 期间已打开其他行——丢弃过期响应
  rolesTarget.value = row;
  allRoles.value = (rolesResp as any).data.list;
  boundRoleIds.value = (boundResp as any).data.roles;
  rolesVisible.value = true;
};
const onSaveRoles = async () => {
  if (!rolesTarget.value) return;
  await adminApi.setRoles(rolesTarget.value.id, boundRoleIds.value);
  ElMessage.success("角色绑定已保存");
  rolesVisible.value = false;
};
</script>

<template>
  <div class="table-box">
    <el-card shadow="never" class="content-card">
      <div class="page-header">
        <span class="page-title">管理员</span>
        <div class="page-actions">
          <el-input
            v-model="query.username"
            placeholder="按用户名搜索"
            clearable
            style="width: 200px"
            @keyup.enter="onSearch"
            @clear="onSearch"
          />
          <el-button @click="onSearch">搜索</el-button>
          <el-button v-auth="'admin.admins.create'" type="primary" @click="openCreate">新建管理员</el-button>
        </div>
      </div>

      <el-table v-loading="loading" :data="rows" border stripe>
        <el-table-column label="头像" width="80" align="center">
          <template #default="{ row }">
            <el-avatar :size="36" :src="row.avatarUrl || undefined" icon="UserFilled" />
          </template>
        </el-table-column>
        <el-table-column prop="username" label="用户名" min-width="130" show-overflow-tooltip />
        <el-table-column prop="name" label="姓名" min-width="110" show-overflow-tooltip />
        <el-table-column prop="email" label="邮箱" min-width="160" show-overflow-tooltip />
        <el-table-column prop="phone" label="手机号" min-width="130" show-overflow-tooltip>
          <template #default="{ row }">{{ row.phone || "—" }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? "启用" : "禁用" }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="首登改密" width="100" align="center">
          <template #default="{ row }">{{ row.mustChangePassword ? "是" : "否" }}</template>
        </el-table-column>
        <el-table-column label="最后登录" min-width="150">
          <template #default="{ row }">{{ fmtTime(row.lastLoginAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button v-auth="'admin.admins.update'" size="small" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" @click="onRoles(row)">角色</el-button>
            <el-dropdown trigger="click" @command="(cmd: string) => onRowCommand(cmd, row)">
              <el-button size="small">
                更多<el-icon class="el-icon--right"><ArrowDown /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item v-if="row.status === 1" command="disable">禁用</el-dropdown-item>
                  <el-dropdown-item v-else command="enable">启用</el-dropdown-item>
                  <el-dropdown-item v-auth="'admin.admins.reset_password'" command="reset">重置密码</el-dropdown-item>
                  <el-dropdown-item v-auth="'admin.admins.delete'" command="delete" divided>删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
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

    <el-dialog v-model="editVisible" :title="editMode === 'create' ? '新建管理员' : '编辑管理员'" width="480" destroy-on-close>
      <el-form ref="editFormRef" :model="editForm" :rules="editRules" label-width="90">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="editForm.username" :disabled="editMode === 'update'" placeholder="3-32 位" />
        </el-form-item>
        <el-form-item v-if="editMode === 'create'" label="初始密码" prop="password">
          <el-input v-model="editForm.password" type="password" show-password-on="click" placeholder="≥8 位（首登强制修改）" />
        </el-form-item>
        <el-form-item label="姓名" prop="name">
          <el-input v-model="editForm.name" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="editForm.email" placeholder="选填" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="onSubmitEdit">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="rolesVisible" :title="`角色绑定：${rolesTarget?.username ?? ''}`" width="440" destroy-on-close>
      <el-checkbox-group v-model="boundRoleIds">
        <el-checkbox v-for="r in allRoles" :key="r.id" :value="r.id" :label="r.name" border class="w-full mb-2" />
      </el-checkbox-group>
      <template #footer>
        <el-button @click="rolesVisible = false">取消</el-button>
        <el-button type="primary" @click="onSaveRoles">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.mb-2 {
  margin-bottom: 8px;
}
.mt-2 {
  margin-top: 12px;
}
.flex-center {
  display: flex;
  justify-content: center;
}
</style>
