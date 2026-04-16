<script setup lang="tsx">
import { NButton, NPopconfirm, NSpace, NSwitch, NTag } from 'naive-ui';
import { fetchGetUserList, fetchUpdateUserStatus, fetchDeleteUser } from '@/service/api/v1/system-manage';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
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
  apiFn: fetchGetUserList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 10,
    username: '',
    phone: '',
    email: '',
    status: undefined,
    total: 0
  },
  columns: () => [
    { key: 'index', title: $t('common.index'), align: 'center', width: 64 },
    {
      key: 'username',
      title: $t('page.manage.admin.userName'),
      align: 'center',
      minWidth: 120
    } as any,
    {
      key: 'nickname',
      title: $t('page.manage.admin.nickName'),
      align: 'center',
      minWidth: 120
    } as any,
    {
      key: 'phone',
      title: $t('page.manage.admin.userPhone'),
      align: 'center',
      width: 130
    } as any,
    {
      key: 'gender',
      title: $t('page.manage.admin.userGender'),
      align: 'center',
      width: 80,
      render: (row: any) => renderDictTag('user_gender', row.gender)
    } as any,
    {
      key: 'status',
      title: $t('page.manage.admin.userStatus'),
      align: 'center',
      width: 100,
      render: (row: any) => (
        <NSwitch
          value={row.status === '1'}
          onUpdateValue={(val: boolean) => handleUpdateStatus(row.id, val ? '1' : '0')}
        />
      )
    } as any,
    {
      key: 'lastLoginAt',
      title: '最后登录',
      align: 'center',
      width: 160
    } as any,
    {
      key: 'createdAt',
      title: $t('common.createTime'),
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

async function handleUpdateStatus(id: string, status: string) {
  const { error } = await fetchUpdateUserStatus(id, status);
  if (!error) {
    window.$message?.success($t('common.updateSuccess'));
    getData();
  }
}

async function handleDelete(id: string) {
  const { error } = await fetchDeleteUser(id);
  if (!error) {
    window.$message?.success($t('common.deleteSuccess'));
    getData();
  }
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <NCard :title="$t('route.manage_user')" :bordered="false" size="small" class="sm:flex-1-hidden card-wrapper">
      <template #header-extra>
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :loading="loading"
          @refresh="getData"
        />
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
    </NCard>
  </div>
</template>

<style scoped></style>
