<!-- eslint-disable vue/no-mutating-props -->
<script setup lang="ts">
/* eslint-disable vue/no-mutating-props */
import { NAlert, NCard, NFormItem, NInputNumber, NSpace, NSwitch } from 'naive-ui';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

export interface ConfigItem extends SystemManage.SysConfig {
  numValue?: number;
}

defineProps<{
  updating: string;
  logConfigs: Record<string, ConfigItem | undefined>;
}>();

const emit = defineEmits<{
  (e: 'update', item: SystemManage.SysConfig): void;
  (e: 'numberUpdate', item: ConfigItem): void;
}>();
</script>

<template>
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
            @update:value="emit('update', logConfigs.task_log_enabled!)"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.log.retentionDays')" label-placement="left">
          <NInputNumber
            v-if="logConfigs.task_retention !== undefined"
            v-model:value="logConfigs.task_retention.numValue"
            :min="0"
            :max="3650"
            class="w-200px"
            @blur="emit('numberUpdate', logConfigs.task_retention!)"
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
</template>
