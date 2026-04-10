<script setup lang="tsx">
import { NButton, NImage, NPopconfirm, NTag } from 'naive-ui';
import dayjs from 'dayjs';
import {
  fetchBatchDeleteUploadRecord,
  fetchDeleteUploadRecord,
  fetchGetUploadRecordList
} from '@/service/api/v1/storage';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useAuth } from '@/hooks/business/auth';
import type { Storage } from '@/typings/api/v1/storage';
import { $t } from '@/locales';
import UploadRecordSearch from './components/upload-record-search.vue';

const appStore = useAppStore();
const { hasAuth } = useAuth();

const uploadSourceRecord: Record<Storage.UploadSource, { label: string; type: NaiveUI.ThemeColor }> = {
  admin: { label: $t('page.manage.upload.sourceAdmin'), type: 'primary' },
  client: { label: $t('page.manage.upload.sourceClient'), type: 'info' },
  api: { label: $t('page.manage.upload.sourceApi'), type: 'warning' },
  system: { label: $t('page.manage.upload.sourceSystem'), type: 'default' }
};

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${Number.parseFloat((bytes / k ** i).toFixed(2))} ${sizes[i]}`;
}

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
  apiFn: fetchGetUploadRecordList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 10,
    fileName: undefined,
    source: undefined,
    businessType: undefined,
    storageConfigId: undefined,
    startTime: undefined,
    endTime: undefined
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
      key: 'fileUrl',
      title: $t('page.manage.upload.preview'),
      align: 'center',
      width: 80,
      render: row => {
        if (!row.fileUrl) return <span class="text-gray">-</span>;
        if (row.mimeType?.startsWith('image/')) {
          return <NImage src={row.fileUrl} width={40} height={40} object-fit="cover" preview-disabled={false} />;
        }
        return <span class="text-gray">-</span>;
      }
    },
    {
      key: 'fileName',
      title: $t('page.manage.upload.fileName'),
      align: 'left',
      minWidth: 150,
      ellipsis: { tooltip: true }
    },
    {
      key: 'fileSize',
      title: $t('page.manage.upload.fileSize'),
      align: 'center',
      width: 100,
      render: row => <span>{formatFileSize(row.fileSize)}</span>
    },
    {
      key: 'source',
      title: $t('page.manage.upload.source'),
      align: 'center',
      width: 100,
      render: row => {
        const item = uploadSourceRecord[row.source];
        if (!item) return <span>{row.source}</span>;
        return <NTag type={item.type}>{item.label}</NTag>;
      }
    },
    {
      key: 'businessType',
      title: $t('page.manage.upload.businessType'),
      align: 'center',
      width: 100,
      render: row => {
        if (!row.businessType) return <span class="text-gray">-</span>;
        return <span>{row.businessType}</span>;
      }
    },
    {
      key: 'storageName',
      title: $t('page.manage.upload.storageName'),
      align: 'center',
      minWidth: 100
    },
    {
      key: 'uploaderIp',
      title: $t('page.manage.upload.uploaderIp'),
      align: 'center',
      width: 130
    },
    {
      key: 'uploadedAt',
      title: $t('page.manage.upload.uploadedAt'),
      align: 'center',
      width: 170,
      render: row => (row.uploadedAt ? dayjs(row.uploadedAt).format('YYYY-MM-DD HH:mm:ss') : '-')
    },
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 150,
      render: row => (
        <div class="flex-center gap-8px">
          {row.fileUrl && (
            <NButton type="primary" ghost size="small" onClick={() => window.open(row.fileUrl, '_blank')}>
              {$t('page.manage.upload.view')}
            </NButton>
          )}
          {hasAuth('ops:upload-record:delete') && (
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
  const { error } = await fetchBatchDeleteUploadRecord(ids);
  loading.value = false;
  if (!error) {
    await onBatchDeleted();
  }
}

async function handleDelete(id: number) {
  loading.value = true;
  const { error } = await fetchDeleteUploadRecord(id);
  loading.value = false;
  if (!error) {
    await onDeleted();
  }
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <UploadRecordSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />
    <NCard :title="$t('page.manage.upload.title')" :bordered="false" size="small" class="card-wrapper sm:flex-1-hidden">
      <template #header-extra>
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :show-add="false"
          :show-delete="hasAuth('ops:upload-record:batch-delete')"
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

<style scoped></style>
