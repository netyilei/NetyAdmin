<script setup lang="tsx">
import { NButton, NPopconfirm, NSpace, NTag } from 'naive-ui';
import dayjs from 'dayjs';
import { deleteApp, fetchAppList, resetAppSecret } from '@/service/api/v1/open-app';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useDict } from '@/hooks/common/dict';
import { $t } from '@/locales';
import AppSearch from './components/app-search.vue';
import AppOperateModal from './components/app-operate-modal.vue';

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
  apiFn: fetchAppList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 10,
    name: '',
    appKey: '',
    status: undefined,
    total: 0
  },
  columns: () => [
    { type: 'selection', align: 'center', width: 48 },
    { key: 'index', title: $t('common.index'), align: 'center', width: 64 },
    {
      key: 'name',
      title: $t('page.openPlatform.app.name'),
      align: 'center',
      minWidth: 120
    } as any,
    {
      key: 'appKey',
      title: $t('page.openPlatform.app.appKey'),
      align: 'center',
      width: 150
    } as any,
    {
      key: 'ipStrategy',
      title: $t('page.openPlatform.app.ipStrategy'),
      align: 'center',
      width: 150,
      render: (row: any) => renderDictTag('sys_app_ip_strategy', String(row.ipStrategy))
    } as any,
    {
      key: 'status',
      title: $t('page.openPlatform.app.status'),
      align: 'center',
      width: 100,
      render: (row: any) => renderDictTag('sys_status', String(row.status))
    } as any,
    {
      key: 'createdAt',
      title: $t('page.openPlatform.app.time'),
      align: 'center',
      width: 160,
      render: (row: any) => (row.createdAt ? dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss') : '-')
    } as any,
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 250,
      render: (row: any) => (
        <NSpace justify="center">
          <NButton type="primary" ghost size="small" onClick={() => edit(row.id)}>
            {$t('common.edit')}
          </NButton>
          <NPopconfirm onPositiveClick={() => handleResetSecret(row.id)}>
            {{
              default: () => $t('page.openPlatform.app.confirmResetSecret'),
              trigger: () => (
                <NButton type="warning" ghost size="small">
                  {$t('page.openPlatform.app.resetSecret')}
                </NButton>
              )
            }}
          </NPopconfirm>
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

const { drawerVisible, operateType, editingData, handleAdd, handleEdit, checkedRowKeys, onBatchDeleted, onDeleted } =
  useTableOperate(data, getData);

async function handleDelete(id: string) {
  const { error } = await deleteApp(id);
  if (!error) {
    onDeleted();
  }
}

async function handleResetSecret(id: string) {
  const { data: res, error } = await resetAppSecret(id);
  if (!error && res) {
    window.$dialog?.success({
      title: $t('page.openPlatform.app.resetSecret'),
      content: () => (
        <NSpace vertical>
          <span>{$t('page.openPlatform.app.resetSecretSuccess')}</span>
          <NTag type="success" size="large">
            {res.appSecret}
          </NTag>
          <span class="text-error">请务必妥善保管，关闭此窗口后将无法再次查看！</span>
        </NSpace>
      )
    });
  }
}

function edit(id: string) {
  handleEdit(id);
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <AppSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />
    <NCard
      :title="$t('page.openPlatform.app.title')"
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
      <AppOperateModal
        v-model:visible="drawerVisible"
        :operate-type="operateType"
        :row-data="editingData"
        @submitted="getData"
      />
    </NCard>
  </div>
</template>

<style scoped></style>
