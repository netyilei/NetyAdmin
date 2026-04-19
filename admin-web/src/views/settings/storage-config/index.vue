<script setup lang="tsx">
import { computed, ref } from 'vue';
import { NButton, NPopconfirm, NTag } from 'naive-ui';
import {
  fetchDeleteStorageConfig,
  fetchGetStorageConfigList,
  fetchSetDefaultStorageConfig
} from '@/service/api/v1/storage';
import { useAppStore } from '@/store/modules/app';
import { useDict } from '@/hooks/common/dict';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useAuth } from '@/hooks/business/auth';
import type { Storage } from '@/typings/api/v1/storage';
import { $t } from '@/locales';
import StorageConfigOperateModal from './components/storage-config-operate-modal.vue';
import StorageTestUploadModal from './components/storage-test-upload-modal.vue';

const appStore = useAppStore();
const { hasAuth } = useAuth();
const { loadDicts, renderDictTag } = useDict();
loadDicts(['sys_status']);

const storageProviderRecord: Record<Storage.StorageProvider, string> = {
  aliyun: $t('page.settings.storageConfig.provider.aliyun'),
  tencent: $t('page.settings.storageConfig.provider.tencent'),
  huawei: $t('page.settings.storageConfig.provider.huawei'),
  qiniu: $t('page.settings.storageConfig.provider.qiniu'),
  minio: $t('page.settings.storageConfig.provider.minio'),
  aws: $t('page.settings.storageConfig.provider.aws'),
  cloudflare: $t('page.settings.storageConfig.provider.cloudflare'),
  custom: $t('page.settings.storageConfig.provider.custom')
};

const { columns, data, getData, loading, mobilePagination } = useTable({
  apiFn: fetchGetStorageConfigList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 20
  },
  columns: () => [
    {
      key: 'index',
      title: $t('common.index'),
      align: 'center',
      width: 64
    },
    {
      key: 'name',
      title: $t('page.manage.storage.configName'),
      align: 'center',
      minWidth: 120
    },
    {
      key: 'provider',
      title: $t('page.manage.storage.provider'),
      align: 'center',
      width: 140,
      render: row => {
        const label = storageProviderRecord[row.provider] || row.provider;
        return <span>{label}</span>;
      }
    },
    {
      key: 'bucket',
      title: $t('page.manage.storage.bucket'),
      align: 'center',
      minWidth: 120
    },
    {
      key: 'domain',
      title: $t('page.manage.storage.domain'),
      align: 'center',
      minWidth: 150,
      render: row => {
        if (!row.domain) return <span class="text-gray">-</span>;
        return <span>{row.domain}</span>;
      }
    },
    {
      key: 'isDefault',
      title: $t('page.manage.storage.isDefault'),
      align: 'center',
      width: 100,
      render: row => {
        if (row.isDefault) {
          return <NTag type="success">{$t('common.yes')}</NTag>;
        }
        return <NTag type="default">{$t('common.no')}</NTag>;
      }
    },
    {
      key: 'status',
      title: $t('page.manage.storage.status'),
      align: 'center',
      width: 100,
      render: row => renderDictTag('sys_status', row.status ?? '')
    },
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 200,
      render: row => (
        <div class="flex-center gap-8px">
          {hasAuth('storage:edit') && (
            <NButton type="primary" ghost size="small" onClick={() => edit(row.id)}>
              {$t('common.edit')}
            </NButton>
          )}
          {hasAuth('storage:default') && !row.isDefault && (
            <NButton type="info" ghost size="small" onClick={() => handleSetDefault(row.id)}>
              {$t('page.manage.storage.setDefault')}
            </NButton>
          )}
          {hasAuth('storage:delete') && (
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

const { drawerVisible, operateType, editingData, handleAdd, handleEdit, onDeleted } = useTableOperate(data, getData);

async function handleDelete(id: number) {
  loading.value = true;
  const { error } = await fetchDeleteStorageConfig(id);
  loading.value = false;
  if (!error) {
    await onDeleted();
  }
}

async function handleSetDefault(id: number) {
  loading.value = true;
  const { error } = await fetchSetDefaultStorageConfig(id);
  loading.value = false;
  if (!error) {
    await getData();
  }
}

function edit(id: number) {
  handleEdit(id);
}

const testModalVisible = ref(false);

const configOptions = computed(() => {
  return data.value.map(item => ({
    label: item.name,
    value: item.id
  }));
});
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <NCard
      :title="$t('page.manage.storage.configTitle')"
      :bordered="false"
      size="small"
      class="card-wrapper sm:flex-1-hidden"
    >
      <template #header-extra>
        <div class="flex-center gap-8px">
          <NButton v-if="hasAuth('storage:add')" type="primary" size="small" @click="handleAdd">
            <template #icon>
              <SvgIcon icon="mdi:plus" />
            </template>
            {{ $t('common.add') }}
          </NButton>
          <NButton v-if="hasAuth('storage:test')" size="small" @click="testModalVisible = true">
            <template #icon>
              <SvgIcon icon="mdi:test-tube" />
            </template>
            {{ $t('page.settings.storageConfig.testUpload') }}
          </NButton>
          <NButton size="small" :loading="loading" @click="getData">
            <template #icon>
              <SvgIcon icon="mdi:refresh" />
            </template>
            {{ $t('common.refresh') }}
          </NButton>
        </div>
      </template>
      <NDataTable
        :columns="columns"
        :data="data"
        size="small"
        :flex-height="!appStore.isMobile"
        :scroll-x="1000"
        :loading="loading"
        remote
        :row-key="row => row.id"
        :pagination="mobilePagination"
        class="sm:h-full"
      />
      <StorageConfigOperateModal
        v-model:visible="drawerVisible"
        :operate-type="operateType"
        :row-data="editingData"
        @submitted="getData"
      />
      <StorageTestUploadModal v-model:visible="testModalVisible" :config-options="configOptions" />
    </NCard>
  </div>
</template>

<style scoped></style>
