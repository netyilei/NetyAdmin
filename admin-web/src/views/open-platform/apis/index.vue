<script setup lang="tsx">
import { NButton, NPopconfirm, NSpace, NTag } from 'naive-ui';
import dayjs from 'dayjs';
import { deleteApi, fetchApiList } from '@/service/api/v1/open-api';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useDict } from '@/hooks/common/dict';
import { $t } from '@/locales';
import ApiSearch from './components/api-search.vue';
import ApiOperateModal from './components/api-operate-modal.vue';

const appStore = useAppStore();
const { renderDictTag } = useDict();

const methodColorMap: Record<string, 'success' | 'info' | 'warning' | 'error' | 'default' | 'primary'> = {
  GET: 'success',
  POST: 'info',
  PUT: 'warning',
  DELETE: 'error',
  PATCH: 'default'
};

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
  apiFn: fetchApiList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 30,
    method: '',
    path: '',
    name: '',
    group: '',
    status: undefined
  },
  columns: () => [
    { type: 'selection', align: 'center', width: 48 },
    { key: 'index', title: $t('common.index'), align: 'center', width: 64 },
    {
      key: 'method',
      title: $t('page.openPlatform.api.method'),
      align: 'center',
      width: 100,
      render: (row: any) => (
        <NTag type={methodColorMap[row.method] || 'default'} size="small">
          {row.method}
        </NTag>
      )
    } as any,
    {
      key: 'path',
      title: $t('page.openPlatform.api.path'),
      align: 'left',
      minWidth: 200
    } as any,
    {
      key: 'name',
      title: $t('page.openPlatform.api.name'),
      align: 'center',
      width: 150
    } as any,
    {
      key: 'group',
      title: $t('page.openPlatform.api.group'),
      align: 'center',
      width: 120
    } as any,
    {
      key: 'status',
      title: $t('page.openPlatform.api.status'),
      align: 'center',
      width: 100,
      render: (row: any) => renderDictTag('sys_status', String(row.status))
    } as any,
    {
      key: 'createdAt',
      title: $t('page.openPlatform.api.time'),
      align: 'center',
      width: 160,
      render: (row: any) => (row.createdAt ? dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss') : '-')
    } as any,
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 150,
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
    } as any
  ]
});

const { drawerVisible, operateType, editingData, handleAdd, handleEdit, checkedRowKeys, onDeleted } = useTableOperate(
  data,
  getData
);

async function handleDelete(id: number) {
  const { error } = await deleteApi(id);
  if (!error) {
    onDeleted();
  }
}

function edit(id: number) {
  handleEdit(id);
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <ApiSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />
    <NCard
      :title="$t('page.openPlatform.api.title')"
      :bordered="false"
      size="small"
      class="card-wrapper sm:flex-1-hidden"
    >
      <template #header-extra>
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :disabled-delete="checkedRowKeys.length === 0"
          :loading="loading"
          @add="handleAdd"
          @refresh="getData"
        />
      </template>
      <NDataTable
        v-model:checked-row-keys="checkedRowKeys"
        remote
        striped
        size="small"
        class="sm:h-full"
        :data="data"
        :columns="columns"
        :flex-height="!appStore.isMobile"
        :loading="loading"
        :single-line="false"
        :row-key="row => row.id"
        :pagination="mobilePagination"
        @update:page="getDataByPage"
      />
      <ApiOperateModal
        v-model:visible="drawerVisible"
        :operate-type="operateType"
        :row-data="editingData"
        @submitted="getData"
      />
    </NCard>
  </div>
</template>

<style scoped></style>
