<script setup lang="tsx">
import { useRouter } from 'vue-router';
import { NButton, NPopconfirm, NTag } from 'naive-ui';
import dayjs from 'dayjs';
import { fetchDeleteBannerGroup, fetchGetBannerGroupList } from '@/service/api/v1/content';
import { useAppStore } from '@/store/modules/app';
import { useDict } from '@/hooks/common/dict';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { $t } from '@/locales';
import BannerGroupOperateModal from './components/banner-group-operate-modal.vue';
import BannerGroupSearch from './components/banner-group-search.vue';

const appStore = useAppStore();
const router = useRouter();
const { loadDicts, renderDictTag } = useDict();

loadDicts(['sys_status']);

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
  apiFn: fetchGetBannerGroupList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 20,
    name: undefined,
    code: undefined,
    status: undefined
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
      key: 'name',
      title: $t('page.content.bannerGroup.groupName'),
      align: 'left',
      minWidth: 150
    },
    {
      key: 'code',
      title: $t('page.content.bannerGroup.groupCode'),
      align: 'center',
      width: 120
    },
    {
      key: 'position',
      title: $t('page.content.bannerGroup.position'),
      align: 'center',
      width: 100
    },
    {
      key: 'width',
      title: $t('page.content.bannerGroup.size'),
      align: 'center',
      width: 120,
      render: row => {
        if (row.width && row.height) {
          return (
            <span>
              {row.width} x {row.height}
            </span>
          );
        }
        return <span class="text-gray">-</span>;
      }
    },
    {
      key: 'maxItems',
      title: $t('page.content.bannerGroup.maxItems'),
      align: 'center',
      width: 80
    },
    {
      key: 'autoPlay',
      title: $t('page.content.bannerGroup.autoPlay'),
      align: 'center',
      width: 80,
      render: row => {
        return row.autoPlay ? (
          <NTag type="success">{$t('common.yes')}</NTag>
        ) : (
          <NTag type="default">{$t('common.no')}</NTag>
        );
      }
    },
    {
      key: 'interval',
      title: $t('page.content.bannerGroup.interval'),
      align: 'center',
      width: 80
    },
    {
      key: 'status',
      title: $t('common.status'),
      align: 'center',
      width: 80,
      render: row => renderDictTag('sys_status', row.status ?? '')
    },
    {
      key: 'createdAt',
      title: $t('common.createdAt'),
      align: 'center',
      width: 170,
      render: row => (row.createdAt ? dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss') : '-')
    },
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 240,
      render: row => (
        <div class="flex-center gap-8px">
          <NButton type="info" ghost size="small" onClick={() => handleManageBanners(row.id)}>
            {$t('page.content.bannerGroup.manageBanners')}
          </NButton>
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

const { drawerVisible, operateType, handleAdd, handleEdit, editingData, checkedRowKeys, onDeleted } = useTableOperate(
  data,
  getData
);

async function handleDelete(id: number) {
  const { error } = await fetchDeleteBannerGroup(id);
  if (!error) {
    await onDeleted();
  }
}

function edit(id: number) {
  handleEdit(id);
}

function handleManageBanners(groupId: number) {
  router.push(`/content/banner?groupId=${groupId}`);
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <BannerGroupSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />
    <NCard
      :title="$t('page.content.bannerGroup.listTitle')"
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
        :columns="columns"
        :data="data"
        size="small"
        :flex-height="!appStore.isMobile"
        :scroll-x="1100"
        :loading="loading"
        :row-key="row => row.id"
        :pagination="mobilePagination"
        class="sm:h-full"
      />
    </NCard>
    <BannerGroupOperateModal
      v-model:visible="drawerVisible"
      :operate-type="operateType"
      :row-data="editingData"
      @submitted="getDataByPage"
    />
  </div>
</template>

<style scoped></style>
