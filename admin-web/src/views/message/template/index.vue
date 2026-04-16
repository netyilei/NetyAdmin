<script setup lang="tsx">
import { NButton, NPopconfirm, NSpace, NTag } from 'naive-ui';
import { deleteTemplate, fetchTemplateList } from '@/service/api/v1/message-hub';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { $t } from '@/locales';
import { useDict } from '@/hooks/common/dict';
import MsgTemplateOperateModal from './components/msg-template-operate-modal.vue';

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
  apiFn: fetchTemplateList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 10,
    channel: '',
    code: '',
    name: '',
    status: undefined,
    total: 0
  },
  columns: () => [
    { key: 'index', title: $t('common.index'), align: 'center', width: 64 },
    {
      key: 'code',
      title: $t('page.messageHub.template.code'),
      align: 'center',
      minWidth: 120
    } as any,
    {
      key: 'name',
      title: $t('page.messageHub.template.name'),
      align: 'center',
      minWidth: 150
    } as any,
    {
      key: 'channel',
      title: $t('page.messageHub.template.channel'),
      align: 'center',
      width: 100,
      render: (row: any) => renderDictTag('sys_msg_channel', row.channel)
    } as any,
    {
      key: 'status',
      title: $t('page.messageHub.template.status'),
      align: 'center',
      width: 100,
      render: (row: any) => renderDictTag('sys_status', String(row.status))
    } as any,
    {
      key: 'createdAt',
      title: $t('page.messageHub.template.time'),
      align: 'center',
      width: 160
    } as any,
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 180,
      render: (row: any) => (
        <NSpace justify="center">
          <NButton type="primary" ghost size="small" onClick={() => handleEdit(row.id)}>
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

const {
  drawerVisible,
  operateType,
  editingData,
  handleAdd,
  handleEdit,
  onDeleted
} = useTableOperate(data, getData);

async function handleDelete(id: number) {
  const { error } = await deleteTemplate(id);
  if (!error) {
    window.$message?.success($t('common.deleteSuccess'));
    onDeleted();
  }
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <NCard :title="$t('page.messageHub.template.title')" :bordered="false" size="small" class="sm:flex-1-hidden card-wrapper">
      <template #header-extra>
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :loading="loading"
          @add="handleAdd"
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
      <MsgTemplateOperateModal
        v-model:visible="drawerVisible"
        :operate-type="operateType"
        :row-data="editingData"
        @submitted="getData"
      />
    </NCard>
  </div>
</template>

<style scoped></style>
