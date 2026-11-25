<script setup lang="ts" name="configs">
// 系统配置页：键值编辑 + 新增键值（upsert）+ 回滚（装-A2 端点消费）。
// PUT 契约=items 全量提交（group/key/value/valueType/description/isPublic）
// ——整批单版本落历史+广播热更（评审 P0-2 语义）。
import { ElMessage, ElMessageBox } from "element-plus";
import { onMounted, reactive, ref } from "vue";

import { configApi, type ConfigRow } from "@/api/modules/config";

const VALUE_TYPES = [
  { value: 1, label: "字符串" },
  { value: 2, label: "整数" },
  { value: 3, label: "布尔" },
  { value: 4, label: "JSON" }
];

const loading = ref(false);
const rows = ref<ConfigRow[]>([]);
// 编辑缓冲：以 group+key 为定位写入 value/description/isPublic（提交时组装）
const edited = reactive<Record<string, Partial<ConfigRow>>>({});

const editKeyOf = (r: ConfigRow) => `${r.group}/${r.key}`;
const rowValue = (r: ConfigRow) => edited[editKeyOf(r)]?.value ?? r.value;
const rowDesc = (r: ConfigRow) => edited[editKeyOf(r)]?.description ?? r.description ?? "";

const load = async () => {
  loading.value = true;
  try {
    const resp: any = await configApi.list();
    rows.value = (resp.data ?? []) as ConfigRow[];
    for (const k of Object.keys(edited)) delete edited[k];
  } finally {
    loading.value = false;
  }
};
onMounted(load);

const onEditValue = (r: ConfigRow, v: string) => {
  edited[editKeyOf(r)] = { ...edited[editKeyOf(r)], value: v };
};
const onEditDesc = (r: ConfigRow, v: string) => {
  edited[editKeyOf(r)] = { ...edited[editKeyOf(r)], description: v };
};

const onRemoveRow = async (r: ConfigRow) => {
  await ElMessageBox.confirm(`确认删除键「${r.key}」？删除即生效（变更落历史可回滚；系统保护键会被拒绝）。`, "删除确认", {
    type: "warning"
  });
  await configApi.del(r.group, r.key);
  ElMessage.success("已删除");
  await load();
};

// —— 新增键值（弹窗录入 → 并入列表 → 随「提交变更」BatchUpsert 生效） ——
const createVisible = ref(false);
const createForm = reactive({ group: "", key: "", value: "", valueType: 1, description: "" });
const onCreate = () => {
  Object.assign(createForm, { group: "", key: "", value: "", valueType: 1, description: "" });
  createVisible.value = true;
};
const onSubmitCreate = () => {
  const group = createForm.group.trim();
  const key = createForm.key.trim();
  if (!group || !key) {
    ElMessage.warning("请填写分组与键名");
    return;
  }
  // 键字符集白名单（与后端 domain keyPattern 同规则——：'/' 等进
  // DELETE 路径参数会断路由，创建成功后无法移除）
  if (!/^[A-Za-z0-9._:-]{1,100}$/.test(key)) {
    ElMessage.warning("键名仅允许字母、数字与 . : _ -（1-100 字符），禁用 / ? # 等 URL 语义字符");
    return;
  }
  if (rows.value.some(r => r.group === group && r.key === key)) {
    ElMessage.warning(`键「${key}」已存在于分组「${group}」——请直接在列表中修改其值`);
    return;
  }
  rows.value = [
    ...rows.value,
    {
      group,
      key,
      value: createForm.value,
      valueType: createForm.valueType,
      description: createForm.description,
      isPublic: false,
      updatedAt: ""
    }
  ];
  edited[`${group}/${key}`] = {
    value: createForm.value,
    description: createForm.description,
    ...(rows.value[rows.value.length - 1] as any)
  };
  createVisible.value = false;
  ElMessage.success("已加入变更集——点「提交变更」生效");
};

const onSave = async () => {
  await ElMessageBox.confirm("提交将整体覆盖配置键集（变更落历史可回滚）", "提交确认", { type: "warning" });
  const items = rows.value.map(r => {
    const e = edited[editKeyOf(r)] ?? {};
    return {
      group: r.group,
      key: r.key,
      value: e.value ?? r.value,
      valueType: r.valueType ?? 1,
      description: e.description ?? r.description ?? "",
      isPublic: r.isPublic ?? false
    };
  });
  await configApi.update({ items, reason: "管理端全量提交" });
  ElMessage.success("已提交（watcher 热更即时生效）");
  await load();
};

const history = ref<{ version: number; createdAt: string }[]>([]);
const historyVisible = ref(false);
const onShowHistory = async () => {
  const resp: any = await configApi.history();
  history.value = resp.data ?? [];
  historyVisible.value = true;
};

const onRollback = async (version: number) => {
  await ElMessageBox.confirm(`回滚到 v${version} 之前的状态？`, "回滚确认", { type: "warning" });
  await configApi.rollback(version);
  ElMessage.success("已回滚（watcher 即时 reload）");
  historyVisible.value = false;
  await load();
};
</script>

<template>
  <div class="table-box">
    <el-card shadow="never" class="content-card">
      <div class="page-header">
        <span class="page-title">系统配置</span>
        <div class="page-actions">
          <el-button @click="onShowHistory">变更历史</el-button>
          <el-button v-auth="'admin.configs.update'" @click="onCreate">新增键值</el-button>
          <el-button v-auth="'admin.configs.update'" type="primary" @click="onSave">提交变更</el-button>
        </div>
      </div>
      <el-table v-loading="loading" :data="rows" border>
        <el-table-column label="分组" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">{{ row.group || "—" }}</template>
        </el-table-column>
        <el-table-column prop="key" label="键" min-width="180" show-overflow-tooltip />
        <el-table-column label="值" min-width="260">
          <template #default="{ row }">
            <el-input
              :model-value="rowValue(row)"
              type="textarea"
              :rows="1"
              autosize
              @update:model-value="(v: string) => onEditValue(row, v)"
            />
          </template>
        </el-table-column>
        <el-table-column label="类型" width="90" align="center">
          <template #default="{ row }">
            <el-tag size="small">{{ (VALUE_TYPES.find(t => t.value === row.valueType) ?? VALUE_TYPES[0]).label }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="描述" min-width="150">
          <template #default="{ row }">
            <el-input :model-value="rowDesc(row)" placeholder="—" @update:model-value="(v: string) => onEditDesc(row, v)" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button v-auth="'admin.configs.update'" size="small" type="danger" plain @click="onRemoveRow(row)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="createVisible" title="新增键值" width="480" destroy-on-close>
      <el-form label-width="80">
        <el-form-item label="分组" required>
          <el-input v-model="createForm.group" placeholder="业务分组（如 message/taskcenter）" maxlength="100" />
        </el-form-item>
        <el-form-item label="键名" required>
          <el-input v-model="createForm.key" placeholder="配置键（分区内唯一）" maxlength="100" />
        </el-form-item>
        <el-form-item label="类型" required>
          <el-select v-model="createForm.valueType" style="width: 160px">
            <el-option v-for="t in VALUE_TYPES" :key="t.value" :value="t.value" :label="t.label" />
          </el-select>
        </el-form-item>
        <el-form-item label="值" required>
          <el-input v-model="createForm.value" type="textarea" :rows="3" placeholder="配置值" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="createForm.description" placeholder="用途说明" maxlength="255" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" @click="onSubmitCreate">确定</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="historyVisible" title="变更历史（版本回滚）" size="460">
      <el-table :data="history" border>
        <el-table-column prop="version" label="版本" width="80" />
        <el-table-column prop="createdAt" label="时间" min-width="150" />
        <el-table-column label="操作" width="90">
          <template #default="{ row }">
            <el-button size="small" @click="onRollback(row.version)">回滚</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-drawer>
  </div>
</template>
