<script setup lang="tsx">
import { NButton, NSpace, NTag } from 'naive-ui';
import { fetchOpenLogList } from '@/service/api/v1/open-log';
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
  apiFn: fetchOpenLogList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 10,
    appId: '',
    appKey: '',
    apiPath: '',
    statusCode: undefined,
    startTime: '',
    endTime: '',
    total: 0
  },
  columns: () => [
    { key: 'index', title: $t('common.index'), align: 'center', width: 64 },
    {
      key: 'appId',
      title: 'AppID',
      align: 'center',
      width: 120
    } as any,
    {
      key: 'apiPath',
      title: 'API路径',
      align: 'center',
      minWidth: 150
    } as any,
    {
      key: 'apiMethod',
      title: '方法',
      align: 'center',
      width: 80,
      render: (row: any) => {
        const methodMap: Record<string, NaiveUI.ThemeColor> = {
          GET: 'success',
          POST: 'primary',
          PUT: 'warning',
          DELETE: 'error'
        };
        return <NTag type={methodMap[row.apiMethod] || 'default'}>{row.apiMethod}</NTag>;
      }
    } as any,
    {
      key: 'statusCode',
      title: '状态码',
      align: 'center',
      width: 100,
      render: (row: any) => {
        const type = row.statusCode >= 200 && row.statusCode < 300 ? 'success' : 'error';
        return <NTag type={type}>{row.statusCode}</NTag>;
      }
    } as any,
    {
      key: 'latency',
      title: '耗时',
      align: 'center',
      width: 100,
      render: (row: any) => {
        const ms = (row.latency / 1000000).toFixed(2);
        return <span>{ms}ms</span>;
      }
    } as any,
    {
      key: 'clientIp',
      title: '来源IP',
      align: 'center',
      width: 130
    } as any,
    {
      key: 'createdAt',
      title: '调用时间',
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
          <NButton type="primary" ghost size="small" onClick={() => viewDetail(row)}>
            {$t('common.detail')}
          </NButton>
        </NSpace>
      )
    } as any
  ]
});

function viewDetail(row: any) {
  // TODO: 实现详情查看弹窗
  window.$message?.info('详情功能开发中...');
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <NCard title="开放平台调用日志" :bordered="false" size="small" class="sm:flex-1-hidden card-wrapper">
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
