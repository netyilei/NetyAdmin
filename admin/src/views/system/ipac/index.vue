<script setup lang="ts" name="ipac">
// IP 访问控制（网关管理一级下，配置组模型）：策略组+组内规则
// 字典管理+全节点重载。应用侧组集绑定在「应用平台」页；判定语义=组集严格合成
// 「禁止 > 开放」（deny 跨组恒优先；组集非空无命中=拒 fail-closed；组集空=不限）。
// 变更由后端自动 Reload+Bus 广播全节点，「重载全节点」为手动强制同步面。
import { ElMessage, ElMessageBox } from "element-plus";
import type { FormInstance, FormRules } from "element-plus";
import { onMounted, reactive, ref } from "vue";

import { ipacApi, type IPGroup, type IPRule } from "@/api/modules/gwAdmin";

const PERM_IP_GROUPS_WRITE = "admin.gw.ip-groups.write";
const PERM_IP_RULES_WRITE = "admin.gw.ip-rules.write";
const PERM_IP_RELOAD = "admin.gw.ip-reload";

const ipLoading = ref(false);
const ipGroups = ref<IPGroup[]>([]);
const ipSelected = ref<IPGroup | null>(null);
const ipRules = ref<IPRule[]>([]);
const rulesLoading = ref(false);

const loadIpRules = async () => {
  const groupId = ipSelected.value?.id;
  if (!groupId) {
    ipRules.value = [];
    return;
  }
  rulesLoading.value = true;
  try {
    const resp: any = await ipacApi.rules(groupId);
    ipRules.value = (resp.data ?? []) as IPRule[];
  } finally {
    rulesLoading.value = false;
  }
};
const loadIpGroups = async () => {
  ipLoading.value = true;
  try {
    const resp: any = await ipacApi.groups();
    ipGroups.value = (resp.data ?? []) as IPGroup[];
    // 删除组后保持选中一致性：选中组已不存在则清空选择与规则列表
    if (ipSelected.value) {
      ipSelected.value = ipGroups.value.find(g => g.id === ipSelected.value!.id) ?? null;
      if (!ipSelected.value) ipRules.value = [];
    }
  } finally {
    ipLoading.value = false;
  }
};
const onSelectGroup = (row: IPGroup) => {
  ipSelected.value = row;
  loadIpRules();
};
const groupRowClass = ({ row }: { row: IPGroup }) =>
  ipSelected.value && row.id === ipSelected.value.id ? "ip-group-selected" : "";
onMounted(loadIpGroups);

const onReloadNodes = async () => {
  await ElMessageBox.confirm("确认向全部网关节点广播重载 IP 访问控制配置？", "重载确认", { type: "warning" });
  await ipacApi.reload();
  ElMessage.success("重载指令已广播全节点");
};

// 组新建/编辑
const groupVisible = ref(false);
const groupMode = ref<"create" | "update">("create");
const groupRow = ref<IPGroup | null>(null);
const groupForm = reactive({ name: "", remark: "" });
const openGroupCreate = () => {
  groupMode.value = "create";
  groupRow.value = null;
  Object.assign(groupForm, { name: "", remark: "" });
  groupVisible.value = true;
};
const openGroupEdit = (row: IPGroup) => {
  groupMode.value = "update";
  groupRow.value = row;
  Object.assign(groupForm, { name: row.name, remark: row.remark });
  groupVisible.value = true;
};
const onSubmitGroup = async () => {
  if (groupMode.value === "create") {
    await ipacApi.createGroup({ name: groupForm.name, remark: groupForm.remark || undefined });
    ElMessage.success("策略组已创建");
  } else if (groupRow.value) {
    await ipacApi.updateGroup(groupRow.value.id, { name: groupForm.name, remark: groupForm.remark || undefined });
    ElMessage.success("策略组已更新");
  }
  groupVisible.value = false;
  await loadIpGroups();
};
const onDeleteGroup = async (row: IPGroup) => {
  await ElMessageBox.confirm(
    `确认删除策略组「${row.name}」？组内规则将一并删除；若仍有应用绑定该组将被拒绝（需先在「应用平台」解绑）。`,
    "删除确认",
    { type: "warning" }
  );
  try {
    await ipacApi.deleteGroup(row.id);
  } catch (e: any) {
    // 占用守卫（后端 409 ip group in use by apps）——拦截器 reject 信封
    // {code, message_key, details:{reason}}；命中即本地引导，不叠加全局 toast
    const reason = String(e?.details?.reason ?? "");
    if (reason.includes("in use")) {
      ElMessage.warning("该组仍被应用绑定，请先在「应用平台」页解绑后再删除");
      return;
    }
    if (reason) return; // 其余业务错已由全局拦截器 toast——吞掉避免未处理 rejection
    throw e;
  }
  ElMessage.success("已删除");
  await loadIpGroups();
};

// 规则新建/删除（规则创建后不可变更——变更=删除后重建，后端无 PUT 端点）
const ruleVisible = ref(false);
const ruleFormRef = ref<FormInstance>();
const ruleForm = reactive({ cidrs: "", action: 1, priority: 0, enabled: true });
// CIDR 格式前置提示（③/G2 + P2-5）：IPv4/IPv6 CIDR 带前缀长度，逗号分隔；
// 仅本地正则提示，权威校验仍交后端 netip.ParsePrefix（fail-closed——IPv6 形状
// 按 RFC 3986 IPv6address 全集：含 : 压缩、内嵌 IPv4 与 /0-8 前缀）
const OCTET = "(?:25[0-5]|2[0-4]\\d|1\\d\\d|[1-9]?\\d)";
const H16 = "[0-9a-fA-F]{1,4}";
const LS32 = `(?:${H16}:${H16}|${OCTET}(?:\\.${OCTET}){3})`;
const IPV6_ADDR_SRC =
  `(?:` +
  `(?:${H16}:){6}${LS32}` + // 1:2:3:4:5:6:7:8 / ...:1.2.3.4
  `|::(?:${H16}:){5}${LS32}` + // ::2:3:4:5:6:7:8
  `|(?:${H16})?::(?:${H16}:){4}${LS32}` + // 1::3:4:5:6:7:8
  `|(?:(?:${H16}:){0,1}${H16})?::(?:${H16}:){3}${LS32}` + // 1:2::4:5:6:7:8
  `|(?:(?:${H16}:){0,2}${H16})?::(?:${H16}:){2}${LS32}` + // 1:2:3::5:6:7:8
  `|(?:(?:${H16}:){0,3}${H16})?::${H16}:${LS32}` + // 1:2:3:4::6:7:8
  `|(?:(?:${H16}:){0,4}${H16})?::${LS32}` + // 1:2:3:4:5::7:8 / 1:2:3:4:5::1.2.3.4
  `|(?:(?:${H16}:){0,5}${H16})?::${H16}` + // 1:2:3:4:5:6::8
  `|(?:(?:${H16}:){0,6}${H16})?::` + // 1:2:3:4:5:6:7:: / ::
  `)`;
const IPV4_CIDR_RE = new RegExp(`^(?:${OCTET}\\.){3}${OCTET}/(?:[0-2]?\\d|3[0-2])$`);
const IPV6_CIDR_RE = new RegExp(`^${IPV6_ADDR_SRC}/(?:12[0-8]|1[01]\\d|[1-9]?\\d)$`);
const isCIDR = (s: string) => IPV4_CIDR_RE.test(s) || IPV6_CIDR_RE.test(s);
const validateCidrs = (_rule: unknown, value: string, callback: (err?: Error) => void) => {
  if (!value?.trim()) return callback(new Error("请输入 CIDR 列表"));
  const parts = value
    .split(/[,，]/)
    .map(s => s.trim())
    .filter(Boolean);
  const bad = parts.find(p => !isCIDR(p));
  if (bad) return callback(new Error(`格式错误（每条须为 IPv4/IPv6 CIDR 带前缀长度，如 10.0.0.0/8 或 2001:db8::/32）：${bad}`));
  callback();
};
const ruleRules = reactive<FormRules>({
  cidrs: [{ required: true, validator: validateCidrs, trigger: "blur" }]
});
const openRuleCreate = () => {
  Object.assign(ruleForm, { cidrs: "", action: 1, priority: 0, enabled: true });
  ruleVisible.value = true;
};
const onSubmitRule = async () => {
  if (!ipSelected.value) return;
  const ok = await ruleFormRef.value
    ?.validate()
    .then(() => true)
    .catch(() => false);
  if (!ok) return;
  await ipacApi.createRule({
    groupId: ipSelected.value.id,
    cidrs: ruleForm.cidrs,
    action: ruleForm.action,
    priority: ruleForm.priority,
    enabled: ruleForm.enabled
  });
  ElMessage.success("规则已创建");
  ruleVisible.value = false;
  await loadIpRules();
};
const onDeleteRule = async (row: IPRule) => {
  await ElMessageBox.confirm(`确认删除规则「${row.cidrs}」？`, "删除确认", { type: "warning" });
  await ipacApi.deleteRule(row.id);
  ElMessage.success("已删除");
  await loadIpRules();
};
</script>

<template>
  <div class="table-box">
    <el-card shadow="never" class="content-card">
      <div class="page-header">
        <span class="page-title">IP 访问控制</span>
        <div class="page-actions">
          <el-button v-auth="PERM_IP_RELOAD" type="warning" plain @click="onReloadNodes">重载全节点</el-button>
          <el-button v-auth="PERM_IP_GROUPS_WRITE" type="primary" @click="openGroupCreate">新建策略组</el-button>
        </div>
      </div>

      <div class="semantic-tip">
        判定语义：应用启用组集内任一<strong>黑名单</strong>命中 → 拒（跨组恒优先）；
        <strong>白名单</strong>命中且无黑名单命中 → 放；组集非空无命中 → 拒；组集空 → 不限。 应用侧组集绑定在「应用平台」页。
      </div>

      <el-row :gutter="12">
        <!-- 左：策略组列表（点击行查看组内规则） -->
        <el-col :span="8">
          <div class="ip-pane-title">策略组（点击行查看组内规则）</div>
          <el-table
            v-loading="ipLoading"
            :data="ipGroups"
            border
            stripe
            highlight-current-row
            :row-class-name="groupRowClass"
            @row-click="onSelectGroup"
          >
            <el-table-column prop="name" label="名称" min-width="110" show-overflow-tooltip />
            <el-table-column label="备注" min-width="120" show-overflow-tooltip>
              <template #default="{ row }">{{ row.remark || "—" }}</template>
            </el-table-column>
            <el-table-column label="操作" width="130" fixed="right">
              <template #default="{ row }">
                <el-button v-auth="PERM_IP_GROUPS_WRITE" size="small" @click.stop="openGroupEdit(row)">编辑</el-button>
                <el-button v-auth="PERM_IP_GROUPS_WRITE" size="small" type="danger" plain @click.stop="onDeleteGroup(row)">
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-col>

        <!-- 右：选中组的规则列表（规则创建后不可变更——变更=删除后重建） -->
        <el-col :span="16">
          <div class="ip-pane-title">
            组内规则{{ ipSelected ? `：${ipSelected.name}` : "" }}
            <el-button
              v-if="ipSelected"
              v-auth="PERM_IP_RULES_WRITE"
              size="small"
              type="primary"
              class="ip-rule-add"
              @click="openRuleCreate"
            >
              新建规则
            </el-button>
          </div>
          <div v-if="ipSelected" class="form-tip ip-pane-tip">
            规则创建后不可修改（变更请删除后重建）；保存后后端自动重载并广播，「重载全节点」用于手动强制同步。
          </div>
          <el-table v-if="ipSelected" v-loading="rulesLoading" :data="ipRules" border stripe>
            <el-table-column prop="cidrs" label="CIDR 列表" min-width="200" show-overflow-tooltip />
            <el-table-column label="动作" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="row.action === 1 ? 'success' : 'danger'">{{ row.action === 1 ? "白名单" : "黑名单" }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="priority" label="优先级" width="90" align="center" />
            <el-table-column label="启用" width="80" align="center">
              <template #default="{ row }">
                <el-switch :model-value="row.enabled" disabled />
              </template>
            </el-table-column>
            <el-table-column label="备注" min-width="140" show-overflow-tooltip>
              <template #default="{ row }">{{ row.remark || "—" }}</template>
            </el-table-column>
            <el-table-column label="操作" width="90" fixed="right">
              <template #default="{ row }">
                <el-button v-auth="PERM_IP_RULES_WRITE" size="small" type="danger" plain @click="onDeleteRule(row)"
                  >删除</el-button
                >
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else description="请选择左侧策略组" :image-size="80" />
        </el-col>
      </el-row>
    </el-card>

    <el-dialog v-model="groupVisible" :title="groupMode === 'create' ? '新建策略组' : '编辑策略组'" width="480" destroy-on-close>
      <el-form label-width="80">
        <el-form-item label="名称" required>
          <el-input v-model="groupForm.name" placeholder="策略组名称（≤100 字，唯一）" maxlength="100" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="groupForm.remark" type="textarea" :rows="2" maxlength="255" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupVisible = false">取消</el-button>
        <el-button type="primary" @click="onSubmitGroup">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="ruleVisible" :title="`新建规则：${ipSelected?.name ?? ''}`" width="520" destroy-on-close>
      <el-form ref="ruleFormRef" :model="ruleForm" :rules="ruleRules" label-width="80">
        <el-form-item label="CIDR" prop="cidrs" required>
          <el-input
            v-model="ruleForm.cidrs"
            type="textarea"
            :rows="3"
            placeholder="逗号分隔，每条须带前缀长度：10.0.0.0/8,192.168.1.0/24"
          />
        </el-form-item>
        <el-form-item label="动作" required>
          <el-radio-group v-model="ruleForm.action">
            <el-radio :value="1">白名单（allow）</el-radio>
            <el-radio :value="2">黑名单（deny）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number v-model="ruleForm.priority" :min="0" />
          <span class="form-tip" style="margin-left: 8px">数值越小优先级越高</span>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="ruleForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ruleVisible = false">取消</el-button>
        <el-button type="primary" @click="onSubmitRule">确定</el-button>
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
.ip-pane-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.ip-pane-tip {
  margin-bottom: 8px;
}
/* 选中组行高亮（重载列表后保持——highlight-current-row 会被重建打断） */
:deep(.ip-group-selected td) {
  background-color: var(--el-color-primary-light-9) !important;
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
</style>
