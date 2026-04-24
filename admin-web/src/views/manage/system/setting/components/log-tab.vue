<!-- eslint-disable vue/no-mutating-props -->
<script setup lang="ts">
/* eslint-disable vue/no-mutating-props */
import { NAlert, NCard, NDivider, NFormItem, NGrid, NGridItem, NInputNumber, NSpace, NSpin, NSwitch } from 'naive-ui';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

export interface ConfigItem extends SystemManage.SysConfig {
  numValue?: number;
}

defineProps<{
  loading: boolean;
  updating: string;
  logConfigs: Record<string, ConfigItem | undefined>;
  logbusConfigs: Record<string, ConfigItem | undefined>;
}>();

const emit = defineEmits<{
  (e: 'update', item: SystemManage.SysConfig): void;
  (e: 'numberUpdate', item: ConfigItem): void;
}>();
</script>

<template>
  <div class="max-w-1000px pt-4">
    <NGrid :x-gap="24" :y-gap="24" cols="1 s:1 m:2" responsive="screen">
      <NGridItem>
        <NCard :title="$t('page.manage.setting.log.operationLog')" size="small">
          <NFormItem :label="$t('page.manage.setting.log.retentionDays')">
            <NInputNumber
              v-if="logConfigs.ops_retention !== undefined"
              v-model:value="logConfigs.ops_retention.numValue"
              :min="0"
              :max="3650"
              class="w-full"
              @blur="emit('numberUpdate', logConfigs.ops_retention!)"
            >
              <template #suffix>{{ $t('page.manage.setting.log.daysUnit') }}</template>
            </NInputNumber>
          </NFormItem>
        </NCard>
      </NGridItem>

      <NGridItem>
        <NCard :title="$t('page.manage.setting.log.errorLog')" size="small">
          <NFormItem :label="$t('page.manage.setting.log.retentionDays')">
            <NInputNumber
              v-if="logConfigs.err_retention !== undefined"
              v-model:value="logConfigs.err_retention.numValue"
              :min="0"
              :max="3650"
              class="w-full"
              @blur="emit('numberUpdate', logConfigs.err_retention!)"
            >
              <template #suffix>{{ $t('page.manage.setting.log.daysUnit') }}</template>
            </NInputNumber>
          </NFormItem>
        </NCard>
      </NGridItem>

      <NGridItem>
        <NCard :title="$t('page.manage.setting.log.msgRecord')" size="small">
          <NFormItem :label="$t('page.manage.setting.log.retentionDays')">
            <NInputNumber
              v-if="logConfigs.msg_record_retention !== undefined"
              v-model:value="logConfigs.msg_record_retention.numValue"
              :min="0"
              :max="3650"
              class="w-full"
              @blur="emit('numberUpdate', logConfigs.msg_record_retention!)"
            >
              <template #suffix>{{ $t('page.manage.setting.log.daysUnit') }}</template>
            </NInputNumber>
          </NFormItem>
        </NCard>
      </NGridItem>

      <NGridItem>
        <NCard :title="$t('page.manage.setting.log.openLog')" size="small">
          <NFormItem :label="$t('page.manage.setting.log.retentionDays')">
            <NInputNumber
              v-if="logConfigs.open_log_retention !== undefined"
              v-model:value="logConfigs.open_log_retention.numValue"
              :min="0"
              :max="3650"
              class="w-full"
              @blur="emit('numberUpdate', logConfigs.open_log_retention!)"
            >
              <template #suffix>{{ $t('page.manage.setting.log.daysUnit') }}</template>
            </NInputNumber>
          </NFormItem>
        </NCard>
      </NGridItem>
    </NGrid>

    <NCard :title="$t('page.manage.setting.logbus.title')" size="small" class="mt-6">
      <template #header-extra>
        <NAlert type="info" :show-icon="false" size="small" class="max-w-500px">
          {{ $t('page.manage.setting.logbus.description') }}
        </NAlert>
      </template>
      <NSpin :show="loading">
        <NGrid :x-gap="24" :y-gap="16" cols="1 s:2 m:4" responsive="screen">
          <NGridItem>
            <NFormItem :label="$t('page.manage.setting.logbus.globalMaxEntries')" label-placement="top">
              <NInputNumber
                v-if="logbusConfigs.global_max_entries !== undefined"
                v-model:value="logbusConfigs.global_max_entries.numValue"
                :min="100"
                :max="100000"
                class="w-full"
                @blur="emit('numberUpdate', logbusConfigs.global_max_entries!)"
              >
                <template #suffix>{{ $t('page.manage.setting.log.recordsUnit') }}</template>
              </NInputNumber>
            </NFormItem>
          </NGridItem>
          <NGridItem>
            <NFormItem :label="$t('page.manage.setting.logbus.globalMaxBytes')" label-placement="top">
              <NInputNumber
                v-if="logbusConfigs.global_max_bytes_mb !== undefined"
                v-model:value="logbusConfigs.global_max_bytes_mb.numValue"
                :min="1"
                :max="1024"
                class="w-full"
                @blur="emit('numberUpdate', logbusConfigs.global_max_bytes_mb!)"
              >
                <template #suffix>MB</template>
              </NInputNumber>
            </NFormItem>
          </NGridItem>
          <NGridItem>
            <NFormItem :label="$t('page.manage.setting.logbus.defaultBatchSize')" label-placement="top">
              <NInputNumber
                v-if="logbusConfigs.default_batch_size !== undefined"
                v-model:value="logbusConfigs.default_batch_size.numValue"
                :min="10"
                :max="10000"
                class="w-full"
                @blur="emit('numberUpdate', logbusConfigs.default_batch_size!)"
              >
                <template #suffix>{{ $t('page.manage.setting.log.recordsUnit') }}</template>
              </NInputNumber>
            </NFormItem>
          </NGridItem>
          <NGridItem>
            <NFormItem :label="$t('page.manage.setting.logbus.defaultTimeThreshold')" label-placement="top">
              <NInputNumber
                v-if="logbusConfigs.default_time_threshold !== undefined"
                v-model:value="logbusConfigs.default_time_threshold.numValue"
                :min="1"
                :max="3600"
                class="w-full"
                @blur="emit('numberUpdate', logbusConfigs.default_time_threshold!)"
              >
                <template #suffix>{{ $t('page.manage.setting.log.secondsUnit') }}</template>
              </NInputNumber>
            </NFormItem>
          </NGridItem>
          <NGridItem>
            <NFormItem :label="$t('page.manage.setting.logbus.forceSync')" label-placement="left">
              <NSwitch
                v-if="logbusConfigs.force_sync !== undefined"
                v-model:value="logbusConfigs.force_sync.configValue"
                checked-value="true"
                unchecked-value="false"
                :loading="updating === logbusConfigs.force_sync?.configKey"
                @update:value="emit('update', logbusConfigs.force_sync!)"
              />
            </NFormItem>
          </NGridItem>
        </NGrid>

        <NDivider title-placement="left" class="mt-2">
          <span class="text-13px text-gray-500 font-bold tracking-wider uppercase">
            {{ $t('page.manage.setting.logbus.bucketConfig') }}
          </span>
        </NDivider>
        <NGrid :x-gap="24" :y-gap="16" cols="1 s:2 m:4" responsive="screen">
          <NGridItem>
            <NCard size="small" :title="$t('page.manage.setting.logbus.operationBucket')" class="h-full">
              <NSpace vertical :size="12">
                <NFormItem :label="$t('page.manage.setting.logbus.batchSize')" label-placement="left">
                  <NInputNumber
                    v-if="logbusConfigs.operation_batch_size !== undefined"
                    v-model:value="logbusConfigs.operation_batch_size.numValue"
                    :min="10"
                    :max="10000"
                    class="w-full"
                    @blur="emit('numberUpdate', logbusConfigs.operation_batch_size!)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.logbus.timeThreshold')" label-placement="left">
                  <NInputNumber
                    v-if="logbusConfigs.operation_time_threshold !== undefined"
                    v-model:value="logbusConfigs.operation_time_threshold.numValue"
                    :min="1"
                    :max="3600"
                    class="w-full"
                    @blur="emit('numberUpdate', logbusConfigs.operation_time_threshold!)"
                  >
                    <template #suffix>{{ $t('page.manage.setting.log.secondsUnit') }}</template>
                  </NInputNumber>
                </NFormItem>
              </NSpace>
            </NCard>
          </NGridItem>
          <NGridItem>
            <NCard size="small" :title="$t('page.manage.setting.logbus.errorBucket')" class="h-full">
              <NSpace vertical :size="12">
                <NFormItem :label="$t('page.manage.setting.logbus.batchSize')" label-placement="left">
                  <NInputNumber
                    v-if="logbusConfigs.error_batch_size !== undefined"
                    v-model:value="logbusConfigs.error_batch_size.numValue"
                    :min="10"
                    :max="10000"
                    class="w-full"
                    @blur="emit('numberUpdate', logbusConfigs.error_batch_size!)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.logbus.timeThreshold')" label-placement="left">
                  <NInputNumber
                    v-if="logbusConfigs.error_time_threshold !== undefined"
                    v-model:value="logbusConfigs.error_time_threshold.numValue"
                    :min="1"
                    :max="3600"
                    class="w-full"
                    @blur="emit('numberUpdate', logbusConfigs.error_time_threshold!)"
                  >
                    <template #suffix>{{ $t('page.manage.setting.log.secondsUnit') }}</template>
                  </NInputNumber>
                </NFormItem>
              </NSpace>
            </NCard>
          </NGridItem>
          <NGridItem>
            <NCard size="small" :title="$t('page.manage.setting.logbus.openBucket')" class="h-full">
              <NSpace vertical :size="12">
                <NFormItem :label="$t('page.manage.setting.logbus.batchSize')" label-placement="left">
                  <NInputNumber
                    v-if="logbusConfigs.open_batch_size !== undefined"
                    v-model:value="logbusConfigs.open_batch_size.numValue"
                    :min="10"
                    :max="10000"
                    class="w-full"
                    @blur="emit('numberUpdate', logbusConfigs.open_batch_size!)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.logbus.timeThreshold')" label-placement="left">
                  <NInputNumber
                    v-if="logbusConfigs.open_time_threshold !== undefined"
                    v-model:value="logbusConfigs.open_time_threshold.numValue"
                    :min="1"
                    :max="3600"
                    class="w-full"
                    @blur="emit('numberUpdate', logbusConfigs.open_time_threshold!)"
                  >
                    <template #suffix>{{ $t('page.manage.setting.log.secondsUnit') }}</template>
                  </NInputNumber>
                </NFormItem>
              </NSpace>
            </NCard>
          </NGridItem>
          <NGridItem>
            <NCard size="small" :title="$t('page.manage.setting.logbus.taskBucket')" class="h-full">
              <NSpace vertical :size="12">
                <NFormItem :label="$t('page.manage.setting.logbus.batchSize')" label-placement="left">
                  <NInputNumber
                    v-if="logbusConfigs.task_batch_size !== undefined"
                    v-model:value="logbusConfigs.task_batch_size.numValue"
                    :min="10"
                    :max="10000"
                    class="w-full"
                    @blur="emit('numberUpdate', logbusConfigs.task_batch_size!)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.logbus.timeThreshold')" label-placement="left">
                  <NInputNumber
                    v-if="logbusConfigs.task_time_threshold !== undefined"
                    v-model:value="logbusConfigs.task_time_threshold.numValue"
                    :min="1"
                    :max="3600"
                    class="w-full"
                    @blur="emit('numberUpdate', logbusConfigs.task_time_threshold!)"
                  >
                    <template #suffix>{{ $t('page.manage.setting.log.secondsUnit') }}</template>
                  </NInputNumber>
                </NFormItem>
              </NSpace>
            </NCard>
          </NGridItem>
        </NGrid>
      </NSpin>
    </NCard>
  </div>
</template>
