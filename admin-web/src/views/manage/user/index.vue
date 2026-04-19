<script setup lang="tsx">
import { NAvatar, NButton, NPopconfirm, NSpace, NSwitch } from 'naive-ui';
import dayjs from 'dayjs';
import { fetchDeleteUser, fetchGetUserList, fetchUpdateUserStatus } from '@/service/api/v1/system-manage';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useDict } from '@/hooks/common/dict';
import { $t } from '@/locales';
import UserSearch from './components/user-search.vue';
import UserOperateModal from './components/user-operate-modal.vue';

function getAvatarText(row: any): string {
  if (row.userName) return row.userName.charAt(0).toUpperCase();
  if (row.email) return row.email.charAt(0).toUpperCase();
  return '?';
}

const appStore = useAppStore();
const { loadDicts, renderDictTag } = useDict();

loadDicts(['sys_gender', 'sys_status']);

let handleEditFn: (row: any) => void;

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
  apiFn: fetchGetUserList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 20,
    username: '',
    nickname: '',
    phone: '',
    email: '',
    gender: null,
    status: null
  },
  columns: () => [
    { key: 'index', title: $t('common.index'), align: 'center', width: 64 },
    {
      key: 'avatar',
      title: $t('page.manage.user.avatar'),
      align: 'center',
      width: 60,
      render: (row: any) =>
        row.avatar ? <NAvatar size={36} src={row.avatar} /> : <NAvatar size={36}>{getAvatarText(row)}</NAvatar>
    } as any,
    {
      key: 'userName',
      title: $t('page.manage.user.username'),
      align: 'center',
      minWidth: 120
    } as any,
    {
      key: 'nickName',
      title: $t('page.manage.user.nickname'),
      align: 'center',
      minWidth: 120
    } as any,
    {
      key: 'gender',
      title: $t('page.manage.user.gender'),
      align: 'center',
      width: 80,
      render: (row: any) => renderDictTag('sys_gender', row.gender)
    } as any,
    {
      key: 'phone',
      title: $t('page.manage.user.phone'),
      align: 'center',
      width: 120
    } as any,
    {
      key: 'email',
      title: $t('page.manage.user.email'),
      align: 'center',
      minWidth: 150
    } as any,
    {
      key: 'status',
      title: $t('page.manage.user.status'),
      align: 'center',
      width: 100,
      render: (row: any) => (
        <NSwitch
          value={row.status === '1'}
          loading={loading.value}
          onUpdateValue={(val: boolean) => handleStatusChange(row.id, val ? '1' : '0')}
        >
          {{ checked: () => $t('common.enable'), unchecked: () => $t('common.disable') }}
        </NSwitch>
      )
    } as any,
    {
      key: 'createdAt',
      title: $t('common.createdAt'),
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
          <NButton type="primary" ghost size="small" onClick={() => handleEditFn(row)}>
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

const { drawerVisible, operateType, editingData, handleAdd, handleEdit, onDeleted } = useTableOperate(data, getData);
handleEditFn = handleEdit;

async function handleStatusChange(id: string, status: string) {
  const { error } = await fetchUpdateUserStatus(id, status);
  if (!error) {
    window.$message?.success($t('common.updateSuccess'));
    await getData();
  }
}

async function handleDelete(id: string) {
  const { error } = await fetchDeleteUser(id);
  if (!error) {
    window.$message?.success($t('common.deleteSuccess'));
    onDeleted();
  }
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <UserSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />
    <NCard :title="$t('page.manage.user.title')" :bordered="false" size="small" class="card-wrapper sm:flex-1-hidden">
      <template #header-extra>
        <TableHeaderOperation v-model:columns="columnChecks" :loading="loading" @add="handleAdd" @refresh="getData" />
      </template>
      <NDataTable
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
      <UserOperateModal
        v-model:visible="drawerVisible"
        :operate-type="operateType"
        :row-data="editingData"
        @submitted="getData"
      />
    </NCard>
  </div>
</template>
