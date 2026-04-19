<script setup lang="tsx">
import { ref } from 'vue';
import { NButton, NPopconfirm, NTag } from 'naive-ui';
import dayjs from 'dayjs';
import {
  fetchBatchDeleteErrorLog,
  fetchDeleteErrorLog,
  fetchGetErrorLogList,
  fetchResolveErrorLog
} from '@/service/api/v1/log';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useAuth } from '@/hooks/business/auth';
import { $t } from '@/locales';
import ErrorLogSearch from './components/error-log-search.vue';
import ErrorLogDetailModal from './components/error-log-detail-modal.vue';

const appStore = useAppStore();
const { hasAuth } = useAuth();

const detailVisible = ref(false);
const detailRow = ref<any>(null);

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
  apiFn: fetchGetErrorLogList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 20,
    level: undefined,
    resolved: undefined,
    startTime: undefined,
    endTime: undefined
  },
  columns: () => [
    {
      type: 'selection',
      align: 'center',
      width: 48
    },
    {
      key: 'index',
      title: $t('common.index'),
      align: 'center',
      width: 64
    },
    {
      key: 'level',
      title: $t('page.ops.errorLog.level'),
      align: 'center',
      width: 100,
      render: row => {
        const levelMap: Record<string, NaiveUI.ThemeColor> = {
          error: 'warning',
          panic: 'error',
          warn: 'default'
        };
        const labelMap: Record<string, string> = {
          error: $t('page.ops.errorLog.levelError'),
          panic: $t('page.ops.errorLog.levelPanic'),
          warn: $t('page.ops.errorLog.levelWarn')
        };
        const type = levelMap[row.level] || 'default';
        return <NTag type={type}>{labelMap[row.level] || row.level}</NTag>;
      }
    },
    {
      key: 'message',
      title: $t('page.ops.errorLog.message'),
      align: 'left',
      minWidth: 200,
      ellipsis: {
        tooltip: true
      }
    },
    {
      key: 'path',
      title: $t('page.ops.errorLog.path'),
      align: 'center',
      minWidth: 150
    },
    {
      key: 'method',
      title: $t('page.ops.errorLog.method'),
      align: 'center',
      width: 100
    },
    {
      key: 'ip',
      title: $t('page.ops.errorLog.ip'),
      align: 'center',
      width: 130
    },
    {
      key: 'resolved',
      title: $t('page.ops.errorLog.status'),
      align: 'center',
      width: 100,
      render: row => {
        if (row.resolved) {
          return <NTag type="success">{$t('page.ops.errorLog.statusResolved')}</NTag>;
        }
        return <NTag type="warning">{$t('page.ops.errorLog.statusPending')}</NTag>;
      }
    },
    {
      key: 'occurCount',
      title: $t('page.ops.errorLog.occurCount'),
      align: 'center',
      width: 100
    },
    {
      key: 'createdAt',
      title: $t('page.ops.errorLog.time'),
      align: 'center',
      width: 170,
      render: row => (row.createdAt ? dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss') : '-')
    },
    {
      key: 'lastOccurredAt',
      title: $t('page.ops.errorLog.lastOccurredAt'),
      align: 'center',
      width: 170,
      render: row => (row.lastOccurredAt ? dayjs(row.lastOccurredAt).format('YYYY-MM-DD HH:mm:ss') : '-')
    },
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 200,
      render: row => (
        <div class="flex-center gap-8px">
          <NButton type="primary" ghost size="small" onClick={() => viewDetail(row)}>
            {$t('common.detail')}
          </NButton>
          {hasAuth('ops:error-log:resolve') && !row.resolved && (
            <NPopconfirm onPositiveClick={() => handleResolve(row.id)}>
              {{
                default: () => $t('page.ops.errorLog.confirmResolve'),
                trigger: () => (
                  <NButton type="primary" ghost size="small">
                    {$t('page.ops.errorLog.resolve')}
                  </NButton>
                )
              }}
            </NPopconfirm>
          )}
          {hasAuth('ops:error-log:delete') && (
            <NPopconfirm onPositiveClick={() => handleDelete(row.id)}>
              {{
                default: () => $t('common.confirmDelete'),
                trigger: () => (
                  <NButton type="error" ghost size="small">
                    {$t('common.delete')}
                  </NButton>
                )
              }}
            </NPopconfirm>
          )}
        </div>
      )
    }
  ]
});

const { checkedRowKeys, onBatchDeleted, onDeleted } = useTableOperate(data, getData);

async function handleBatchDelete() {
  const ids = checkedRowKeys.value as unknown as number[];
  if (!ids.length) return;

  loading.value = true;
  const { error } = await fetchBatchDeleteErrorLog(ids);
  loading.value = false;
  if (!error) {
    await onBatchDeleted();
  }
}

async function handleDelete(id: number) {
  loading.value = true;
  const { error } = await fetchDeleteErrorLog(id);
  loading.value = false;
  if (!error) {
    await onDeleted();
  }
}

async function handleResolve(id: number) {
  loading.value = true;
  const { error } = await fetchResolveErrorLog(id);
  loading.value = false;
  if (!error) {
    await getData();
  }
}

function viewDetail(row: any) {
  detailRow.value = row;
  detailVisible.value = true;
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <ErrorLogSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />
    <NCard :title="$t('page.ops.errorLog.title')" :bordered="false" size="small" class="card-wrapper sm:flex-1-hidden">
      <template #header-extra>
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :show-add="false"
          :show-delete="hasAuth('ops:error-log:batch-delete')"
          :disabled-delete="checkedRowKeys.length === 0"
          :loading="loading"
          @delete="handleBatchDelete"
          @refresh="getData"
        />
      </template>
      <NDataTable
        v-model:checked-row-keys="checkedRowKeys"
        :columns="columns"
        :data="data"
        size="small"
        :flex-height="!appStore.isMobile"
        :scroll-x="1300"
        :loading="loading"
        remote
        :row-key="row => row.id"
        :pagination="mobilePagination"
        class="sm:h-full"
      />
    </NCard>
    <ErrorLogDetailModal v-model:visible="detailVisible" :row-data="detailRow" />
  </div>
</template>
