<script setup lang="ts">
import { watch } from 'vue';
import dayjs from 'dayjs';
import { useDict } from '@/hooks/common/dict';
import type { MessageHub } from '@/typings/api/v1/message-hub';
import { $t } from '@/locales';

defineOptions({
  name: 'MsgRecordDetailModal'
});

interface Props {
  rowData?: MessageHub.Record | null;
}

defineProps<Props>();

const visible = defineModel<boolean>('visible', {
  default: false
});

const { loadDicts, renderDictTag } = useDict();

loadDicts(['sys_msg_channel', 'sys_msg_status', 'sys_msg_priority']);

function closeModal() {
  visible.value = false;
}

watch(
  () => visible.value,
  val => {
    if (val) {
      loadDicts(['sys_msg_channel', 'sys_msg_status', 'sys_msg_priority']);
    }
  }
);
</script>

<template>
  <NModal v-model:show="visible" :title="$t('page.messageHub.record.detail')" preset="card" class="w-600px">
    <NDescriptions label-placement="left" :column="1" bordered size="small">
      <NDescriptionsItem :label="$t('page.messageHub.record.receiver')">
        {{ rowData?.receiver }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.messageHub.record.channel')">
        <component :is="() => renderDictTag('sys_msg_channel', rowData?.channel || '')" />
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.messageHub.record.status')">
        <component :is="() => renderDictTag('sys_msg_status', String(rowData?.status || ''))" />
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.messageHub.record.priority')">
        <component :is="() => renderDictTag('sys_msg_priority', String(rowData?.priority || ''))" />
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.messageHub.record.retryCount')">
        {{ rowData?.retryCount }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.messageHub.record.time')">
        {{ rowData?.createdAt ? dayjs(rowData.createdAt).format('YYYY-MM-DD HH:mm:ss') : '-' }}
      </NDescriptionsItem>
      <NDescriptionsItem v-if="rowData?.title" :label="$t('page.messageHub.template.msgTitle')">
        {{ rowData?.title }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('page.messageHub.template.content')">
        <div class="whitespace-pre-wrap">{{ rowData?.content }}</div>
      </NDescriptionsItem>
      <NDescriptionsItem
        v-if="rowData?.errorMsg"
        :label="$t('page.messageHub.record.errorMsg')"
        label-style="color: red"
      >
        <span class="text-error">{{ rowData?.errorMsg }}</span>
      </NDescriptionsItem>
    </NDescriptions>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="closeModal">{{ $t('common.close') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
