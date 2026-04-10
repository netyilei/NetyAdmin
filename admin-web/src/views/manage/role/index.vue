<script setup lang="tsx">
import { NButton, NPopconfirm } from 'naive-ui';
import { consola } from 'consola';
import { fetchBatchDeleteRole, fetchDeleteRole, fetchGetRoleList } from '@/service/api/v1/system-manage';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useDict } from '@/hooks/common/dict';
import { isSuperByCode } from '@/utils/common';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';
import RoleOperateDrawer from './components/role-operate-drawer.vue';
import RoleSearch from './components/role-search.vue';

const appStore = useAppStore();
const { loadDicts, renderDictTag } = useDict();

loadDicts(['sys_status']);

const {
  columns,
  columnChecks,
  data,
  loading,
  getData,
  getDataByPage,
  mobilePagination,
  searchParams,
  resetSearchParams
} = useTable({
  apiFn: fetchGetRoleList,
  apiParams: {
    current: 1,
    size: 20,
    // if you want to use the searchParams in Form, you need to define the following properties, and the value is null
    // the value can not be undefined, otherwise the property in Form will not be reactive
    status: null,
    name: null,
    code: null
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
      width: 64,
      align: 'center'
    },
    {
      key: 'name',
      title: $t('page.manage.role.roleName'),
      align: 'center',
      minWidth: 120
    },
    {
      key: 'code',
      title: $t('page.manage.role.roleCode'),
      align: 'center',
      minWidth: 120
    },
    {
      key: 'desc',
      title: $t('page.manage.role.roleDesc'),
      minWidth: 120
    },
    {
      key: 'status',
      title: $t('page.manage.role.roleStatus'),
      align: 'center',
      width: 100,
      render: row => renderDictTag('sys_status', row.status ?? '')
    },
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 130,
      render: row => {
        // 超级管理员不显示操作按钮
        if (isSuperByCode(row.code)) {
          return <div></div>;
        }
        return (
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
        );
      }
    }
  ]
});

const {
  drawerVisible,
  operateType,
  editingData,
  handleAdd,
  handleEdit,
  checkedRowKeys,
  onBatchDeleted,
  onDeleted
  // closeDrawer
} = useTableOperate(data, getData);

async function handleBatchDelete() {
  consola.log(checkedRowKeys.value);

  const ids = checkedRowKeys.value as unknown as number[];
  if (!ids.length) return;

  loading.value = true;
  const { error } = await fetchBatchDeleteRole(ids);
  loading.value = false;
  if (!error) {
    await onBatchDeleted();
  }
}

async function handleDelete(id: number) {
  consola.log(id);

  loading.value = true;
  const result = await fetchDeleteRole(id);
  loading.value = false;
  if (!result.error) {
    await onDeleted();
  }
}

function edit(id: number) {
  handleEdit(id);
}

async function handleSubmit(roleData?: SystemManage.RoleBase) {
  // if (!roleData) return;
  consola.info(roleData);
  await getData();
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <RoleSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />
    <NCard :title="$t('page.manage.role.title')" :bordered="false" size="small" class="card-wrapper sm:flex-1-hidden">
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
        :scroll-x="702"
        :loading="loading"
        remote
        :row-key="row => row.id"
        :pagination="mobilePagination"
        class="sm:h-full"
      />
      <RoleOperateDrawer
        v-model:visible="drawerVisible"
        :operate-type="operateType"
        :row-data="editingData"
        @submitted="handleSubmit"
      />
    </NCard>
  </div>
</template>

<style scoped></style>
