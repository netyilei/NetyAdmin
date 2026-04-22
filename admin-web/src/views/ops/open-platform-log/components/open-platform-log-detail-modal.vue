<script setup lang="ts">
import type { Log } from '@/typings/api/v1/log';
import { $t } from '@/locales';

defineOptions({ name: 'OpenPlatformLogDetailModal' });

const visible = defineModel<boolean>('visible', { default: false });

const rowData = defineModel<Log.OpenLog | null>('rowData', { default: null });

function formatLatency(ns: number) {
  return `${(ns / 1000000).toFixed(2)}ms`;
}

function formatJson(str: string) {
  if (!str) return '-';
  try {
    return JSON.stringify(JSON.parse(str), null, 2);
  } catch {
    return str;
  }
}
</script>

<template>
  <NModal v-model:show="visible" preset="card" :title="$t('common.detail')" class="w-800px" :bordered="false">
    <NDescriptions label-placement="left" bordered :column="2" size="small">
      <NDescriptionsItem label="AppID">{{ rowData?.appId }}</NDescriptionsItem>
      <NDescriptionsItem label="AppKey">{{ rowData?.appKey }}</NDescriptionsItem>
      <NDescriptionsItem label="API路径">{{ rowData?.apiPath }}</NDescriptionsItem>
      <NDescriptionsItem label="请求方法">
        <NTag
          :type="
            rowData?.apiMethod === 'GET'
              ? 'success'
              : rowData?.apiMethod === 'POST'
                ? 'primary'
                : rowData?.apiMethod === 'PUT'
                  ? 'warning'
                  : 'error'
          "
          size="small"
        >
          {{ rowData?.apiMethod }}
        </NTag>
      </NDescriptionsItem>
      <NDescriptionsItem label="状态码">
        <NTag :type="rowData?.statusCode >= 200 && rowData?.statusCode < 300 ? 'success' : 'error'" size="small">
          {{ rowData?.statusCode }}
        </NTag>
      </NDescriptionsItem>
      <NDescriptionsItem label="耗时">{{ formatLatency(rowData?.latency || 0) }}</NDescriptionsItem>
      <NDescriptionsItem label="来源IP">{{ rowData?.clientIp }}</NDescriptionsItem>
      <NDescriptionsItem label="调用时间">{{ rowData?.createdAt }}</NDescriptionsItem>
    </NDescriptions>

    <NDivider title-placement="left">请求头</NDivider>
    <NCode :code="formatJson(rowData?.requestHeader || '')" language="json" word-wrap />

    <NDivider title-placement="left">请求体</NDivider>
    <NCode :code="formatJson(rowData?.requestBody || '')" language="json" word-wrap />

    <NDivider title-placement="left">响应体</NDivider>
    <NCode :code="formatJson(rowData?.responseBody || '')" language="json" word-wrap />

    <template v-if="rowData?.errorMsg">
      <NDivider title-placement="left">错误信息</NDivider>
      <NAlert type="error" :bordered="false">
        {{ rowData?.errorMsg }}
      </NAlert>
    </template>
  </NModal>
</template>
