<script setup lang="tsx">
import { NButton, NPopconfirm, NSpace, NTag } from 'naive-ui';
import { batchDeleteIPAC, deleteIPAC, fetchIPACList } from '@/service/api/v1/system-ipac';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { $t } from '@/locales';
import { useDict } from '@/hooks/common/dict';
import type { SystemIPAC } from '@/typings/api/v1/system-ipac';
import IPACSearch from './components/ipac-search.vue';
import IPACOperateModal from './components/ipac-operate-modal.vue';

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
  apiFn: fetchIPACList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 10,
    appId: undefined,
    ipAddr: '',
    type: undefined,
    status: undefined,
    total: 0
  },
  columns: () => [
    { type: 'selection', align: 'center', width: 48 },
    { key: 'index', title: $t('common.index'), align: 'center', width: 64 },
    {
      key: 'ipAddr',
      title: $t('page.ops.ipac.ipAddr'),
      align: 'center',
      minWidth: 150
    } as any,
    {
      key: 'type',
      title: $t('page.ops.ipac.type'),
      align: 'center',
      width: 100,
      render: (row: any) => renderDictTag('sys_ip_action_type', String(row.type))
    } as any,
    {
      key: 'appId',
      title: $t('page.ops.ipac.appId'),
      align: 'center',
      width: 120,
      render: (row: any) => (row.appId ? row.appId : <NTag type="info">{$t('page.ops.ipac.global')}</NTag>)
    } as any,
    {
      key: 'reason',
      title: $t('page.ops.ipac.reason'),
      align: 'center',
      minWidth: 150
    } as any,
    {
      key: 'expiredAt',
      title: $t('page.ops.ipac.expiredAt'),
      align: 'center',
      width: 160,
      render: (row: any) => (row.expiredAt ? row.expiredAt : $t('page.ops.ipac.permanent'))
    } as any,
    {
      key: 'status',
      title: $t('page.ops.ipac.status'),
      align: 'center',
      width: 100,
      render: (row: any) => renderDictTag('sys_status', String(row.status))
    } as any,
    {
      key: 'createdAt',
      title: $t('page.ops.ipac.time'),
      align: 'center',
      width: 160
    } as any,
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 130,
      render: (row: any) => (
        <NSpace justify="center">
          <NButton type="primary" ghost size="small" onClick={() => edit(row.id)}>
            {$t('common.edit')}
          </NButton>
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
        </NSpace>
      )
    }
  ]
});

const {
  checkedRowKeys,
  onBatchDeleted,
  onDeleted,
  handleAdd,
  handleEdit,
  drawerVisible,
  operateType,
  editingData
} = useTableOperate(data, getData);

async function handleBatchDelete() {
  const ids = (checkedRowKeys.value as unknown) as number[];
  if (!ids.length) return;

  const { error } = await batchDeleteIPAC(ids);
  if (!error) {
    window.$message?.success?.($t('common.deleteSuccess'));
    onBatchDeleted();
  }
}

async function handleDelete(id: number) {
  const { error } = await deleteIPAC(id);
  if (!error) {
    window.$message?.success?.($t('common.deleteSuccess'));
    onDeleted();
  }
}

function edit(id: number) {
  handleEdit(id);
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <IPACSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />
    <NCard :title="$t('page.ops.ipac.title')" :bordered="false" size="small" class="sm:flex-1-hidden card-wrapper">
      <template #header-extra>
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :disabled-delete="checkedRowKeys.length === 0"
          :loading="loading"
          @add="handleAdd"
          @delete="handleBatchDelete"
          @refresh="getData"
        />
      </template>
      <NDataTable
        v-model:checked-row-keys="checkedRowKeys"
        :columns="columns"
        :data="data"
        :size="appStore.isMobile ? 'small' : 'medium'"
        :flex-height="!appStore.isMobile"
        :scroll-x="962"
        :loading="loading"
        remote
        :row-key="row => row.id"
        :pagination="mobilePagination"
        class="sm:h-full"
      />
      <IPACOperateModal
        v-model:visible="drawerVisible"
        :operate-type="operateType"
        :row-data="editingData as any"
        @submitted="getData"
      />
    </NCard>
  </div>
</template>

<style scoped></style>
