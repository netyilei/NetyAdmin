<script setup lang="tsx">
import { NButton, NPopconfirm, NTag } from 'naive-ui';
import { fetchBatchDeleteOperationLog, fetchDeleteOperationLog, fetchGetOperationLogList } from '@/service/api/v1/log';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useAuth } from '@/hooks/business/auth';
import { $t } from '@/locales';

const appStore = useAppStore();
const { hasAuth } = useAuth();

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
  apiFn: fetchGetOperationLogList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 20,
    userId: undefined,
    action: undefined,
    startDate: undefined,
    endDate: undefined
  },
  columns: () => [
    {
      type: 'selection',
      align: 'center',
      width: 48
    },
    {
      key: 'id',
      title: $t('page.ops.operationLog.id'),
      align: 'center',
      width: 64
    },
    {
      key: 'username',
      title: $t('page.ops.operationLog.operator'),
      align: 'center',
      minWidth: 100
    },
    {
      key: 'action',
      title: $t('page.ops.operationLog.type'),
      align: 'center',
      width: 100,
      render: row => {
        const actionMap: Record<string, NaiveUI.ThemeColor> = {
          [$t('page.ops.operationLog.actionCreate')]: 'success',
          [$t('page.ops.operationLog.actionUpdate')]: 'warning',
          [$t('page.ops.operationLog.actionDelete')]: 'error'
        };
        const type = actionMap[row.action] || 'default';
        return <NTag type={type}>{row.action}</NTag>;
      }
    },
    {
      key: 'resource',
      title: $t('page.ops.operationLog.resource'),
      align: 'center',
      minWidth: 120
    },
    {
      key: 'detail',
      title: $t('page.ops.operationLog.detail'),
      align: 'left',
      minWidth: 200,
      ellipsis: {
        tooltip: true
      }
    },
    {
      key: 'ip',
      title: $t('page.ops.operationLog.ip'),
      align: 'center',
      width: 130
    },
    {
      key: 'createdAt',
      title: $t('page.ops.operationLog.time'),
      align: 'center',
      width: 170
    },
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 100,
      render: row => (
        <div class="flex-center gap-8px">
          {hasAuth('ops:operation-log:delete') && (
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
  const { error } = await fetchBatchDeleteOperationLog(ids);
  loading.value = false;
  if (!error) {
    await onBatchDeleted();
  }
}

async function handleDelete(id: number) {
  loading.value = true;
  const { error } = await fetchDeleteOperationLog(id);
  loading.value = false;
  if (!error) {
    await onDeleted();
  }
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <NCard
      :title="$t('page.ops.operationLog.title')"
      :bordered="false"
      size="small"
      class="card-wrapper sm:flex-1-hidden"
    >
      <template #header-extra>
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :show-add="false"
          :show-delete="hasAuth('ops:operation-log:batch-delete')"
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
        :scroll-x="1200"
        :loading="loading"
        remote
        :row-key="row => row.id"
        :pagination="mobilePagination"
        class="sm:h-full"
      />
    </NCard>
  </div>
</template>
