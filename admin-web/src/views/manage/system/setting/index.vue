<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import {
  NAlert,
  NButton,
  NCard,
  NDivider,
  NFormItem,
  NGrid,
  NGridItem,
  NInput,
  NInputNumber,
  NSelect,
  NSpace,
  NSpin,
  NSwitch,
  NTabPane,
  NTabs
} from 'naive-ui';
import { fetchGetSysConfigs, fetchTestEmail, fetchUpdateSysConfig } from '@/service/api/v1/system-manage';
import { fetchGetAllEnabledStorageConfigs } from '@/service/api/v1/storage';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

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
  msg_record_retention: undefined
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
  user_reset_pwd_verify: undefined,
  user_reset_pwd_verify_type: undefined
});

const storageOptions = ref<{ label: string; value: string }[]>([]);

const loginStorageOptions = [
  { label: $t('page.manage.setting.user.login_storage_memory'), value: 'memory' },
  { label: $t('page.manage.setting.user.login_storage_db'), value: 'db' }
];

const verifyTypeOptions = [
  { label: $t('page.manage.setting.user.verify_type_email'), value: 'email' },
  { label: $t('page.manage.setting.user.verify_type_sms'), value: 'sms' }
];

async function init() {
  loading.value = true;

  // 1. 加载所有相关配置
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
    fetchGetSysConfigs('msg_record_config')
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
      if (item.configKey.endsWith('_enabled') || item.configKey.startsWith('user_login_')) {
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
    <NCard :bordered="false" class="h-full rounded-8px shadow-sm">
      <NTabs type="line" animated>
        <!-- Tab 1: Cache Management -->
        <NTabPane name="cache" :tab="$t('page.manage.setting.tabs.cache')">
          <div class="pt-4">
            <NAlert type="info" class="mb-6">
              {{ $t('page.manage.setting.cache.description') }}
            </NAlert>
            <NSpin :show="loading">
              <div v-if="systemCaches.length > 0">
                <NDivider title-placement="left">
                  <span class="text-13px text-gray-500 font-bold tracking-wider uppercase">
                    {{ $t('page.manage.setting.cache.systemGroup') }}
                  </span>
                </NDivider>
                <NGrid :x-gap="16" :y-gap="16" cols="1 s:2 m:3" responsive="screen">
                  <NGridItem v-for="item in systemCaches" :key="item.configKey">
                    <NCard size="small" class="cursor-default transition-colors hover:border-primary">
                      <div class="flex-y-center justify-between">
                        <div>
                          <div class="mb-1 flex items-center text-16px font-bold">
                            {{ $t(`page.manage.setting.cache.${item.configKey}`) }}
                          </div>
                          <div class="text-12px text-gray-400">Key: {{ item.configKey }}</div>
                        </div>
                        <NSwitch
                          v-model:value="item.configValue"
                          checked-value="true"
                          unchecked-value="false"
                          :loading="updating === item.configKey"
                          @update:value="handleUpdate(item)"
                        />
                      </div>
                    </NCard>
                  </NGridItem>
                </NGrid>
              </div>

              <div v-if="moduleCaches.length > 0" class="mt-8">
                <NDivider title-placement="left">
                  <span class="text-13px text-gray-500 font-bold tracking-wider uppercase">
                    {{ $t('page.manage.setting.cache.moduleGroup') }}
                  </span>
                </NDivider>
                <NGrid :x-gap="16" :y-gap="16" cols="1 s:2 m:3" responsive="screen">
                  <NGridItem v-for="item in moduleCaches" :key="item.configKey">
                    <NCard size="small" class="cursor-default transition-colors hover:border-primary">
                      <div class="flex-y-center justify-between">
                        <div>
                          <div class="mb-1 text-16px font-bold">
                            {{ $t(`page.manage.setting.cache.${item.configKey}`) }}
                          </div>
                          <div class="text-12px text-gray-400">Key: {{ item.configKey }}</div>
                        </div>
                        <NSwitch
                          v-model:value="item.configValue"
                          checked-value="true"
                          unchecked-value="false"
                          :loading="updating === item.configKey"
                          @update:value="handleUpdate(item)"
                        />
                      </div>
                    </NCard>
                  </NGridItem>
                </NGrid>
              </div>
            </NSpin>
          </div>
        </NTabPane>

        <!-- Tab 2: Captcha Configuration -->
        <NTabPane name="captcha" :tab="$t('page.manage.setting.tabs.captcha')">
          <div class="pt-4">
            <NAlert type="info" class="mb-6">
              {{ $t('page.manage.setting.captcha.description') }}
            </NAlert>
            <NSpin :show="loading">
              <NDivider title-placement="left">
                <span class="text-13px text-gray-500 font-bold tracking-wider uppercase">
                  {{ $t('page.manage.setting.captcha.switches') }}
                </span>
              </NDivider>
              <NGrid :x-gap="16" :y-gap="16" cols="1 s:2 m:3" responsive="screen">
                <NGridItem v-for="item in captchaConfigs.switches" :key="item.configKey">
                  <NCard size="small" class="cursor-default transition-colors hover:border-primary">
                    <div class="flex-y-center justify-between">
                      <div>
                        <div class="mb-1 flex items-center text-16px font-bold">
                          {{ $t(`page.manage.setting.captcha.${item.configKey}`) }}
                        </div>
                        <div class="text-12px text-gray-400">Key: {{ item.configKey }}</div>
                      </div>
                      <NSwitch
                        v-model:value="item.configValue"
                        checked-value="true"
                        unchecked-value="false"
                        :loading="updating === item.configKey"
                        @update:value="handleUpdate(item)"
                      />
                    </div>
                  </NCard>
                </NGridItem>
              </NGrid>

              <NDivider title-placement="left" class="mt-8">
                <span class="text-13px text-gray-500 font-bold tracking-wider uppercase">
                  {{ $t('page.manage.setting.captcha.params') }}
                </span>
              </NDivider>
              <div class="max-w-600px">
                <NSpace vertical :size="20">
                  <NFormItem :label="$t('page.manage.setting.captcha.type')" label-placement="left">
                    <NSelect
                      v-if="captchaConfigs.params.captcha_type"
                      v-model:value="captchaConfigs.params.captcha_type.configValue"
                      :options="captchaTypeOptions"
                      class="w-240px"
                      @update:value="handleUpdate(captchaConfigs.params.captcha_type)"
                    />
                  </NFormItem>
                  <NFormItem :label="$t('page.manage.setting.captcha.length')" label-placement="left">
                    <NInputNumber
                      v-if="captchaConfigs.params.captcha_length"
                      v-model:value="captchaConfigs.params.captcha_length.numValue"
                      :min="2"
                      :max="10"
                      class="w-240px"
                      @blur="handleNumberUpdate(captchaConfigs.params.captcha_length)"
                    />
                  </NFormItem>
                  <NFormItem :label="$t('page.manage.setting.captcha.width')" label-placement="left">
                    <NInputNumber
                      v-if="captchaConfigs.params.captcha_width"
                      v-model:value="captchaConfigs.params.captcha_width.numValue"
                      :min="100"
                      :max="1000"
                      class="w-240px"
                      @blur="handleNumberUpdate(captchaConfigs.params.captcha_width)"
                    />
                  </NFormItem>
                  <NFormItem :label="$t('page.manage.setting.captcha.height')" label-placement="left">
                    <NInputNumber
                      v-if="captchaConfigs.params.captcha_height"
                      v-model:value="captchaConfigs.params.captcha_height.numValue"
                      :min="30"
                      :max="500"
                      class="w-240px"
                      @blur="handleNumberUpdate(captchaConfigs.params.captcha_height)"
                    />
                  </NFormItem>
                  <NFormItem :label="$t('page.manage.setting.captcha.expire')" label-placement="left">
                    <NInputNumber
                      v-if="captchaConfigs.params.captcha_expire"
                      v-model:value="captchaConfigs.params.captcha_expire.numValue"
                      :min="30"
                      :max="3600"
                      class="w-240px"
                      @blur="handleNumberUpdate(captchaConfigs.params.captcha_expire)"
                    />
                  </NFormItem>
                </NSpace>
              </div>
            </NSpin>
          </div>
        </NTabPane>

        <!-- Tab 3: User Configuration -->
        <NTabPane name="user" :tab="$t('page.manage.setting.tabs.user')">
          <div class="pt-4">
            <NAlert type="info" class="mb-6">
              {{ $t('page.manage.setting.user.description') }}
            </NAlert>
            <NSpin :show="loading">
              <div class="max-w-1000px">
                <NGrid :x-gap="24" :y-gap="24" cols="1 s:1 m:2" responsive="screen">
                  <NGridItem>
                    <!-- Basic & Storage -->
                    <NCard :title="$t('page.manage.setting.user.basic')" size="small" class="h-full">
                      <NSpace vertical :size="16">
                        <NFormItem :label="$t('page.manage.setting.user.storage_module')" label-placement="left">
                          <NSelect
                            v-if="userConfigs.storage_module"
                            v-model:value="userConfigs.storage_module.configValue"
                            :options="storageOptions"
                            clearable
                            :placeholder="$t('page.manage.setting.user.storage_module_placeholder')"
                            class="w-240px"
                            @update:value="handleUpdate(userConfigs.storage_module)"
                          />
                        </NFormItem>
                        <NFormItem :label="$t('page.manage.setting.user.login_storage')" label-placement="left">
                          <NSelect
                            v-if="userConfigs.login_storage"
                            v-model:value="userConfigs.login_storage.configValue"
                            :options="loginStorageOptions"
                            class="w-240px"
                            @update:value="handleUpdate(userConfigs.login_storage)"
                          />
                        </NFormItem>
                        <NFormItem :label="$t('page.manage.setting.user.token_expire')" label-placement="left">
                          <NInputNumber
                            v-if="userConfigs.token_expire"
                            v-model:value="userConfigs.token_expire.numValue"
                            :min="60"
                            class="w-240px"
                            @blur="handleNumberUpdate(userConfigs.token_expire)"
                          />
                        </NFormItem>
                        <NFormItem :label="$t('page.manage.setting.user.login_max_retry')" label-placement="left">
                          <NInputNumber
                            v-if="userConfigs.login_max_retry"
                            v-model:value="userConfigs.login_max_retry.numValue"
                            :min="1"
                            class="w-240px"
                            @blur="handleNumberUpdate(userConfigs.login_max_retry)"
                          />
                        </NFormItem>
                        <NFormItem :label="$t('page.manage.setting.user.login_lock_duration')" label-placement="left">
                          <NInputNumber
                            v-if="userConfigs.login_lock_duration"
                            v-model:value="userConfigs.login_lock_duration.numValue"
                            :min="1"
                            class="w-240px"
                            @blur="handleNumberUpdate(userConfigs.login_lock_duration)"
                          />
                        </NFormItem>
                        <NFormItem :label="$t('page.manage.setting.user.password_min_length')" label-placement="left">
                          <NInputNumber
                            v-if="userConfigs.password_min_length"
                            v-model:value="userConfigs.password_min_length.numValue"
                            :min="4"
                            class="w-240px"
                            @blur="handleNumberUpdate(userConfigs.password_min_length)"
                          />
                        </NFormItem>
                        <NFormItem
                          :label="$t('page.manage.setting.user.password_require_types')"
                          label-placement="left"
                        >
                          <NInputNumber
                            v-if="userConfigs.password_require_types"
                            v-model:value="userConfigs.password_require_types.numValue"
                            :min="1"
                            :max="4"
                            class="w-240px"
                            @blur="handleNumberUpdate(userConfigs.password_require_types)"
                          />
                        </NFormItem>
                      </NSpace>
                    </NCard>
                  </NGridItem>

                  <NGridItem>
                    <!-- Verify Settings -->
                    <NCard :title="$t('page.manage.setting.user.verify')" size="small" class="h-full">
                      <NSpace vertical :size="16">
                        <!-- Register Verify -->
                        <div class="border border-gray-100 rounded-8px bg-gray-50/30 p-4">
                          <NFormItem
                            :label="$t('page.manage.setting.user.user_register_verify')"
                            label-placement="left"
                          >
                            <NSwitch
                              v-if="userConfigs.user_register_verify"
                              v-model:value="userConfigs.user_register_verify.configValue"
                              checked-value="true"
                              unchecked-value="false"
                              :loading="updating === userConfigs.user_register_verify.configKey"
                              @update:value="handleUpdate(userConfigs.user_register_verify)"
                            />
                          </NFormItem>
                          <NFormItem
                            v-if="userConfigs.user_register_verify?.configValue === 'true'"
                            :label="$t('page.manage.setting.user.user_register_verify_type')"
                            label-placement="left"
                            class="mt-2"
                          >
                            <NSelect
                              v-if="userConfigs.user_register_verify_type"
                              v-model:value="userConfigs.user_register_verify_type.configValue"
                              :options="verifyTypeOptions"
                              class="w-240px"
                              @update:value="handleUpdate(userConfigs.user_register_verify_type)"
                            />
                          </NFormItem>
                        </div>

                        <!-- Reset Pwd Verify -->
                        <div class="mt-4 border border-gray-100 rounded-8px bg-gray-50/30 p-4">
                          <NFormItem
                            :label="$t('page.manage.setting.user.user_reset_pwd_verify')"
                            label-placement="left"
                          >
                            <NSwitch
                              v-if="userConfigs.user_reset_pwd_verify"
                              v-model:value="userConfigs.user_reset_pwd_verify.configValue"
                              checked-value="true"
                              unchecked-value="false"
                              :loading="updating === userConfigs.user_reset_pwd_verify.configKey"
                              @update:value="handleUpdate(userConfigs.user_reset_pwd_verify)"
                            />
                          </NFormItem>
                          <NFormItem
                            v-if="userConfigs.user_reset_pwd_verify?.configValue === 'true'"
                            :label="$t('page.manage.setting.user.user_reset_pwd_verify_type')"
                            label-placement="left"
                            class="mt-2"
                          >
                            <NSelect
                              v-if="userConfigs.user_reset_pwd_verify_type"
                              v-model:value="userConfigs.user_reset_pwd_verify_type.configValue"
                              :options="verifyTypeOptions"
                              class="w-240px"
                              @update:value="handleUpdate(userConfigs.user_reset_pwd_verify_type)"
                            />
                          </NFormItem>
                        </div>
                      </NSpace>
                    </NCard>
                  </NGridItem>
                </NGrid>
              </div>
            </NSpin>
          </div>
        </NTabPane>

        <!-- Tab 2: Task Configuration -->
        <NTabPane name="task" :tab="$t('page.manage.setting.tabs.task')">
          <div class="max-w-600px pt-4">
            <NCard :title="$t('page.manage.setting.log.taskLog')" size="small">
              <NSpace vertical :size="20">
                <NFormItem :label="$t('page.manage.setting.log.enabled')" label-placement="left">
                  <NSwitch
                    v-if="logConfigs.task_log_enabled !== undefined"
                    v-model:value="logConfigs.task_log_enabled.configValue"
                    checked-value="true"
                    unchecked-value="false"
                    :loading="updating === logConfigs.task_log_enabled?.configKey"
                    @update:value="handleUpdate(logConfigs.task_log_enabled)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.log.retentionDays')" label-placement="left">
                  <NInputNumber
                    v-if="logConfigs.task_retention !== undefined"
                    v-model:value="logConfigs.task_retention.numValue"
                    :min="0"
                    :max="3650"
                    class="w-200px"
                    @blur="handleNumberUpdate(logConfigs.task_retention)"
                  >
                    <template #suffix>{{ $t('page.manage.setting.log.daysUnit') }}</template>
                  </NInputNumber>
                </NFormItem>
                <NAlert type="info" size="small">
                  {{ $t('page.manage.setting.log.description') }}
                </NAlert>
              </NSpace>
            </NCard>
          </div>
        </NTabPane>

        <!-- Tab 3: Log Maintenance -->
        <NTabPane name="log" :tab="$t('page.manage.setting.tabs.log')">
          <div class="max-w-1000px pt-4">
            <NGrid :x-gap="24" :y-gap="24" cols="1 s:1 m:2" responsive="screen">
              <NGridItem>
                <!-- Ops Logs -->
                <NCard :title="$t('page.manage.setting.log.operationLog')" size="small">
                  <NFormItem :label="$t('page.manage.setting.log.retentionDays')">
                    <NInputNumber
                      v-if="logConfigs.ops_retention !== undefined"
                      v-model:value="logConfigs.ops_retention.numValue"
                      :min="0"
                      :max="3650"
                      class="w-full"
                      @blur="handleNumberUpdate(logConfigs.ops_retention)"
                    >
                      <template #suffix>{{ $t('page.manage.setting.log.daysUnit') }}</template>
                    </NInputNumber>
                  </NFormItem>
                </NCard>
              </NGridItem>

              <NGridItem>
                <!-- Error Logs -->
                <NCard :title="$t('page.manage.setting.log.errorLog')" size="small">
                  <NFormItem :label="$t('page.manage.setting.log.retentionDays')">
                    <NInputNumber
                      v-if="logConfigs.err_retention !== undefined"
                      v-model:value="logConfigs.err_retention.numValue"
                      :min="0"
                      :max="3650"
                      class="w-full"
                      @blur="handleNumberUpdate(logConfigs.err_retention)"
                    >
                      <template #suffix>{{ $t('page.manage.setting.log.daysUnit') }}</template>
                    </NInputNumber>
                  </NFormItem>
                </NCard>
              </NGridItem>

              <NGridItem>
                <!-- Message Record -->
                <NCard :title="$t('page.manage.setting.log.msgRecord')" size="small">
                  <NFormItem :label="$t('page.manage.setting.log.retentionDays')">
                    <NInputNumber
                      v-if="logConfigs.msg_record_retention !== undefined"
                      v-model:value="logConfigs.msg_record_retention.numValue"
                      :min="0"
                      :max="3650"
                      class="w-full"
                      @blur="handleNumberUpdate(logConfigs.msg_record_retention)"
                    >
                      <template #suffix>{{ $t('page.manage.setting.log.daysUnit') }}</template>
                    </NInputNumber>
                  </NFormItem>
                </NCard>
              </NGridItem>
            </NGrid>
          </div>
        </NTabPane>

        <!-- Tab: Email Configuration -->
        <NTabPane name="email" :tab="$t('page.manage.setting.tabs.email')">
          <div class="max-w-600px pt-4">
            <NAlert type="info" class="mb-6">
              {{ $t('page.manage.setting.email.description') }}
            </NAlert>
            <NSpin :show="loading">
              <NSpace vertical :size="20">
                <NFormItem :label="$t('page.manage.setting.email.enabled')" label-placement="left">
                  <NSwitch
                    v-if="emailConfigs.enabled"
                    v-model:value="emailConfigs.enabled.configValue"
                    checked-value="true"
                    unchecked-value="false"
                    :loading="updating === emailConfigs.enabled?.configKey"
                    @update:value="handleUpdate(emailConfigs.enabled)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.email.host')" label-placement="left">
                  <NInput
                    v-if="emailConfigs.host"
                    v-model:value="emailConfigs.host.configValue"
                    :placeholder="$t('page.manage.setting.email.hostPlaceholder')"
                    class="w-300px"
                    @blur="handleUpdate(emailConfigs.host)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.email.port')" label-placement="left">
                  <NInputNumber
                    v-if="emailConfigs.port"
                    v-model:value="emailConfigs.port.numValue"
                    :min="1"
                    :max="65535"
                    class="w-200px"
                    @blur="handleNumberUpdate(emailConfigs.port)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.email.sslEnabled')" label-placement="left">
                  <NSwitch
                    v-if="emailConfigs.ssl_enabled"
                    v-model:value="emailConfigs.ssl_enabled.configValue"
                    checked-value="true"
                    unchecked-value="false"
                    :loading="updating === emailConfigs.ssl_enabled?.configKey"
                    @update:value="handleUpdate(emailConfigs.ssl_enabled)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.email.starttlsEnabled')" label-placement="left">
                  <NSwitch
                    v-if="emailConfigs.starttls_enabled"
                    v-model:value="emailConfigs.starttls_enabled.configValue"
                    checked-value="true"
                    unchecked-value="false"
                    :loading="updating === emailConfigs.starttls_enabled?.configKey"
                    @update:value="handleUpdate(emailConfigs.starttls_enabled)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.email.authType')" label-placement="left">
                  <NSelect
                    v-if="emailConfigs.auth_type"
                    v-model:value="emailConfigs.auth_type.configValue"
                    :options="emailAuthTypeOptions"
                    class="w-200px"
                    @update:value="handleUpdate(emailConfigs.auth_type)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.email.connectTimeout')" label-placement="left">
                  <NInputNumber
                    v-if="emailConfigs.connect_timeout"
                    v-model:value="emailConfigs.connect_timeout.numValue"
                    :min="5"
                    :max="120"
                    class="w-200px"
                    @blur="handleNumberUpdate(emailConfigs.connect_timeout)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.email.sendTimeout')" label-placement="left">
                  <NInputNumber
                    v-if="emailConfigs.send_timeout"
                    v-model:value="emailConfigs.send_timeout.numValue"
                    :min="5"
                    :max="120"
                    class="w-200px"
                    @blur="handleNumberUpdate(emailConfigs.send_timeout)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.email.user')" label-placement="left">
                  <NInput
                    v-if="emailConfigs.user"
                    v-model:value="emailConfigs.user.configValue"
                    :placeholder="$t('page.manage.setting.email.userPlaceholder')"
                    class="w-300px"
                    @blur="handleUpdate(emailConfigs.user)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.email.password')" label-placement="left">
                  <NInput
                    v-if="emailConfigs.password"
                    v-model:value="emailConfigs.password.configValue"
                    type="password"
                    show-password-on="click"
                    :placeholder="$t('page.manage.setting.email.passwordPlaceholder')"
                    class="w-300px"
                    @blur="handleUpdate(emailConfigs.password)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.email.from')" label-placement="left">
                  <NInput
                    v-if="emailConfigs.from"
                    v-model:value="emailConfigs.from.configValue"
                    :placeholder="$t('page.manage.setting.email.fromPlaceholder')"
                    class="w-300px"
                    @blur="handleUpdate(emailConfigs.from)"
                  />
                </NFormItem>
                <NDivider />
                <NFormItem :label="$t('page.manage.setting.email.testEmail')" label-placement="left">
                  <NSpace align="center">
                    <NInput
                      v-model:value="testEmailReceiver"
                      :placeholder="$t('page.manage.setting.email.testReceiverPlaceholder')"
                      class="w-250px"
                    />
                    <NButton
                      type="primary"
                      :loading="testEmailLoading"
                      :disabled="!testEmailReceiver"
                      @click="handleTestEmail"
                    >
                      {{ $t('page.manage.setting.email.testSend') }}
                    </NButton>
                  </NSpace>
                </NFormItem>
              </NSpace>
            </NSpin>
          </div>
        </NTabPane>

        <!-- Tab: SMS Configuration -->
        <NTabPane name="sms" :tab="$t('page.manage.setting.tabs.sms')">
          <div class="max-w-600px pt-4">
            <NAlert type="info" class="mb-6">
              {{ $t('page.manage.setting.sms.description') }}
            </NAlert>
            <NSpin :show="loading">
              <NSpace vertical :size="20">
                <NFormItem :label="$t('page.manage.setting.sms.enabled')" label-placement="left">
                  <NSwitch
                    v-if="smsConfigs.enabled"
                    v-model:value="smsConfigs.enabled.configValue"
                    checked-value="true"
                    unchecked-value="false"
                    :loading="updating === smsConfigs.enabled?.configKey"
                    @update:value="handleUpdate(smsConfigs.enabled)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.sms.driver')" label-placement="left">
                  <NSelect
                    v-if="smsConfigs.driver"
                    v-model:value="smsConfigs.driver.configValue"
                    :options="[{ label: 'Tencent Cloud', value: 'tencent' }]"
                    class="w-200px"
                    @update:value="handleUpdate(smsConfigs.driver)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.sms.secretId')" label-placement="left">
                  <NInput
                    v-if="smsConfigs.secret_id"
                    v-model:value="smsConfigs.secret_id.configValue"
                    type="password"
                    show-password-on="click"
                    :placeholder="$t('page.manage.setting.sms.secretIdPlaceholder')"
                    class="w-300px"
                    @blur="handleUpdate(smsConfigs.secret_id)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.sms.secretKey')" label-placement="left">
                  <NInput
                    v-if="smsConfigs.secret_key"
                    v-model:value="smsConfigs.secret_key.configValue"
                    type="password"
                    show-password-on="click"
                    :placeholder="$t('page.manage.setting.sms.secretKeyPlaceholder')"
                    class="w-300px"
                    @blur="handleUpdate(smsConfigs.secret_key)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.sms.appId')" label-placement="left">
                  <NInput
                    v-if="smsConfigs.app_id"
                    v-model:value="smsConfigs.app_id.configValue"
                    :placeholder="$t('page.manage.setting.sms.appIdPlaceholder')"
                    class="w-300px"
                    @blur="handleUpdate(smsConfigs.app_id)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.sms.signName')" label-placement="left">
                  <NInput
                    v-if="smsConfigs.sign_name"
                    v-model:value="smsConfigs.sign_name.configValue"
                    :placeholder="$t('page.manage.setting.sms.signNamePlaceholder')"
                    class="w-300px"
                    @blur="handleUpdate(smsConfigs.sign_name)"
                  />
                </NFormItem>
              </NSpace>
            </NSpin>
          </div>
        </NTabPane>
      </NTabs>
    </NCard>
  </div>
</template>

<style scoped>
.flex-y-center {
  display: flex;
  align-items: center;
}
</style>
