<!-- eslint-disable vue/no-mutating-props -->
<script setup lang="ts">
/* eslint-disable vue/no-mutating-props */
import { NAlert, NCard, NFormItem, NGrid, NGridItem, NInputNumber, NSelect, NSpace, NSpin, NSwitch } from 'naive-ui';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

export interface ConfigItem extends SystemManage.SysConfig {
  numValue?: number;
}

defineProps<{
  loading: boolean;
  updating: string;
  userConfigs: Record<string, ConfigItem | undefined>;
  storageOptions: { label: string; value: string }[];
  loginStorageOptions: { label: string; value: string }[];
  verifyTypeOptions: { label: string; value: string }[];
}>();

const emit = defineEmits<{
  (e: 'update', item: SystemManage.SysConfig): void;
  (e: 'numberUpdate', item: ConfigItem): void;
}>();
</script>

<template>
  <div class="pt-4">
    <NAlert type="info" class="mb-6">
      {{ $t('page.manage.setting.user.description') }}
    </NAlert>
    <NSpin :show="loading">
      <div class="max-w-1000px">
        <NGrid :x-gap="24" :y-gap="24" cols="1 s:1 m:2" responsive="screen">
          <NGridItem>
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
                    @update:value="emit('update', userConfigs.storage_module)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.user.login_storage')" label-placement="left">
                  <NSelect
                    v-if="userConfigs.login_storage"
                    v-model:value="userConfigs.login_storage.configValue"
                    :options="loginStorageOptions"
                    class="w-240px"
                    @update:value="emit('update', userConfigs.login_storage)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.user.token_expire')" label-placement="left">
                  <NInputNumber
                    v-if="userConfigs.token_expire"
                    v-model:value="userConfigs.token_expire.numValue"
                    :min="60"
                    class="w-240px"
                    @blur="emit('numberUpdate', userConfigs.token_expire)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.user.login_max_retry')" label-placement="left">
                  <NInputNumber
                    v-if="userConfigs.login_max_retry"
                    v-model:value="userConfigs.login_max_retry.numValue"
                    :min="1"
                    class="w-240px"
                    @blur="emit('numberUpdate', userConfigs.login_max_retry)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.user.login_lock_duration')" label-placement="left">
                  <NInputNumber
                    v-if="userConfigs.login_lock_duration"
                    v-model:value="userConfigs.login_lock_duration.numValue"
                    :min="1"
                    class="w-240px"
                    @blur="emit('numberUpdate', userConfigs.login_lock_duration)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.user.password_min_length')" label-placement="left">
                  <NInputNumber
                    v-if="userConfigs.password_min_length"
                    v-model:value="userConfigs.password_min_length.numValue"
                    :min="4"
                    class="w-240px"
                    @blur="emit('numberUpdate', userConfigs.password_min_length)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.user.password_require_types')" label-placement="left">
                  <NInputNumber
                    v-if="userConfigs.password_require_types"
                    v-model:value="userConfigs.password_require_types.numValue"
                    :min="1"
                    :max="4"
                    class="w-240px"
                    @blur="emit('numberUpdate', userConfigs.password_require_types)"
                  />
                </NFormItem>
              </NSpace>
            </NCard>
          </NGridItem>

          <NGridItem>
            <NCard :title="$t('page.manage.setting.user.verify')" size="small" class="h-full">
              <NSpace vertical :size="16">
                <div class="border border-gray-100 rounded-8px bg-gray-50/30 p-4">
                  <NFormItem :label="$t('page.manage.setting.user.user_register_verify')" label-placement="left">
                    <NSwitch
                      v-if="userConfigs.user_register_verify"
                      v-model:value="userConfigs.user_register_verify.configValue"
                      checked-value="true"
                      unchecked-value="false"
                      :loading="updating === userConfigs.user_register_verify.configKey"
                      @update:value="emit('update', userConfigs.user_register_verify)"
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
                      @update:value="emit('update', userConfigs.user_register_verify_type)"
                    />
                  </NFormItem>
                </div>

                <div class="border border-gray-100 rounded-8px bg-gray-50/30 p-4">
                  <NFormItem :label="$t('page.manage.setting.user.user_login_verify')" label-placement="left">
                    <NSwitch
                      v-if="userConfigs.user_login_verify"
                      v-model:value="userConfigs.user_login_verify.configValue"
                      checked-value="true"
                      unchecked-value="false"
                      :loading="updating === userConfigs.user_login_verify.configKey"
                      @update:value="emit('update', userConfigs.user_login_verify)"
                    />
                  </NFormItem>
                  <NFormItem
                    v-if="userConfigs.user_login_verify?.configValue === 'true'"
                    :label="$t('page.manage.setting.user.user_login_verify_type')"
                    label-placement="left"
                    class="mt-2"
                  >
                    <NSelect
                      v-if="userConfigs.user_login_verify_type"
                      v-model:value="userConfigs.user_login_verify_type.configValue"
                      :options="verifyTypeOptions"
                      class="w-240px"
                      @update:value="emit('update', userConfigs.user_login_verify_type)"
                    />
                  </NFormItem>
                </div>

                <div class="mt-4 border border-gray-100 rounded-8px bg-gray-50/30 p-4">
                  <NFormItem :label="$t('page.manage.setting.user.user_reset_pwd_verify')" label-placement="left">
                    <NSwitch
                      v-if="userConfigs.user_reset_pwd_verify"
                      v-model:value="userConfigs.user_reset_pwd_verify.configValue"
                      checked-value="true"
                      unchecked-value="false"
                      :loading="updating === userConfigs.user_reset_pwd_verify.configKey"
                      @update:value="emit('update', userConfigs.user_reset_pwd_verify)"
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
                      @update:value="emit('update', userConfigs.user_reset_pwd_verify_type)"
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
</template>
