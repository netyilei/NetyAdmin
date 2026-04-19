<script setup lang="tsx">
import { NButton, NSpace } from 'naive-ui';
import dayjs from 'dayjs';
import { fetchRecordList, retryRecord } from '@/service/api/v1/message-hub';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useAuth } from '@/hooks/business/auth';
import { useDict } from '@/hooks/common/dict';
import { $t } from '@/locales';
import MsgRecordSearch from './components/msg-record-search.vue';
import MsgRecordDetailModal from './components/msg-record-detail-modal.vue';

const appStore = useAppStore();
const { hasAuth } = useAuth();
const { renderDictTag } = useDict();

let handleViewDetailFn: (row: any) => void;

const {
  columns,
  columnChecks,
  data,
  getData,
  getDataByPage,
  loading,
  mobilePagination,
  searchParams,
  resetSearchParams
} = useTable({
  apiFn: fetchRecordList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 10,
    channel: undefined,
    receiver: '',
    status: undefined,
    total: 0
  },
  columns: () => [
    { key: 'index', title: $t('common.index'), align: 'center', width: 64 },
    {
      key: 'receiver',
      title: $t('page.messageHub.record.receiver'),
      align: 'center',
      minWidth: 150
    } as any,
    {
      key: 'channel',
      title: $t('page.messageHub.record.channel'),
      align: 'center',
      width: 100,
      render: (row: any) => renderDictTag('sys_msg_channel', row.channel)
    } as any,
    {
      key: 'status',
      title: $t('page.messageHub.record.status'),
      align: 'center',
      width: 100,
      render: (row: any) => renderDictTag('sys_msg_status', String(row.status))
    } as any,
    {
      key: 'priority',
      title: $t('page.messageHub.record.priority'),
      align: 'center',
      width: 100,
      render: (row: any) => renderDictTag('sys_msg_priority', String(row.priority))
    } as any,
    {
      key: 'createdAt',
      title: $t('page.messageHub.record.time'),
      align: 'center',
      width: 160,
      render: (row: any) => (row.createdAt ? dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss') : '-')
    } as any,
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 180,
      render: (row: any) => (
        <NSpace justify="center">
          {hasAuth('msg:record:query') && (
            <NButton type="primary" ghost size="small" onClick={() => handleViewDetailFn(row)}>
              {$t('page.messageHub.record.detail')}
            </NButton>
          )}
          {hasAuth('msg:log:retry') && row.status === 2 && (
            <NButton type="warning" ghost size="small" onClick={() => handleRetry(row.id)}>
              {$t('page.messageHub.record.retry')}
            </NButton>
          )}
        </NSpace>
      )
    } as any
  ]
});

const { drawerVisible, editingData, handleEdit: handleViewDetail } = useTableOperate(data, getData);
handleViewDetailFn = handleViewDetail;

async function handleRetry(id: number) {
  loading.value = true;
  const { error } = await retryRecord(id);
  loading.value = false;
  if (!error) {
    window.$message?.success($t('common.operationSuccess'));
    getData();
  }
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <MsgRecordSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />
    <NCard
      :title="$t('page.messageHub.record.title')"
      :bordered="false"
      size="small"
      class="card-wrapper sm:flex-1-hidden"
    >
      <template #header-extra>
        <TableHeaderOperation v-model:columns="columnChecks" :show-add="false" :loading="loading" @refresh="getData" />
      </template>
      <NDataTable
        remote
        striped
        size="small"
        class="sm:flex-1-hidden"
        :data="data"
        :columns="columns"
        :flex-height="!appStore.isMobile"
        :loading="loading"
        :single-line="false"
        :row-key="row => row.id"
        :pagination="mobilePagination"
        @update:page="getDataByPage"
      />
      <MsgRecordDetailModal v-model:visible="drawerVisible" :row-data="editingData" />
    </NCard>
  </div>
</template>

<style scoped></style>
