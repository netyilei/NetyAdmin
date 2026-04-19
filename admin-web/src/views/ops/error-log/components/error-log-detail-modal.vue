<script setup lang="ts">
import { $t } from '@/locales';

defineOptions({ name: 'ErrorLogDetailModal' });

const visible = defineModel<boolean>('visible', { default: false });

defineProps<{
  rowData?: any;
}>();
</script>

<template>
  <NModal v-model:show="visible" preset="card" :title="$t('common.detail')" class="w-800px" :bordered="false">
    <NDescriptions label-placement="left" bordered :column="2" size="small">
      <NDescriptionsItem label="RequestID">{{ rowData?.requestId }}</NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.ops.errorLog.level')">
        <NTag
          :type="rowData?.level === 'panic' ? 'error' : rowData?.level === 'error' ? 'warning' : 'default'"
          size="small"
        >
          {{ rowData?.level }}
        </NTag>
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.ops.errorLog.path')">{{ rowData?.path }}</NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.ops.errorLog.method')">{{ rowData?.method }}</NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.ops.errorLog.ip')">{{ rowData?.ip }}</NDescriptionsItem>
      <NDescriptionsItem label="UserAgent">{{ rowData?.userAgent }}</NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.ops.errorLog.status')">
        <NTag :type="rowData?.resolved ? 'success' : 'warning'" size="small">
          {{ rowData?.resolved ? $t('page.ops.errorLog.statusResolved') : $t('page.ops.errorLog.statusPending') }}
        </NTag>
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.ops.errorLog.occurCount')">{{ rowData?.occurrenceCount }}</NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.ops.errorLog.time')">{{ rowData?.createdAt }}</NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.ops.errorLog.lastOccurredAt')">
        {{ rowData?.lastOccurredAt }}
      </NDescriptionsItem>
      <NDescriptionsItem v-if="rowData?.resolvedAt" label="解决时间">{{ rowData?.resolvedAt }}</NDescriptionsItem>
      <NDescriptionsItem v-if="rowData?.resolvedBy" label="解决人ID">{{ rowData?.resolvedBy }}</NDescriptionsItem>
      <NDescriptionsItem label="错误指纹">{{ rowData?.hash }}</NDescriptionsItem>
      <NDescriptionsItem label="分组ID">{{ rowData?.groupId }}</NDescriptionsItem>
    </NDescriptions>

    <NDivider title-placement="left">错误信息</NDivider>
    <NAlert type="error" :bordered="false">
      {{ rowData?.message }}
    </NAlert>

    <template v-if="rowData?.stack">
      <NDivider title-placement="left">堆栈信息</NDivider>
      <NCode :code="rowData.stack" language="text" word-wrap />
    </template>
  </NModal>
</template>
