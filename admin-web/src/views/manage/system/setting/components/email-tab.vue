<!-- eslint-disable vue/no-mutating-props -->
<script setup lang="ts">
/* eslint-disable vue/no-mutating-props */
import { NAlert, NButton, NDivider, NFormItem, NInput, NInputNumber, NSelect, NSpace, NSpin, NSwitch } from 'naive-ui';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

export interface ConfigItem extends SystemManage.SysConfig {
  numValue?: number;
}

defineProps<{
  loading: boolean;
  updating: string;
  emailConfigs: Record<string, ConfigItem | undefined>;
  emailAuthTypeOptions: { label: string; value: string }[];
  testEmailLoading: boolean;
  testEmailReceiver: string;
}>();

const emit = defineEmits<{
  (e: 'update', item: SystemManage.SysConfig): void;
  (e: 'numberUpdate', item: ConfigItem): void;
  (e: 'testEmail'): void;
  (e: 'update:testEmailReceiver', value: string): void;
}>();
</script>

<template>
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
            @update:value="emit('update', emailConfigs.enabled)"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.email.host')" label-placement="left">
          <NInput
            v-if="emailConfigs.host"
            v-model:value="emailConfigs.host.configValue"
            :placeholder="$t('page.manage.setting.email.hostPlaceholder')"
            class="w-300px"
            @blur="emit('update', emailConfigs.host)"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.email.port')" label-placement="left">
          <NInputNumber
            v-if="emailConfigs.port"
            v-model:value="emailConfigs.port.numValue"
            :min="1"
            :max="65535"
            class="w-200px"
            @blur="emit('numberUpdate', emailConfigs.port)"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.email.sslEnabled')" label-placement="left">
          <NSwitch
            v-if="emailConfigs.ssl_enabled"
            v-model:value="emailConfigs.ssl_enabled.configValue"
            checked-value="true"
            unchecked-value="false"
            :loading="updating === emailConfigs.ssl_enabled?.configKey"
            @update:value="emit('update', emailConfigs.ssl_enabled)"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.email.starttlsEnabled')" label-placement="left">
          <NSwitch
            v-if="emailConfigs.starttls_enabled"
            v-model:value="emailConfigs.starttls_enabled.configValue"
            checked-value="true"
            unchecked-value="false"
            :loading="updating === emailConfigs.starttls_enabled?.configKey"
            @update:value="emit('update', emailConfigs.starttls_enabled)"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.email.authType')" label-placement="left">
          <NSelect
            v-if="emailConfigs.auth_type"
            v-model:value="emailConfigs.auth_type.configValue"
            :options="emailAuthTypeOptions"
            class="w-200px"
            @update:value="emit('update', emailConfigs.auth_type)"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.email.connectTimeout')" label-placement="left">
          <NInputNumber
            v-if="emailConfigs.connect_timeout"
            v-model:value="emailConfigs.connect_timeout.numValue"
            :min="5"
            :max="120"
            class="w-200px"
            @blur="emit('numberUpdate', emailConfigs.connect_timeout)"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.email.sendTimeout')" label-placement="left">
          <NInputNumber
            v-if="emailConfigs.send_timeout"
            v-model:value="emailConfigs.send_timeout.numValue"
            :min="5"
            :max="120"
            class="w-200px"
            @blur="emit('numberUpdate', emailConfigs.send_timeout)"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.email.user')" label-placement="left">
          <NInput
            v-if="emailConfigs.user"
            v-model:value="emailConfigs.user.configValue"
            :placeholder="$t('page.manage.setting.email.userPlaceholder')"
            class="w-300px"
            @blur="emit('update', emailConfigs.user)"
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
            @blur="emit('update', emailConfigs.password)"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.email.from')" label-placement="left">
          <NInput
            v-if="emailConfigs.from"
            v-model:value="emailConfigs.from.configValue"
            :placeholder="$t('page.manage.setting.email.fromPlaceholder')"
            class="w-300px"
            @blur="emit('update', emailConfigs.from)"
          />
        </NFormItem>
        <NDivider />
        <NFormItem :label="$t('page.manage.setting.email.testEmail')" label-placement="left">
          <NSpace align="center">
            <NInput
              :value="testEmailReceiver"
              :placeholder="$t('page.manage.setting.email.testReceiverPlaceholder')"
              class="w-250px"
              @update:value="emit('update:testEmailReceiver', $event)"
            />
            <NButton
              type="primary"
              :loading="testEmailLoading"
              :disabled="!testEmailReceiver"
              @click="emit('testEmail')"
            >
              {{ $t('page.manage.setting.email.testSend') }}
            </NButton>
          </NSpace>
        </NFormItem>
      </NSpace>
    </NSpin>
  </div>
</template>
