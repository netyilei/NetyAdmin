<script setup lang="tsx">
import { NButton, NSpace, NTag } from 'naive-ui';
import { fetchRecordList } from '@/service/api/v1/message-hub';
import { useAppStore } from '@/store/modules/app';
import { useTable } from '@/hooks/common/table';
import { $t } from '@/locales';
import { useDict } from '@/hooks/common/dict';

const appStore = useAppStore();
const { renderDictTag } = useDict();

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
    channel: '',
    receiver: '',
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
      width: 160
    } as any,
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 100,
      render: (row: any) => (
        <NSpace justify="center">
          <NButton type="primary" ghost size="small">
            {$t('page.messageHub.record.detail')}
          </NButton>
        </NSpace>
      )
    } as any
  ]
});
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <NCard :title="$t('page.messageHub.record.title')" :bordered="false" size="small" class="sm:flex-1-hidden card-wrapper">
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
    </NCard>
  </div>
</template>

<style scoped></style>
