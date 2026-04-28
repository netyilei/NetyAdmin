<script setup lang="tsx">
import { NButton, NPopconfirm } from 'naive-ui';
import { fetchBatchDeleteAdmin, fetchDeleteAdmin, fetchGetAdminList } from '@/service/api/v1/system-manage';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useDict } from '@/hooks/common/dict';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';
import AdminOperateModal from './components/admin-operate-modal.vue';
import AdminSearch from './components/admin-search.vue';

const appStore = useAppStore();
const { loadDicts, renderDictTag } = useDict();

loadDicts(['sys_status', 'sys_gender']);

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
  apiFn: fetchGetAdminList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 20,
    status: null,
    username: null,
    gender: null,
    nickname: null,
    phone: null,
    email: null
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
      key: 'userName',
      title: $t('page.manage.admin.userName'),
      align: 'center',
      minWidth: 100
    },
    {
      key: 'userGender',
      title: $t('page.manage.admin.userGender'),
      align: 'center',
      width: 100,
      render: row => renderDictTag('sys_gender', row.userGender ?? '')
    },
    {
      key: 'nickName',
      title: $t('page.manage.admin.nickName'),
      align: 'center',
      minWidth: 100
    },
    {
      key: 'userPhone',
      title: $t('page.manage.admin.userPhone'),
      align: 'center',
      width: 120
    },
    {
      key: 'userEmail',
      title: $t('page.manage.admin.userEmail'),
      align: 'center',
      minWidth: 200
    },
    {
      key: 'status',
      title: $t('page.manage.admin.userStatus'),
      align: 'center',
      width: 100,
      render: row => renderDictTag('sys_status', row.status ?? '')
    },
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 130,
      render: row => (
        <div class="flex-center gap-8px">
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
        </div>
      )
    }
  ]
});

const { drawerVisible, operateType, editingData, handleAdd, handleEdit, checkedRowKeys, onBatchDeleted, onDeleted } =
  useTableOperate(data, getData);

async function handleBatchDelete() {
  const ids = checkedRowKeys.value as unknown as number[];
  if (!ids.length) return;

  loading.value = true;
  const { error } = await fetchBatchDeleteAdmin(ids);
  loading.value = false;
  if (!error) {
    window.$message?.success?.($t('common.deleteSuccess'));
    await onBatchDeleted();
  }
}

async function handleDelete(id: number) {
  loading.value = true;
  const { error } = await fetchDeleteAdmin(id);
  loading.value = false;
  if (!error) {
    window.$message?.success?.($t('common.deleteSuccess'));
    await onDeleted();
  }
}

async function handleSubmit(_?: SystemManage.Admin) {
  await getData();
}

function edit(id: number) {
  handleEdit(id);
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <AdminSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />
    <NCard :title="$t('page.manage.admin.title')" :bordered="false" size="small" class="card-wrapper sm:flex-1-hidden">
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
        size="small"
        :flex-height="!appStore.isMobile"
        :scroll-x="962"
        :loading="loading"
        remote
        :row-key="row => row.id"
        :pagination="mobilePagination"
        class="sm:h-full"
      />
      <AdminOperateModal
        v-model:visible="drawerVisible"
        :operate-type="operateType"
        :row-data="editingData"
        @submitted="handleSubmit"
      />
    </NCard>
  </div>
</template>

<style scoped></style>
