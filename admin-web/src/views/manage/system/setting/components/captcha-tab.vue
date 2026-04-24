<!-- eslint-disable vue/no-mutating-props -->
<script setup lang="ts">
/* eslint-disable vue/no-mutating-props */
import {
  NAlert,
  NCard,
  NDivider,
  NFormItem,
  NGrid,
  NGridItem,
  NInputNumber,
  NSelect,
  NSpace,
  NSpin,
  NSwitch
} from 'naive-ui';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

export interface ConfigItem extends SystemManage.SysConfig {
  numValue?: number;
}

export interface CaptchaConfigs {
  switches: SystemManage.SysConfig[];
  params: Record<string, ConfigItem | undefined>;
}

defineProps<{
  loading: boolean;
  updating: string;
  captchaConfigs: CaptchaConfigs;
  captchaTypeOptions: { label: string; value: string }[];
}>();

const emit = defineEmits<{
  (e: 'update', item: SystemManage.SysConfig): void;
  (e: 'numberUpdate', item: ConfigItem): void;
}>();
</script>

<template>
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
                @update:value="emit('update', item)"
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
              @update:value="emit('update', captchaConfigs.params.captcha_type!)"
            />
          </NFormItem>
          <NFormItem :label="$t('page.manage.setting.captcha.length')" label-placement="left">
            <NInputNumber
              v-if="captchaConfigs.params.captcha_length"
              v-model:value="captchaConfigs.params.captcha_length.numValue"
              :min="2"
              :max="10"
              class="w-240px"
              @blur="emit('numberUpdate', captchaConfigs.params.captcha_length!)"
            />
          </NFormItem>
          <NFormItem :label="$t('page.manage.setting.captcha.width')" label-placement="left">
            <NInputNumber
              v-if="captchaConfigs.params.captcha_width"
              v-model:value="captchaConfigs.params.captcha_width.numValue"
              :min="100"
              :max="1000"
              class="w-240px"
              @blur="emit('numberUpdate', captchaConfigs.params.captcha_width!)"
            />
          </NFormItem>
          <NFormItem :label="$t('page.manage.setting.captcha.height')" label-placement="left">
            <NInputNumber
              v-if="captchaConfigs.params.captcha_height"
              v-model:value="captchaConfigs.params.captcha_height.numValue"
              :min="30"
              :max="500"
              class="w-240px"
              @blur="emit('numberUpdate', captchaConfigs.params.captcha_height!)"
            />
          </NFormItem>
          <NFormItem :label="$t('page.manage.setting.captcha.expire')" label-placement="left">
            <NInputNumber
              v-if="captchaConfigs.params.captcha_expire"
              v-model:value="captchaConfigs.params.captcha_expire.numValue"
              :min="30"
              :max="3600"
              class="w-240px"
              @blur="emit('numberUpdate', captchaConfigs.params.captcha_expire!)"
            />
          </NFormItem>
        </NSpace>
      </div>
    </NSpin>
  </div>
</template>

<style scoped>
.flex-y-center {
  display: flex;
  align-items: center;
}
</style>
