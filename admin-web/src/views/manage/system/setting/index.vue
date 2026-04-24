<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, reactive, ref } from 'vue';
import { NCard, NTabPane, NTabs } from 'naive-ui';
import { fetchGetSysConfigs, fetchTestEmail, fetchUpdateSysConfig } from '@/service/api/v1/system-manage';
import { fetchGetAllEnabledStorageConfigs } from '@/service/api/v1/storage';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

const CacheTab = defineAsyncComponent(() => import('./components/cache-tab.vue'));
const CaptchaTab = defineAsyncComponent(() => import('./components/captcha-tab.vue'));
const UserTab = defineAsyncComponent(() => import('./components/user-tab.vue'));
const TaskTab = defineAsyncComponent(() => import('./components/task-tab.vue'));
const LogTab = defineAsyncComponent(() => import('./components/log-tab.vue'));
const EmailTab = defineAsyncComponent(() => import('./components/email-tab.vue'));
const ContentCacheTab = defineAsyncComponent(() => import('./components/content-cache-tab.vue'));
const SmsTab = defineAsyncComponent(() => import('./components/sms-tab.vue'));

const loading = ref(false);
const updating = ref<string>('');

const cacheConfigs = ref<SystemManage.SysConfig[]>([]);

interface ConfigItem extends SystemManage.SysConfig {
  numValue?: number;
}

const systemCaches = computed(() => cacheConfigs.value.filter(c => c.isSystem));
const moduleCaches = computed(() => cacheConfigs.value.filter(c => !c.isSystem));

const logConfigs = reactive<Record<string, ConfigItem | undefined>>({
  task_log_enabled: undefined,
  task_retention: undefined,
  ops_retention: undefined,
  err_retention: undefined,
  msg_record_retention: undefined,
  open_log_retention: undefined
});

const logbusConfigs = reactive<Record<string, ConfigItem | undefined>>({
  global_max_entries: undefined,
  global_max_bytes_mb: undefined,
  default_batch_size: undefined,
  default_time_threshold: undefined,
  operation_batch_size: undefined,
  operation_time_threshold: undefined,
  error_batch_size: undefined,
  error_time_threshold: undefined,
  open_batch_size: undefined,
  open_time_threshold: undefined,
  task_batch_size: undefined,
  task_time_threshold: undefined,
  force_sync: undefined
});

const emailConfigs = reactive<Record<string, ConfigItem | undefined>>({
  enabled: undefined,
  host: undefined,
  port: undefined,
  user: undefined,
  password: undefined,
  from: undefined,
  ssl_enabled: undefined,
  starttls_enabled: undefined,
  auth_type: undefined,
  connect_timeout: undefined,
  send_timeout: undefined
});

const emailAuthTypeOptions = [
  { label: $t('page.manage.setting.email.authTypePlain'), value: 'plain' },
  { label: $t('page.manage.setting.email.authTypeLogin'), value: 'login' },
  { label: $t('page.manage.setting.email.authTypeCrammd5'), value: 'crammd5' },
  { label: $t('page.manage.setting.email.authTypeAuto'), value: 'auto' }
];

const testEmailLoading = ref(false);
const testEmailReceiver = ref('');

const smsConfigs = reactive<Record<string, ConfigItem | undefined>>({
  enabled: undefined,
  driver: undefined,
  secret_id: undefined,
  secret_key: undefined,
  app_id: undefined,
  sign_name: undefined
});

const contentCacheConfigs = reactive<Record<string, ConfigItem | undefined>>({
  banner_cache_ttl: undefined,
  category_cache_ttl: undefined,
  article_cache_ttl: undefined
});

const captchaConfigs = reactive<{
  switches: SystemManage.SysConfig[];
  params: Record<string, ConfigItem | undefined>;
}>({
  switches: [],
  params: {
    captcha_length: undefined,
    captcha_width: undefined,
    captcha_height: undefined,
    captcha_expire: undefined,
    captcha_type: undefined
  }
});

const captchaTypeOptions = [
  { label: $t('page.manage.setting.captcha.typeDigit'), value: 'digit' },
  { label: $t('page.manage.setting.captcha.typeString'), value: 'string' },
  { label: $t('page.manage.setting.captcha.typeMath'), value: 'math' }
];

const userConfigs = reactive<Record<string, ConfigItem | undefined>>({
  storage_module: undefined,
  login_storage: undefined,
  token_expire: undefined,
  login_max_retry: undefined,
  login_lock_duration: undefined,
  password_min_length: undefined,
  password_require_types: undefined,
  user_register_verify: undefined,
  user_register_verify_type: undefined,
  user_login_verify: undefined,
  user_login_verify_type: undefined,
  user_reset_pwd_verify: undefined,
  user_reset_pwd_verify_type: undefined
});

const storageOptions = ref<{ label: string; value: string }[]>([]);

const loginStorageOptions = [
  { label: $t('page.manage.setting.user.login_storage_cache'), value: 'cache' },
  { label: $t('page.manage.setting.user.login_storage_db'), value: 'db' }
];

const verifyTypeOptions = [
  { label: $t('page.manage.setting.user.verify_type_email'), value: 'email' },
  { label: $t('page.manage.setting.user.verify_type_sms'), value: 'sms' }
];

async function init() {
  loading.value = true;

  const results = await Promise.all([
    fetchGetSysConfigs('cache_switches'),
    fetchGetSysConfigs('task_config'),
    fetchGetSysConfigs('ops_config'),
    fetchGetSysConfigs('error_config'),
    fetchGetSysConfigs('captcha_config'),
    fetchGetSysConfigs('user_config'),
    fetchGetAllEnabledStorageConfigs(),
    fetchGetSysConfigs('email_config'),
    fetchGetSysConfigs('sms_config'),
    fetchGetSysConfigs('msg_record_config'),
    fetchGetSysConfigs('open_platform_config'),
    fetchGetSysConfigs('logbus_config'),
    fetchGetSysConfigs('content_cache')
  ]);

  if (!results[0].error) cacheConfigs.value = results[0].data;

  if (!results[1].error) {
    results[1].data.forEach(item => {
      if (item.configKey === 'log_enabled') logConfigs.task_log_enabled = item;
      if (item.configKey === 'retention_days') {
        logConfigs.task_retention = { ...item, numValue: Number.parseInt(item.configValue, 10) || 0 };
      }
    });
  }

  if (!results[2].error) {
    results[2].data.forEach(item => {
      if (item.configKey === 'retention_days') {
        logConfigs.ops_retention = { ...item, numValue: Number.parseInt(item.configValue, 10) || 0 };
      }
    });
  }

  if (!results[3].error) {
    results[3].data.forEach(item => {
      if (item.configKey === 'retention_days') {
        logConfigs.err_retention = { ...item, numValue: Number.parseInt(item.configValue, 10) || 0 };
      }
    });
  }

  if (!results[4].error) {
    results[4].data.forEach(item => {
      if (item.configKey.endsWith('_enabled') || item.configKey === 'user_login_enabled') {
        captchaConfigs.switches.push(item);
      } else {
        captchaConfigs.params[item.configKey] = {
          ...item,
          numValue: item.configKey !== 'captcha_type' ? Number.parseInt(item.configValue, 10) || 0 : undefined
        };
      }
    });
  }

  if (!results[5].error) {
    results[5].data.forEach(item => {
      if (Object.keys(userConfigs).includes(item.configKey)) {
        userConfigs[item.configKey] = {
          ...item,
          numValue: !['_verify_type', 'storage_module', 'login_storage', '_verify'].some(k =>
            item.configKey.endsWith(k)
          )
            ? Number.parseInt(item.configValue, 10) || 0
            : undefined
        };
      }
    });
  }

  if (!results[6].error) {
    storageOptions.value = results[6].data.map(item => ({
      label: item.name,
      value: item.id.toString()
    }));
  }

  if (!results[7].error) {
    results[7].data.forEach(item => {
      if (Object.keys(emailConfigs).includes(item.configKey)) {
        emailConfigs[item.configKey] = {
          ...item,
          numValue: ['port', 'connect_timeout', 'send_timeout'].includes(item.configKey)
            ? Number.parseInt(item.configValue, 10) || 0
            : undefined
        };
      }
    });
  }

  if (!results[8].error) {
    results[8].data.forEach(item => {
      if (Object.keys(smsConfigs).includes(item.configKey)) {
        smsConfigs[item.configKey] = item;
      }
    });
  }

  if (!results[9].error) {
    results[9].data.forEach(item => {
      if (item.configKey === 'retention_days') {
        logConfigs.msg_record_retention = { ...item, numValue: Number.parseInt(item.configValue, 10) || 0 };
      }
    });
  }

  if (!results[10].error) {
    results[10].data.forEach(item => {
      if (item.configKey === 'log_retention_days') {
        logConfigs.open_log_retention = { ...item, numValue: Number.parseInt(item.configValue, 10) || 0 };
      }
    });
  }

  if (!results[11].error) {
    results[11].data.forEach(item => {
      if (Object.keys(logbusConfigs).includes(item.configKey)) {
        logbusConfigs[item.configKey] = { ...item, numValue: Number.parseInt(item.configValue, 10) || 0 };
      }
    });
  }

  if (!results[12].error) {
    results[12].data.forEach(item => {
      if (Object.keys(contentCacheConfigs).includes(item.configKey)) {
        contentCacheConfigs[item.configKey] = {
          ...item,
          numValue: Number.parseInt(item.configValue, 10) || 0
        };
      }
    });
  }

  loading.value = false;
}

async function handleUpdate(item?: SystemManage.SysConfig) {
  if (!item || !item.configKey) return;
  updating.value = item.configKey;
  const { error } = await fetchUpdateSysConfig(item);
  updating.value = '';
  if (!error) {
    window.$message?.success($t('common.updateSuccess'));
  }
}

async function handleNumberUpdate(item?: ConfigItem) {
  if (!item || item.numValue === undefined || item.numValue === Number.parseInt(item.configValue, 10)) return;
  item.configValue = item.numValue.toString();
  await handleUpdate(item);
}

async function handleTestEmail() {
  if (!testEmailReceiver.value) {
    window.$message?.warning($t('page.manage.setting.email.testReceiverRequired'));
    return;
  }
  testEmailLoading.value = true;
  const { error } = await fetchTestEmail({ receiver: testEmailReceiver.value });
  testEmailLoading.value = false;
  if (!error) {
    window.$message?.success($t('page.manage.setting.email.testSuccess'));
  }
}

onMounted(init);
</script>

<template>
  <div class="h-full">
    <NCard :bordered="false" class="h-full overflow-hidden rounded-8px shadow-sm">
      <NTabs type="line" animated class="h-full-setting">
        <NTabPane name="cache" :tab="$t('page.manage.setting.tabs.cache')">
          <CacheTab
            :loading="loading"
            :updating="updating"
            :system-caches="systemCaches"
            :module-caches="moduleCaches"
            @update="handleUpdate"
          />
        </NTabPane>

        <NTabPane name="captcha" :tab="$t('page.manage.setting.tabs.captcha')">
          <CaptchaTab
            :loading="loading"
            :updating="updating"
            :captcha-configs="captchaConfigs"
            :captcha-type-options="captchaTypeOptions"
            @update="handleUpdate"
            @number-update="handleNumberUpdate"
          />
        </NTabPane>

        <NTabPane name="user" :tab="$t('page.manage.setting.tabs.user')">
          <UserTab
            :loading="loading"
            :updating="updating"
            :user-configs="userConfigs"
            :storage-options="storageOptions"
            :login-storage-options="loginStorageOptions"
            :verify-type-options="verifyTypeOptions"
            @update="handleUpdate"
            @number-update="handleNumberUpdate"
          />
        </NTabPane>

        <NTabPane name="task" :tab="$t('page.manage.setting.tabs.task')">
          <TaskTab
            :updating="updating"
            :log-configs="logConfigs"
            @update="handleUpdate"
            @number-update="handleNumberUpdate"
          />
        </NTabPane>

        <NTabPane name="log" :tab="$t('page.manage.setting.tabs.log')">
          <LogTab
            :loading="loading"
            :updating="updating"
            :log-configs="logConfigs"
            :logbus-configs="logbusConfigs"
            @update="handleUpdate"
            @number-update="handleNumberUpdate"
          />
        </NTabPane>

        <NTabPane name="email" :tab="$t('page.manage.setting.tabs.email')">
          <EmailTab
            :loading="loading"
            :updating="updating"
            :email-configs="emailConfigs"
            :email-auth-type-options="emailAuthTypeOptions"
            :test-email-loading="testEmailLoading"
            :test-email-receiver="testEmailReceiver"
            @update="handleUpdate"
            @number-update="handleNumberUpdate"
            @test-email="handleTestEmail"
            @update:test-email-receiver="testEmailReceiver = $event"
          />
        </NTabPane>

        <NTabPane name="contentCache" :tab="$t('page.manage.setting.tabs.contentCache')">
          <ContentCacheTab
            :loading="loading"
            :content-cache-configs="contentCacheConfigs"
            @number-update="handleNumberUpdate"
          />
        </NTabPane>

        <NTabPane name="sms" :tab="$t('page.manage.setting.tabs.sms')">
          <SmsTab :loading="loading" :updating="updating" :sms-configs="smsConfigs" @update="handleUpdate" />
        </NTabPane>
      </NTabs>
    </NCard>
  </div>
</template>

<style scoped>
.h-full-setting {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.h-full-setting :deep(.n-tabs-pane-wrapper) {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.h-full-setting :deep(.n-tab-pane) {
  height: auto;
}

.overflow-hidden :deep(.n-card__content) {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
</style>
