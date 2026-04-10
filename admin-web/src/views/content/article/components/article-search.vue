<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { fetchGetCategoryTree } from '@/service/api/v1/content';
import type { Content } from '@/typings/api/v1/content';
import { $t } from '@/locales';

defineOptions({
  name: 'ArticleSearch'
});

interface Emits {
  (e: 'reset'): void;
  (e: 'search'): void;
}

const emit = defineEmits<Emits>();

const model = defineModel<Content.ArticleSearchParams>('model', { required: true });

const categoryOptions = ref<{ label: string; value: number }[]>([]);

async function loadCategories() {
  const { data, error } = await fetchGetCategoryTree();
  if (!error && data) {
    const flattenCategories = (categories: Content.CategoryTree[], level = 0): { label: string; value: number }[] => {
      const result: { label: string; value: number }[] = [];
      for (const cat of categories) {
        const prefix = '　'.repeat(level);
        result.push({ label: prefix + cat.name, value: cat.id });
        if (cat.children && cat.children.length > 0) {
          result.push(...flattenCategories(cat.children, level + 1));
        }
      }
      return result;
    };
    categoryOptions.value = flattenCategories(data);
  }
}

function reset() {
  emit('reset');
}

function search() {
  emit('search');
}

onMounted(() => {
  loadCategories();
});
</script>

<template>
  <NCard :bordered="false" size="small" class="card-wrapper">
    <NForm :model="model" label-placement="left" :label-width="80">
      <NGrid responsive="screen" item-responsive>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :label="$t('page.content.article.titleField')" path="title">
            <NInput v-model:value="model.title" :placeholder="$t('page.content.article.form.title')" clearable />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :label="$t('page.content.article.categoryId')" path="categoryId">
            <NSelect
              v-model:value="model.categoryId"
              :placeholder="$t('page.content.article.form.categoryId')"
              clearable
              :options="categoryOptions"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :label="$t('page.content.article.author')" path="author">
            <NInput v-model:value="model.author" :placeholder="$t('page.content.article.form.author')" clearable />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :label="$t('page.content.article.status')" path="publishStatus">
            <NSelect
              v-model:value="model.publishStatus"
              :placeholder="$t('page.content.article.form.status')"
              clearable
              :options="[
                { label: $t('page.content.article.statusDraft'), value: 'draft' },
                { label: $t('page.content.article.statusPublished'), value: 'published' },
                { label: $t('page.content.article.statusScheduled'), value: 'scheduled' }
              ]"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :label="$t('page.content.article.isTop')" path="isTop">
            <NSelect
              v-model:value="model.isTop"
              :placeholder="$t('common.select')"
              clearable
              :options="[
                { label: $t('common.yes'), value: '1' },
                { label: $t('common.no'), value: '0' }
              ]"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :show-label="false">
            <NSpace class="w-full" justify="end">
              <NButton @click="reset">
                {{ $t('common.reset') }}
              </NButton>
              <NButton type="primary" @click="search">
                {{ $t('common.search') }}
              </NButton>
            </NSpace>
          </NFormItem>
        </NGridItem>
      </NGrid>
    </NForm>
  </NCard>
</template>

<style scoped></style>
