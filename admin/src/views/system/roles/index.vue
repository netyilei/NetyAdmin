<script setup lang="ts" name="roles">
// RBAC 角色权限页：角色列表（分页）+ 菜单授权树（全量替换语义）
// + 新建/删除。super-admin 摘除/自操作已被后端防呆拒绝（前端禁用双保险）。
import { ElMessage, ElMessageBox } from "element-plus";
import { nextTick, onMounted, reactive, ref } from "vue";

import { rbacApi, type MenuRow, type RoleRow } from "@/api/modules/rbac";

const loading = ref(false);
const roles = ref<RoleRow[]>([]);
const total = ref(0);
const query = reactive({ page: 1, page_size: 10 });

const load = async () => {
  loading.value = true;
  try {
    const resp: any = await rbacApi.roles(query.page, query.page_size);
    roles.value = resp.data.list;
    total.value = resp.data.total;
  } finally {
    loading.value = false;
  }
};
onMounted(load);

// —— 授权抽屉（菜单树勾选——全量替换）——
const grantVisible = ref(false);
const grantRole = ref<RoleRow | null>(null);
const grantTree = ref<MenuRow[]>([]);
// checkedKeys=勾选集（保存用，含 halfChecked 父——P1-13 半选保留语义）。
// 初始勾选=非响应式快照数组 treeCheckedSnapshot（仅叶 id）+default-checked-keys 稳定
// 引用绑定（真机回归三轮根修定稿）：①绑响应式 ref→@check 更新即重设勾选（取消被回弹）；
// ②快照仅叶 id（含目录 id 会级联全选子树拉回已取消叶）→挂载期应用+交互零回弹。
// 父目录勾选视觉：程序化恢复后 EP 不保证重算父链渲染态（真实机 3/4 子叶时父目录
// 显示未勾）——由下方 syncRestoredDirState 显式回写，交互态仍走 el-tree 原生联动。
const treeCheckedSnapshot: string[] = [];
const checkedKeys = ref<string[]>([]);
const grantLoading = ref(false);
const grantTreeRef = ref<any>();

// 挂载恢复态父目录半选/全选视觉：程序化恢复
// （default-checked-keys 叶快照）后 EP 父链渲染态未可靠重算——按真实子树叶子
// 勾选回写目录节点 checked/indeterminate（仅视觉态同步；保存数据仍由 @check
// 的 checkedKeys/halfCheckedKeys 驱动，用户点选后 EP 原生联动接管，零回弹）。
const syncRestoredDirState = () => {
  const root = grantTreeRef.value?.store?.root;
  if (!root?.childNodes?.length) return false;
  const leafState = (node: any): "all" | "none" | "half" => {
    const leaves: any[] = [];
    const collect = (n: any) => (n.childNodes.length ? n.childNodes.forEach(collect) : leaves.push(n));
    collect(node);
    const checked = leaves.filter((l: any) => l.checked).length;
    return checked === 0 ? "none" : checked === leaves.length ? "all" : "half";
  };
  const walk = (node: any) => {
    if (!node.childNodes.length) return;
    node.childNodes.forEach(walk);
    const s = leafState(node);
    const checked = s === "all";
    const indeterminate = s === "half";
    if (node.checked !== checked) node.checked = checked;
    if (node.indeterminate !== indeterminate) node.indeterminate = indeterminate;
  };
  root.childNodes.forEach(walk);
  return true;
};

const onGrant = async (row: RoleRow) => {
  grantRole.value = row;
  grantLoading.value = true;
  grantVisible.value = true;
  try {
    const [treeResp, grantsResp, apisResp, buttonsResp] = await Promise.all([
      rbacApi.menuTree(),
      rbacApi.roleGrants(row.id).catch(() => ({ data: { menus: [], apis: [], buttons: [] } })),
      rbacApi.apis(1, 500),
      rbacApi.buttons().catch(() => ({ data: [] }))
    ]);
    grantTree.value = treeResp.data;
    const menus = (grantsResp as any).data?.menus ?? [];
    checkedKeys.value = menus; // 保存用全量（含 halfChecked 目录 id——后端全量语义）
    // 快照只填叶 id（挂载期 setCheckedKeys 叶驱动——父目录由 el-tree 按子集自动计算
    // 半选；若含目录 id 会级联全选子树，把后端已取消的叶子拉回）。同一数组引用填充
    // ——模板绑定引用稳定，挂载期一次应用，交互后零回弹。
    const dirIds = new Set<string>();
    const walk = (ns: MenuRow[]) =>
      ns.forEach(m => {
        if (m.children && m.children.length) {
          dirIds.add(m.id);
          walk(m.children);
        }
      });
    walk(grantTree.value);
    treeCheckedSnapshot.length = 0;
    treeCheckedSnapshot.push(...menus.filter((k: string) => !dirIds.has(k)));

    // 等 el-tree 应用 data+default-checked-keys 后回写父目录视觉态（见上注释；
    // 抽屉首开 transition 可能晚一帧挂载，补一次重试）
    await nextTick();
    if (!syncRestoredDirState()) {
      await nextTick();
      syncRestoredDirState();
    }

    // API 树（method+path 分组——act 为叶）
    const apiList: any[] = (apisResp as any).data;
    const byPath = new Map<string, { id: string; label: string; children: any[] }>();
    for (const a of apiList) {
      if (a.act === "*") continue;
      const leaf = { id: a.id, label: `${a.method} ${a.act}` };
      const grp: any = byPath.get(a.path) ?? { id: "grp-" + a.path, label: a.path, children: [] };
      grp.children.push(leaf);
      byPath.set(a.path, grp);
    }
    apiTree.value = [...byPath.values()];
    checkedApiKeys.value = (grantsResp as any).data?.apis ?? [];
    buttonRows.value = (buttonsResp as any).data;
    checkedButtonIds.value = (grantsResp as any).data?.buttons ?? [];
  } finally {
    grantLoading.value = false;
  }
};

const onSaveGrants = async () => {
  if (!grantRole.value) return;
  await Promise.all([
    rbacApi.setRoleMenus(grantRole.value.id, checkedKeys.value),
    rbacApi.setRoleApis(grantRole.value.id, checkedApiKeys.value),
    rbacApi.setRoleButtons(grantRole.value.id, checkedButtonIds.value)
  ]);
  ElMessage.success("授权已保存（reload 即生效）");
  grantVisible.value = false;
};

const grantTab = ref("menus");
const apiTree = ref<any[]>([]);
const checkedApiKeys = ref<string[]>([]);
const buttonRows = ref<any[]>([]);
const checkedButtonIds = ref<string[]>([]);

// —— 新建/编辑 ——
const editVisible = ref(false);
const editMode = ref<"create" | "update">("create");
const editRow = ref<RoleRow | null>(null);
const editForm = reactive({ code: "", name: "", remark: "" });

const openCreate = () => {
  editMode.value = "create";
  editRow.value = null;
  Object.assign(editForm, { code: "", name: "", remark: "" });
  editVisible.value = true;
};
const openEdit = (row: RoleRow) => {
  editMode.value = "update";
  editRow.value = row;
  Object.assign(editForm, { code: row.code, name: row.name, remark: row.remark });
  editVisible.value = true;
};
const onSubmitEdit = async () => {
  if (editMode.value === "create") {
    await rbacApi.createRole({ ...editForm });
    ElMessage.success("角色已创建（绑定管理员后生效）");
  } else if (editRow.value) {
    // code 随更新提交（后端 roleRequest required——编辑弹窗已回填原值，角色码不可改）
    await rbacApi.updateRole(editRow.value.id, { code: editForm.code, name: editForm.name, remark: editForm.remark });
    ElMessage.success("已更新");
  }
  editVisible.value = false;
  await load();
};

const onDelete = async (row: RoleRow) => {
  if (row.code === "super-admin") {
    ElMessage.error("super-admin 为全权通配载体（写保护）");
    return;
  }
  await ElMessageBox.confirm(`确认删除角色「${row.name}」？绑定关系将一并清除。`, "删除确认", { type: "warning" });
  await rbacApi.deleteRole(row.id);
  ElMessage.success("已删除");
  await load();
};

const pageChange = (p: number) => {
  query.page = p;
  load();
};
</script>

<template>
  <div class="table-box">
    <el-card shadow="never" class="content-card">
      <div class="page-header">
        <span class="page-title">角色权限</span>
        <div class="page-actions">
          <el-button v-auth="'admin.roles.create'" type="primary" @click="openCreate">新建角色</el-button>
        </div>
      </div>

      <el-table v-loading="loading" :data="roles" border stripe>
        <el-table-column prop="code" label="角色码" min-width="130" show-overflow-tooltip />
        <el-table-column prop="name" label="名称" min-width="150" show-overflow-tooltip />
        <el-table-column prop="remark" label="备注" min-width="200" show-overflow-tooltip />
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? "启用" : "禁用" }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" type="primary" @click="onGrant(row)">菜单授权</el-button>
            <el-button
              v-auth="'admin.roles.delete'"
              size="small"
              type="danger"
              :disabled="row.code === 'super-admin'"
              @click="onDelete(row)"
            >
              删除
            </el-button>
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
          @current-change="pageChange"
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

    <el-drawer v-model="grantVisible" :title="`授权：${grantRole?.name ?? ''}`" size="460" destroy-on-close>
      <el-tabs v-model="grantTab">
        <el-tab-pane label="菜单" name="menus">
          <el-tree
            ref="grantTreeRef"
            v-loading="grantLoading"
            :data="grantTree"
            show-checkbox
            node-key="id"
            default-expand-all
            :props="{ label: 'name' }"
            :default-checked-keys="treeCheckedSnapshot"
            @check="(d: any, s: any) => (checkedKeys = [...s.checkedKeys, ...s.halfCheckedKeys])"
          />
        </el-tab-pane>
        <el-tab-pane label="API 权限" name="apis">
          <el-tree
            v-loading="grantLoading"
            :data="apiTree"
            show-checkbox
            node-key="id"
            default-expand-all
            :props="{ label: 'label' }"
            :default-checked-keys="checkedApiKeys"
            @check="(d: any, s: any) => (checkedApiKeys = s.checkedKeys)"
          />
        </el-tab-pane>
        <el-tab-pane label="按钮权限" name="buttons">
          <el-checkbox-group v-model="checkedButtonIds" class="btn-grid">
            <el-checkbox v-for="b in buttonRows" :key="b.id" :value="b.id" :label="b.name" border />
          </el-checkbox-group>
        </el-tab-pane>
      </el-tabs>
      <template #footer>
        <el-button @click="grantVisible = false">取消</el-button>
        <el-button type="primary" @click="onSaveGrants">保存授权</el-button>
      </template>
    </el-drawer>

    <el-dialog v-model="editVisible" :title="editMode === 'create' ? '新建角色' : '编辑角色'" width="440" destroy-on-close>
      <el-form label-width="70">
        <el-form-item label="角色码" required>
          <el-input v-model="editForm.code" :disabled="editMode === 'update'" placeholder="如 ops-team" />
        </el-form-item>
        <el-form-item label="名称" required>
          <el-input v-model="editForm.name" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="editForm.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="onSubmitEdit">确定</el-button>
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

<style scoped lang="scss">
.btn-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
}
</style>
