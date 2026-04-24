<!-- eslint-disable vue/no-mutating-props -->
<script setup lang="ts">
/* eslint-disable vue/no-mutating-props */
import { NAlert, NFormItem, NInput, NSelect, NSpace, NSpin, NSwitch } from 'naive-ui';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

export interface ConfigItem extends SystemManage.SysConfig {
  numValue?: number;
}

defineProps<{
  loading: boolean;
  updating: string;
  smsConfigs: Record<string, ConfigItem | undefined>;
}>();

const emit = defineEmits<{
  (e: 'update', item: SystemManage.SysConfig): void;
}>();
</script>

<template>
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
            @update:value="emit('update', smsConfigs.enabled)"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.sms.driver')" label-placement="left">
          <NSelect
            v-if="smsConfigs.driver"
            v-model:value="smsConfigs.driver.configValue"
            :options="[{ label: 'Tencent Cloud', value: 'tencent' }]"
            class="w-200px"
            @update:value="emit('update', smsConfigs.driver)"
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
            @blur="emit('update', smsConfigs.secret_id)"
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
            @blur="emit('update', smsConfigs.secret_key)"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.sms.appId')" label-placement="left">
          <NInput
            v-if="smsConfigs.app_id"
            v-model:value="smsConfigs.app_id.configValue"
            :placeholder="$t('page.manage.setting.sms.appIdPlaceholder')"
            class="w-300px"
            @blur="emit('update', smsConfigs.app_id)"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.sms.signName')" label-placement="left">
          <NInput
            v-if="smsConfigs.sign_name"
            v-model:value="smsConfigs.sign_name.configValue"
            :placeholder="$t('page.manage.setting.sms.signNamePlaceholder')"
            class="w-300px"
            @blur="emit('update', smsConfigs.sign_name)"
          />
        </NFormItem>
      </NSpace>
    </NSpin>
  </div>
</template>
