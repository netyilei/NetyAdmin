<script setup lang="tsx">
import { NButton, NImage, NPopconfirm, NTag } from 'naive-ui';
import dayjs from 'dayjs';
import {
  fetchDeleteArticle,
  fetchGetArticleList,
  fetchPublishArticle,
  fetchUnpublishArticle
} from '@/service/api/v1/content';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import type { Content } from '@/typings/api/v1/content';
import { $t } from '@/locales';
import ArticleOperateModal from './components/article-operate-modal.vue';
import ArticleSearch from './components/article-search.vue';

const appStore = useAppStore();

const publishStatusRecord: Record<Content.PublishStatus, { label: string; type: NaiveUI.ThemeColor }> = {
  draft: { label: $t('page.content.article.statusDraft'), type: 'default' },
  published: { label: $t('page.content.article.statusPublished'), type: 'success' },
  scheduled: { label: $t('page.content.article.statusScheduled'), type: 'warning' }
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
  resetSearchParams
} = useTable({
  apiFn: fetchGetArticleList,
  showTotal: true,
  apiParams: {
    current: 1,
    size: 20,
    categoryId: undefined,
    title: undefined,
    author: undefined,
    publishStatus: undefined,
    isTop: undefined,
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
      key: 'coverImage',
      title: $t('page.content.article.cover'),
      align: 'center',
      width: 100,
      render: row => {
        if (!row.coverImage) return <span class="text-gray">-</span>;
        return <NImage width={60} height={40} src={row.coverImage} objectFit="cover" previewDisabled={false} />;
      }
    },
    {
      key: 'title',
      title: $t('page.content.article.titleField'),
      align: 'left',
      minWidth: 200,
      render: row => {
        const style = row.titleColor ? { color: row.titleColor } : {};
        return (
          <span style={style}>
            {row.isTop && (
              <NTag type="error" size="small" class="mr-4px">
                {$t('page.content.article.isTop')}
              </NTag>
            )}
            {row.title}
          </span>
        );
      }
    },
    {
      key: 'categoryName',
      title: $t('page.content.article.categoryId'),
      align: 'center',
      width: 100,
      render: row => row.categoryName || row.category?.name || '-'
    },
    {
      key: 'author',
      title: $t('page.content.article.author'),
      align: 'center',
      width: 80
    },
    {
      key: 'publishStatus',
      title: $t('page.content.article.status'),
      align: 'center',
      width: 100,
      render: row => {
        const status = publishStatusRecord[row.publishStatus];
        return <NTag type={status.type}>{status.label}</NTag>;
      }
    },
    {
      key: 'viewCount',
      title: $t('page.content.article.viewCount'),
      align: 'center',
      width: 80
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
          <NButton type="primary" ghost size="small" onClick={() => edit(row.id)}>
            {$t('common.edit')}
          </NButton>
          {row.publishStatus === 'published' ? (
            <NButton type="warning" ghost size="small" onClick={() => handleUnpublish(row.id)}>
              {$t('page.content.article.unpublish')}
            </NButton>
          ) : (
            <NButton type="success" ghost size="small" onClick={() => handlePublish(row.id)}>
              {$t('page.content.article.publish')}
            </NButton>
          )}
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
  const { error } = await fetchDeleteArticle(id);
  if (!error) {
    await onDeleted();
  }
}

async function handlePublish(id: number) {
  const { error } = await fetchPublishArticle(id);
  if (!error) {
    window.$message?.success($t('common.updateSuccess'));
    getData();
  }
}

async function handleUnpublish(id: number) {
  const { error } = await fetchUnpublishArticle(id);
  if (!error) {
    window.$message?.success($t('common.updateSuccess'));
    getData();
  }
}

function edit(id: number) {
  handleEdit(id);
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <ArticleSearch v-model:model="searchParams" @reset="resetSearchParams" @search="getDataByPage" />
    <NCard
      :title="$t('page.content.article.title')"
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
        :scroll-x="1200"
        :loading="loading"
        :row-key="row => row.id"
        :pagination="mobilePagination"
        class="sm:h-full"
      />
    </NCard>
    <ArticleOperateModal
      v-model:visible="drawerVisible"
      :operate-type="operateType"
      :row-data="editingData"
      @submitted="getDataByPage"
    />
  </div>
</template>

<style scoped></style>
