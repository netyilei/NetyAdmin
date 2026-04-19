<script setup lang="ts">
import { $t } from '@/locales';

defineOptions({ name: 'OperationLogDetailModal' });

const visible = defineModel<boolean>('visible', { default: false });

const rowData = defineModel<any>('rowData', { default: null });
</script>

<template>
  <NModal v-model:show="visible" preset="card" :title="$t('common.detail')" class="w-700px" :bordered="false">
    <NDescriptions label-placement="left" bordered :column="2" size="small">
      <NDescriptionsItem :label="$t('page.ops.operationLog.id')">{{ rowData?.id }}</NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.ops.operationLog.operator')">{{ rowData?.username }}</NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.ops.operationLog.type')">{{ rowData?.action }}</NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.ops.operationLog.resource')">{{ rowData?.resource }}</NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.ops.operationLog.ip')">{{ rowData?.ip }}</NDescriptionsItem>
      <NDescriptionsItem label="UserAgent">{{ rowData?.userAgent }}</NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.ops.operationLog.time')" :span="2">
        {{ rowData?.createdAt }}
      </NDescriptionsItem>
    </NDescriptions>

    <template v-if="rowData?.detail">
      <NDivider title-placement="left">操作详情</NDivider>
      <NCode :code="rowData.detail" language="text" word-wrap />
    </template>
  </NModal>
</template>
