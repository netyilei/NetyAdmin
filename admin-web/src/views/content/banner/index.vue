<script setup lang="tsx">
import { onMounted, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { NButton, NImage, NPopconfirm, NTag } from 'naive-ui';
import { fetchDeleteBannerItem, fetchGetBannerGroup, fetchGetBannerItemList } from '@/service/api/v1/content';
import { useAppStore } from '@/store/modules/app';
import { useTabStore } from '@/store/modules/tab';
import { useDict } from '@/hooks/common/dict';
import { useTable, useTableOperate } from '@/hooks/common/table';
import type { Content } from '@/typings/api/v1/content';
import { $t } from '@/locales';
import BannerOperateModal from './components/banner-operate-modal.vue';
import BannerSearch from './components/banner-search.vue';

const appStore = useAppStore();
const route = useRoute();
const { loadDicts, renderDictTag } = useDict();
loadDicts(['sys_status']);

const tabStore = useTabStore();

const groupId = ref<number | undefined>(undefined);
const groupName = ref<string>('');

const linkTypeRecord: Record<Content.LinkType, string> = {
  none: $t('page.content.bannerItem.linkTypeNone'),
  internal: $t('page.content.bannerItem.linkTypeInternal'),
  external: $t('page.content.bannerItem.linkTypeExternal'),
  article: $t('page.content.bannerItem.linkTypeArticle')
};

const {
  columns,
  columnChecks,
  data,
  getData,
  getDataByPage,
  loading,
  mobilePagination,
  searchParams,
  updateSearchParams,
  resetSearchParams
} = useTable({
  apiFn: fetchGetBannerItemList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 20,
    groupId: route.query.groupId ? Number(route.query.groupId) : undefined,
    title: undefined,
    status: undefined
  },
  immediate: false,
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
      key: 'imageUrl',
      title: $t('page.content.bannerItem.image'),
      align: 'center',
      width: 100,
      render: row => {
        return <NImage width={60} height={40} src={row.imageUrl} objectFit="cover" previewDisabled={false} />;
      }
    },
    {
      key: 'title',
      title: $t('page.content.bannerItem.titleField'),
      align: 'left',
      minWidth: 150
    },
    {
      key: 'subtitle',
      title: $t('page.content.bannerItem.subtitle'),
      align: 'center',
      width: 150
    },
    {
      key: 'linkType',
      title: $t('page.content.bannerItem.linkType'),
      align: 'center',
      width: 100,
      render: row => {
        const label = linkTypeRecord[row.linkType] || row.linkType;
        return <NTag>{label}</NTag>;
      }
    },
    {
      key: 'sort',
      title: $t('page.content.bannerItem.sort'),
      align: 'center',
      width: 80
    },
    {
      key: 'viewCount',
      title: $t('page.content.bannerItem.view'),
      align: 'center',
      width: 60
    },
    {
      key: 'clickCount',
      title: $t('page.content.bannerItem.click'),
      align: 'center',
      width: 60
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
      width: 160
    },
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 160,
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

const { drawerVisible, operateType, handleAdd, handleEdit, editingData, checkedRowKeys, onDeleted } = useTableOperate(
  data,
  getData
);

async function loadGroupInfo(id: number) {
  const { data: groupData } = await fetchGetBannerGroup(id);
  if (groupData) {
    groupName.value = groupData.name;

    // Update tab label for better UX
    tabStore.setTabLabel(`${$t('page.content.bannerItem.bannerManage')} - ${groupData.name}`);

    // Correctly update search params using the hook's recommended method
    updateSearchParams({ groupId: id });

    // Explicitly trigger data fetch
    getData();
  }
}

async function handleDelete(id: number) {
  const { error } = await fetchDeleteBannerItem(id);
  if (!error) {
    await onDeleted();
  }
}

function edit(id: number) {
  handleEdit(id);
}

onMounted(() => {
  const id = route.query.groupId;
  if (id) {
    groupId.value = Number(id);
    loadGroupInfo(groupId.value);
  } else {
    // If no groupId, load all items manually (since immediate is false)
    getData();
  }
});

watch(
  () => route.query.groupId,
  newId => {
    if (newId) {
      groupId.value = Number(newId);
      loadGroupInfo(groupId.value);
    }
  }
);
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <NCard :bordered="false" size="small">
      <BannerSearch
        v-model:model="searchParams"
        :group-id="groupId"
        @reset="resetSearchParams"
        @search="getDataByPage"
      />
    </NCard>
    <NCard :bordered="false" size="small" class="card-wrapper sm:flex-1-hidden">
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
        :scroll-x="1200"
        :loading="loading"
        :row-key="row => row.id"
        :pagination="mobilePagination"
        class="sm:h-full"
      />
    </NCard>
    <BannerOperateModal
      v-model:visible="drawerVisible"
      :operate-type="operateType"
      :row-data="editingData"
      :group-id="groupId"
      @submitted="getDataByPage"
    />
  </div>
</template>

<style scoped></style>
