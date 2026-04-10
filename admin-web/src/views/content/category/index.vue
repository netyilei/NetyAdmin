<script setup lang="tsx">
import { NButton, NCard, NDataTable, NPopconfirm, NTag } from 'naive-ui';
import dayjs from 'dayjs';
import { Icon } from '@iconify/vue';
import { fetchDeleteCategory, fetchGetCategoryTree } from '@/service/api/v1/content';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useDict } from '@/hooks/common/dict';
import type { Content } from '@/typings/api/v1/content';
import { $t } from '@/locales';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';
import CategoryOperateModal from './components/category-operate-modal.vue';
import CategorySearch from './components/category-search.vue';

const appStore = useAppStore();
const { loadDicts, renderDictTag } = useDict();
loadDicts(['sys_status', 'menu_icon_type']);

const contentTypeRecord: Record<Content.ContentType, string> = {
  plaintext: $t('page.content.category.contentTypePlain'),
  richtext: $t('page.content.category.contentTypeRich')
};

const { columns, columnChecks, data, getData, loading, searchParams, resetSearchParams } = useTable({
  apiFn: async (params: any) => {
    const response = await fetchGetCategoryTree(params?.refresh || false);
    const treeData = response.data || [];
    return {
      ...response,
      data: {
        records: treeData,
        total: treeData.length,
        current: 1,
        size: treeData.length
      }
    } as any;
  },
  showTotal: false,
  immediate: true,
  columns: () =>
    [
      {
        key: 'name',
        title: $t('page.content.category.categoryName'),
        align: 'left',
        minWidth: 180
      },
      {
        key: 'code',
        title: $t('page.content.category.categoryCode'),
        align: 'center',
        width: 120
      },
      {
        key: 'icon',
        title: $t('page.content.category.icon'),
        align: 'center',
        width: 80,
        render: (row: any) => {
          if (!row.icon) return <span class="text-gray">-</span>;
          return <Icon icon={row.icon} width="24" height="24" />;
        }
      },
      {
        key: 'contentType',
        title: $t('page.content.category.contentType'),
        align: 'center',
        width: 100,
        render: (row: any) => {
          const label = contentTypeRecord[row.contentType as Content.ContentType] || row.contentType;
          return <NTag size="small">{label}</NTag>;
        }
      },
      {
        key: 'status',
        title: $t('page.content.category.status'),
        align: 'center',
        width: 100,
        render: (row: any) => renderDictTag('sys_status', row.status ?? '')
      },
      {
        key: 'sort',
        title: $t('page.content.category.sort'),
        align: 'center',
        width: 80
      },
      {
        key: 'remark',
        title: $t('page.content.category.remark'),
        align: 'center',
        minWidth: 150,
        ellipsis: {
          tooltip: true
        }
      },
      {
        key: 'createdAt',
        title: $t('page.content.category.createdAt'),
        align: 'center',
        width: 170,
        render: (row: any) => dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss')
      },
      {
        key: 'operate',
        title: $t('common.operate'),
        align: 'center',
        width: 130,
        fixed: 'right',
        render: (row: any) => (
          <div class="flex-center gap-8px">
            <NButton type="primary" ghost size="small" onClick={() => handleEditTree(row.id)}>
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
    ] as any[]
});

function findNodeById(nodes: Content.CategoryTree[], id: number): Content.CategoryTree | null {
  for (const node of nodes) {
    if (node.id === id) return node;
    if (node.children?.length) {
      const found = findNodeById(node.children, id);
      if (found) return found;
    }
  }
  return null;
}

const { drawerVisible, operateType, handleAdd, handleEdit, editingData, checkedRowKeys, onDeleted } = useTableOperate(
  data as any,
  getData
);

function handleEditTree(id: number) {
  operateType.value = 'edit';
  const node = findNodeById(data.value as any[], id);
  if (node) {
    editingData.value = JSON.parse(JSON.stringify(node));
    drawerVisible.value = true;
  }
}

async function handleRefresh() {
  await getData();
}

async function handleDelete(id: number) {
  const { error } = await fetchDeleteCategory(id);
  if (!error) {
    await onDeleted();
  }
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <CategorySearch v-model:model="searchParams" @reset="resetSearchParams" @search="getData" />
    <NCard
      :title="$t('page.content.category.indexTitle')"
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
          @refresh="handleRefresh"
        />
      </template>
      <NDataTable
        v-model:checked-row-keys="checkedRowKeys"
        :columns="(columns as any)"
        :data="data"
        size="small"
        :flex-height="!appStore.isMobile"
        :scroll-x="962"
        :loading="loading"
        :row-key="row => row.id"
        class="sm:h-full"
      />
    </NCard>
    <CategoryOperateModal
      v-model:visible="drawerVisible"
      :operate-type="operateType"
      :row-data="editingData as any"
      :all-categories="data as any"
      @submitted="getData"
    />
  </div>
</template>

<style scoped></style>
