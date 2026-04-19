<script setup lang="tsx">
import { ref } from 'vue';
import { NButton, NPopconfirm, NSpace } from 'naive-ui';
import dayjs from 'dayjs';
import { useBoolean } from '@na/hooks';
import { deleteTemplate, fetchTemplateList } from '@/service/api/v1/message-hub';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useAuth } from '@/hooks/business/auth';
import { useDict } from '@/hooks/common/dict';
import { $t } from '@/locales';
import MsgTemplateOperateModal from './components/msg-template-operate-modal.vue';
import MsgTemplateSearch from './components/msg-template-search.vue';
import MsgTestSendModal from './components/msg-test-send-modal.vue';

const appStore = useAppStore();
const { hasAuth } = useAuth();
const { renderDictTag } = useDict();

const { bool: testVisible, setTrue: openTestModal } = useBoolean();
const testTemplate = ref<any>(null);

let handleEditFn: (id: any) => void;

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
    size: 20,
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
          {hasAuth('msg:template:test') && (
            <NButton type="primary" ghost size="small" onClick={() => handleTest(row)}>
              {$t('page.messageHub.template.test')}
            </NButton>
          )}
          {hasAuth('msg:template:edit') && (
            <NButton type="primary" ghost size="small" onClick={() => handleEditFn(row.id)}>
              {$t('common.edit')}
            </NButton>
          )}
          {hasAuth('msg:template:delete') && (
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
        </NSpace>
      )
    } as any
  ]
});

const { drawerVisible, operateType, editingData, handleAdd, handleEdit, onDeleted } = useTableOperate(data, getData);
handleEditFn = handleEdit;

async function handleDelete(id: number) {
  loading.value = true;
  const { error } = await deleteTemplate(id);
  loading.value = false;
  if (!error) {
    window.$message?.success($t('common.deleteSuccess'));
    onDeleted();
  }
}

function handleTest(row: any) {
  testTemplate.value = row;
  openTestModal();
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <MsgTemplateSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />
    <NCard
      :title="$t('page.messageHub.template.title')"
      :bordered="false"
      size="small"
      class="card-wrapper sm:flex-1-hidden"
    >
      <template #header-extra>
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :show-add="hasAuth('msg:template:add')"
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
      <MsgTestSendModal v-model:visible="testVisible" :template="testTemplate" />
    </NCard>
  </div>
</template>

<style scoped></style>
