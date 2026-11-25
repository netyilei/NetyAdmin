<script setup lang="ts" name="messageAdmin">
// 消息管理：模板 CRUD/发送 + 发送记录/重发 + 站内信发送。
// 端点=admin.message.*（contracts/openapi/admin.v1.yaml）；权限码与 rbac_apis 同源
// （12560564 种子：admin.message.templates.write / admin.message.send / admin.message.inbox.send）。
import { ElMessage, ElMessageBox } from "element-plus";
import { computed, onMounted, reactive, ref } from "vue";

import { messageApi, type MessageRecordRow, type MessageTemplateRow } from "@/api/modules/messageAdmin";
import { fmtTime } from "@/utils/format";

// 实体状态枚举（msgdomain：模板 1=enabled 2=disabled；记录 1=pending 2=sent 3=failed——具名禁字面量）
const TPL_ENABLED = 1;
const TPL_DISABLED = 2;
const REC_PENDING = 1;
const REC_SENT = 2;
const REC_FAILED = 3;

const activeTab = ref("templates");

// —— 模板 tab ——
const tplLoading = ref(false);
const tplRows = ref<MessageTemplateRow[]>([]);
const tplTotal = ref(0);
const tplQuery = reactive({ page: 1, page_size: 10, name: "" });

const loadTemplates = async () => {
  tplLoading.value = true;
  try {
    const resp: any = await messageApi.listTemplates(tplQuery.page, tplQuery.page_size, {
      name: tplQuery.name || undefined
    });
    tplRows.value = resp.data.list;
    tplTotal.value = resp.data.total;
  } finally {
    tplLoading.value = false;
  }
};
const onTplSearch = () => {
  tplQuery.page = 1;
  loadTemplates();
};

const channelTag = (channel: string): "success" | "warning" => (channel === "email" ? "success" : "warning");
const channelLabel = (channel: string): string => (channel === "email" ? "邮件" : "短信");

// —— 模板新建/编辑弹窗（templateRequest：status 必填枚举，update 缺省=0 静默禁用防呆——必带） ——
const editVisible = ref(false);
const editMode = ref<"create" | "update">("create");
const editRow = ref<MessageTemplateRow | null>(null);
const editForm = reactive({ name: "", channel: "email" as "email" | "sms", subject: "", body: "", status: TPL_ENABLED });

const openCreate = () => {
  editMode.value = "create";
  editRow.value = null;
  Object.assign(editForm, { name: "", channel: "email", subject: "", body: "", status: TPL_ENABLED });
  editVisible.value = true;
};
const openEdit = (row: MessageTemplateRow) => {
  editMode.value = "update";
  editRow.value = row;
  Object.assign(editForm, { name: row.name, channel: row.channel, subject: row.subject, body: row.body, status: row.status });
  editVisible.value = true;
};
const onSubmitEdit = async () => {
  const payload = {
    name: editForm.name,
    channel: editForm.channel,
    subject: editForm.subject,
    body: editForm.body,
    status: editForm.status
  };
  if (editMode.value === "create") {
    await messageApi.createTemplate(payload);
    ElMessage.success("模板已创建");
  } else if (editRow.value) {
    await messageApi.updateTemplate(editRow.value.id, payload);
    ElMessage.success("已更新");
  }
  editVisible.value = false;
  await loadTemplates();
};

const onDeleteTpl = async (row: MessageTemplateRow) => {
  await ElMessageBox.confirm(`确认删除模板「${row.name}」？删除后不可恢复。`, "删除确认", { type: "warning" });
  await messageApi.deleteTemplate(row.id);
  ElMessage.success("已删除");
  await loadTemplates();
};

// —— 发送弹窗（sendRequest：template_id/target 必填；vars 由 body {{key}} 占位符提取） ——
const sendVisible = ref(false);
const sendTemplate = ref<MessageTemplateRow | null>(null);
const sendForm = reactive({ target: "", vars: {} as Record<string, string> });

const varKeys = computed<string[]>(() => {
  const body = sendTemplate.value?.body ?? "";
  const keys = new Set<string>();
  for (const m of body.matchAll(/\{\{([a-zA-Z0-9_]+)\}\}/g)) keys.add(m[1]);
  return [...keys];
});

const onSend = (row: MessageTemplateRow) => {
  sendTemplate.value = row;
  sendForm.target = "";
  sendForm.vars = {};
  sendVisible.value = true;
};
const onSubmitSend = async () => {
  if (!sendTemplate.value) return;
  await messageApi.send({ template_id: sendTemplate.value.id, target: sendForm.target, vars: sendForm.vars });
  ElMessage.success("消息已提交发送");
  sendVisible.value = false;
};

// —— 发送记录 tab ——
const recLoading = ref(false);
const recRows = ref<MessageRecordRow[]>([]);
const recTotal = ref(0);
const recQuery = reactive({ page: 1, page_size: 10, channel: "", status: "" });
let recLoaded = false;

const loadRecords = async () => {
  recLoading.value = true;
  try {
    const resp: any = await messageApi.listRecords(recQuery.page, recQuery.page_size, {
      channel: recQuery.channel || undefined,
      status: recQuery.status || undefined
    });
    recRows.value = resp.data.list;
    recTotal.value = resp.data.total;
    recLoaded = true;
  } finally {
    recLoading.value = false;
  }
};
const onRecSearch = () => {
  recQuery.page = 1;
  loadRecords();
};

const recStatusTag = (status: number): { label: string; type: "info" | "success" | "danger" } => {
  if (status === REC_SENT) return { label: "已发送", type: "success" };
  if (status === REC_FAILED) return { label: "失败", type: "danger" };
  if (status === REC_PENDING) return { label: "待发送", type: "info" };
  return { label: `未知(${status})`, type: "info" };
};

const onResend = async (row: MessageRecordRow) => {
  await ElMessageBox.confirm(`确认重发「${row.target}」的${channelLabel(row.channel)}消息？`, "重发确认", { type: "warning" });
  await messageApi.resend(row.id);
  ElMessage.success("已重新提交发送");
  await loadRecords();
};

// —— 站内信 tab ——
const inboxForm = reactive({ user_id: "", title: "", body: "" });
const inboxSending = ref(false);
const onSendInbox = async () => {
  inboxSending.value = true;
  try {
    await messageApi.sendInbox({ user_id: inboxForm.user_id, title: inboxForm.title, body: inboxForm.body });
    ElMessage.success("站内信已发送");
    Object.assign(inboxForm, { user_id: "", title: "", body: "" });
  } finally {
    inboxSending.value = false;
  }
};

const onTabChange = (tab: string | number) => {
  if (tab === "records" && !recLoaded) loadRecords();
};

onMounted(loadTemplates);
</script>

<template>
  <div class="table-box">
    <el-card shadow="never" class="content-card">
      <el-tabs v-model="activeTab" @tab-change="onTabChange">
        <el-tab-pane label="消息模板" name="templates">
          <div class="page-header">
            <span class="page-title">消息模板</span>
            <div class="page-actions">
              <el-input
                v-model="tplQuery.name"
                placeholder="按名称搜索"
                clearable
                style="width: 200px"
                @keyup.enter="onTplSearch"
                @clear="onTplSearch"
              />
              <el-button @click="onTplSearch">搜索</el-button>
              <el-button v-auth="'admin.message.templates.write'" type="primary" @click="openCreate">新建模板</el-button>
            </div>
          </div>

          <el-table v-loading="tplLoading" :data="tplRows" border stripe>
            <el-table-column prop="name" label="名称" min-width="140" show-overflow-tooltip />
            <el-table-column label="通道" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="channelTag(row.channel)">{{ channelLabel(row.channel) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="subject" label="主题" min-width="150" show-overflow-tooltip>
              <template #default="{ row }">{{ row.subject || "—" }}</template>
            </el-table-column>
            <el-table-column prop="body" label="正文" min-width="220" show-overflow-tooltip />
            <el-table-column label="状态" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === TPL_ENABLED ? 'success' : 'danger'">{{
                  row.status === TPL_ENABLED ? "启用" : "停用"
                }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="更新时间" min-width="150">
              <template #default="{ row }">{{ fmtTime(row.updated_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="200" fixed="right">
              <template #default="{ row }">
                <el-button v-auth="'admin.message.send'" size="small" type="primary" plain @click="onSend(row)">发送</el-button>
                <el-button v-auth="'admin.message.templates.write'" size="small" @click="openEdit(row)">编辑</el-button>
                <el-button v-auth="'admin.message.templates.write'" size="small" type="danger" plain @click="onDeleteTpl(row)"
                  >删除</el-button
                >
              </template>
            </el-table-column>
          </el-table>
          <div class="pagination-wrap">
            <el-pagination
              layout="total, sizes, prev, pager, next"
              :total="tplTotal"
              :page-sizes="[10, 20, 50]"
              :page-size="tplQuery.page_size"
              :current-page="tplQuery.page"
              @current-change="
                (p: number) => {
                  tplQuery.page = p;
                  loadTemplates();
                }
              "
              @size-change="
                (s: number) => {
                  tplQuery.page_size = s;
                  tplQuery.page = 1;
                  loadTemplates();
                }
              "
            />
          </div>
        </el-tab-pane>

        <el-tab-pane label="发送记录" name="records">
          <div class="page-header">
            <span class="page-title">发送记录</span>
            <div class="page-actions">
              <el-select v-model="recQuery.channel" clearable placeholder="通道" style="width: 110px">
                <el-option label="邮件" value="email" />
                <el-option label="短信" value="sms" />
              </el-select>
              <el-select v-model="recQuery.status" clearable placeholder="状态" style="width: 110px">
                <el-option label="待发送" value="1" />
                <el-option label="已发送" value="2" />
                <el-option label="失败" value="3" />
              </el-select>
              <el-button @click="onRecSearch">搜索</el-button>
            </div>
          </div>

          <el-table v-loading="recLoading" :data="recRows" border stripe>
            <el-table-column label="通道" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="channelTag(row.channel)">{{ channelLabel(row.channel) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="target" label="接收目标" min-width="150" show-overflow-tooltip />
            <el-table-column prop="subject" label="主题" min-width="140" show-overflow-tooltip>
              <template #default="{ row }">{{ row.subject || "—" }}</template>
            </el-table-column>
            <el-table-column prop="driver" label="驱动" width="100" align="center">
              <template #default="{ row }">{{ row.driver || "—" }}</template>
            </el-table-column>
            <el-table-column label="状态" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="recStatusTag(row.status).type">{{ recStatusTag(row.status).label }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="retries" label="重试" width="70" align="center" />
            <el-table-column prop="error" label="错误" min-width="140" show-overflow-tooltip>
              <template #default="{ row }">{{ row.error || "—" }}</template>
            </el-table-column>
            <el-table-column label="创建时间" min-width="150">
              <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="发送时间" min-width="150">
              <template #default="{ row }">{{ fmtTime(row.sent_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="90" fixed="right">
              <template #default="{ row }">
                <el-button v-auth="'admin.message.send'" size="small" @click="onResend(row)">重发</el-button>
              </template>
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

        <el-tab-pane label="站内信" name="inbox">
          <div class="page-header">
            <span class="page-title">发送站内信</span>
          </div>
          <el-form label-width="90" style="max-width: 560px">
            <el-form-item label="用户 ID" required>
              <el-input v-model="inboxForm.user_id" placeholder="目标用户 ID（C 端 users）" />
            </el-form-item>
            <el-form-item label="标题" required>
              <el-input v-model="inboxForm.title" />
            </el-form-item>
            <el-form-item label="正文" required>
              <el-input v-model="inboxForm.body" type="textarea" :rows="6" />
            </el-form-item>
            <el-form-item>
              <el-button v-auth="'admin.message.inbox.send'" type="primary" :loading="inboxSending" @click="onSendInbox"
                >发送</el-button
              >
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <el-dialog
      v-model="editVisible"
      :title="editMode === 'create' ? '新建消息模板' : '编辑消息模板'"
      width="520"
      destroy-on-close
    >
      <el-form label-width="90">
        <el-form-item label="名称" required>
          <!-- 名称=模板业务唯一键（下游自举 code）：创建后不可改，改名请删除重建 -->
          <el-input
            v-model="editForm.name"
            :placeholder="editMode === 'update' ? '名称创建后不可修改' : '模板名称'"
            :disabled="editMode === 'update'"
          />
        </el-form-item>
        <el-form-item label="通道" required>
          <el-select v-model="editForm.channel" style="width: 100%">
            <el-option label="邮件" value="email" />
            <el-option label="短信" value="sms" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="editForm.channel === 'email'" label="主题">
          <el-input v-model="editForm.subject" placeholder="邮件主题；短信通道为空" />
        </el-form-item>
        <el-form-item label="正文" required>
          <el-input v-model="editForm.body" type="textarea" :rows="6" placeholder="支持 {{key}} 占位符" />
        </el-form-item>
        <el-form-item label="状态" required>
          <el-select v-model="editForm.status" style="width: 100%">
            <el-option label="启用" :value="TPL_ENABLED" />
            <el-option label="停用" :value="TPL_DISABLED" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="onSubmitEdit">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="sendVisible" :title="`发送消息：${sendTemplate?.name ?? ''}`" width="480" destroy-on-close>
      <el-form label-width="90">
        <el-form-item label="接收目标" required>
          <el-input v-model="sendForm.target" :placeholder="sendTemplate?.channel === 'email' ? '邮箱地址' : '手机号'" />
        </el-form-item>
        <el-form-item v-for="k in varKeys" :key="k" :label="k">
          <el-input v-model="sendForm.vars[k]" :placeholder="`变量 ${k}`" />
        </el-form-item>
        <el-form-item v-if="!varKeys.length">
          <span class="tip-text">模板无占位符，无需变量</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="sendVisible = false">取消</el-button>
        <el-button type="primary" @click="onSubmitSend">发送</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.tip-text {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>
