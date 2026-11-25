<script setup lang="ts" name="smtpSettings">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { smtpApi, type SmtpConfig } from "@/api/modules/smtp";

const loading = ref(false);
const saving = ref(false);
const testing = ref(false);
const raw = ref<SmtpConfig>();
const testTo = ref("");

const form = reactive({
  host: "",
  port: 587,
  username: "",
  from: ""
});

const load = async () => {
  loading.value = true;
  try {
    const { data } = await smtpApi.get();
    raw.value = data;
    form.host = data.host;
    form.port = data.port;
    form.username = data.username;
    form.from = data.from;
    if (!testTo.value && data.from) testTo.value = data.from;
  } finally {
    loading.value = false;
  }
};

const save = async () => {
  saving.value = true;
  try {
    const { data } = await smtpApi.save({ ...form });
    raw.value = data;
    ElMessage.success("SMTP 配置已保存（各节点热更生效，无需重启）");
  } finally {
    saving.value = false;
  }
};

const reset = async () => {
  await ElMessageBox.confirm("将删除配置中心覆盖值，回退为启动配置（config.yaml）。确认重置？", "重置确认", { type: "warning" });
  const { data } = await smtpApi.reset();
  raw.value = data;
  form.host = data.host;
  form.port = data.port;
  form.username = data.username;
  form.from = data.from;
  ElMessage.success("已回退为启动配置");
};

const sendTest = async () => {
  testing.value = true;
  try {
    const { data } = await smtpApi.test(testTo.value);
    ElMessage.success(`测试邮件已提交发送：${data.to}（投递失败见消息管理发送记录/错误日志）`);
  } finally {
    testing.value = false;
  }
};

onMounted(load);
</script>

<template>
  <div class="smtp-page">
    <el-card v-loading="loading" shadow="never">
      <template #header>
        <div class="card-header">
          <span>SMTP 邮件服务</span>
          <div class="header-tags">
            <el-tag v-if="raw" :type="raw.source === 'configcenter' ? 'success' : 'info'" size="small">
              生效来源：{{ raw.source === "configcenter" ? "配置中心（覆盖）" : "启动配置（config.yaml）" }}
            </el-tag>
            <el-tag v-if="raw?.active_driver" type="info" size="small">驱动：{{ raw.active_driver }}</el-tag>
          </div>
        </div>
      </template>

      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="保存后经配置中心广播热更生效（多节点一致），无需重启。密码始终由环境变量 NETYADMIN_MESSAGE_SMTP_PASSWORD 提供，不入库、不回显。"
        class="tip"
      />

      <el-form label-width="110px" class="form">
        <el-form-item label="SMTP 服务器" required>
          <el-input v-model="form.host" placeholder="如 smtp.example.com" />
        </el-form-item>
        <el-form-item label="端口" required>
          <el-input-number v-model="form.port" :min="1" :max="65535" controls-position="right" />
          <span class="hint">587=STARTTLS（有凭据强制加密）/ 465=隐式 TLS（SMTPS）/ 25=内网 relay（无凭据跳过认证）</span>
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.username" placeholder="留空=内网无认证 relay" />
        </el-form-item>
        <el-form-item label="发件人地址" required>
          <el-input v-model="form.from" placeholder="如 noreply@example.com" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            :model-value="raw?.password_set ? '已由环境变量注入' : '未配置（env NETYADMIN_MESSAGE_SMTP_PASSWORD）'"
            disabled
          />
        </el-form-item>
        <el-form-item>
          <el-button v-auth="'admin.settings.smtp.operate'" type="primary" :loading="saving" @click="save"> 保存 </el-button>
          <el-button v-auth="'admin.settings.smtp.operate'" :disabled="raw?.source !== 'configcenter'" @click="reset">
            重置为启动配置
          </el-button>
          <el-button :loading="loading" @click="load">刷新</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span>测试邮件</span></template>
      <div class="test-row">
        <el-input v-model="testTo" placeholder="收件人邮箱" style="width: 320px" />
        <el-button v-auth="'admin.settings.smtp.operate'" type="primary" plain :loading="testing" @click="sendTest">
          发送测试邮件
        </el-button>
        <span class="hint">使用当前生效配置直连 SMTP（不入发送记录台账）</span>
      </div>
    </el-card>
  </div>
</template>

<style scoped lang="scss">
.smtp-page {
  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;

    .header-tags {
      display: flex;
      gap: 8px;
    }
  }

  .tip {
    margin-bottom: 16px;
  }

  .form {
    max-width: 720px;
  }

  .hint {
    margin-left: 8px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .section-card {
    margin-top: 12px;
  }

  .test-row {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
  }
}
</style>
