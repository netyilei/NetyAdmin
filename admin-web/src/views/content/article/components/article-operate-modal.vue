<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import type { UploadFileInfo } from 'naive-ui';
import dayjs from 'dayjs';
import {
  fetchCreateArticle,
  fetchGetArticle,
  fetchGetCategoryTree,
  fetchUpdateArticle
} from '@/service/api/v1/content';
import { fetchCreateUploadRecord, fetchGetUploadCredentials } from '@/service/api/v1/storage';
import { useFormRules } from '@/hooks/common/form';
import { uploadWithPresignedUrl } from '@/utils/upload';
import type { Content } from '@/typings/api/v1/content';
import { $t } from '@/locales';
import ToastUiEditor from '@/components/custom/toast-ui-editor.vue';

defineOptions({
  name: 'ArticleOperateModal'
});

interface Props {
  visible: boolean;
  operateType: 'add' | 'edit';
  rowData?: Content.Article | null;
}

interface Emits {
  (e: 'update:visible', visible: boolean): void;
  (e: 'submitted'): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const { defaultRequiredRule } = useFormRules();

const drawerVisible = computed({
  get() {
    return props.visible;
  },
  set(visible) {
    emit('update:visible', visible);
  }
});

const title = computed(() => {
  return props.operateType === 'add' ? $t('page.content.article.addArticle') : $t('page.content.article.editArticle');
});

type Model = Pick<
  Content.CreateArticleParams,
  | 'categoryId'
  | 'title'
  | 'titleColor'
  | 'coverImage'
  | 'summary'
  | 'content'
  | 'contentType'
  | 'author'
  | 'source'
  | 'keywords'
  | 'tags'
  | 'isTop'
  | 'topSort'
  | 'isHot'
  | 'isRecommend'
  | 'allowComment'
  | 'publishStatus'
  | 'scheduledAt'
>;

const model = ref<Model>({
  categoryId: 0,
  title: '',
  titleColor: '#333333',
  coverImage: '',
  summary: '',
  content: '',
  contentType: 'richtext',
  author: '',
  source: '',
  keywords: '',
  tags: '',
  isTop: false,
  topSort: 0,
  isHot: false,
  isRecommend: false,
  allowComment: true,
  publishStatus: 'draft',
  scheduledAt: null
});

type RuleKey = Extract<keyof Model, 'categoryId' | 'title'>;

const rules: Record<RuleKey, App.Global.FormRule> = {
  categoryId: defaultRequiredRule,
  title: defaultRequiredRule
};

const loading = ref(false);
const categoryOptions = ref<
  { label: string; value: number; contentType: Content.ContentType; storageConfigId: number | null }[]
>([]);
const coverUploading = ref(false);

async function loadCategories() {
  const { data, error } = await fetchGetCategoryTree();
  if (!error && data) {
    const flattenCategories = (
      categories: Content.CategoryTree[],
      level = 0
    ): { label: string; value: number; contentType: Content.ContentType; storageConfigId: number | null }[] => {
      const result: {
        label: string;
        value: number;
        contentType: Content.ContentType;
        storageConfigId: number | null;
      }[] = [];
      for (const cat of categories) {
        const prefix = '　'.repeat(level);
        result.push({
          label: prefix + cat.name,
          value: cat.id,
          contentType: cat.contentType,
          storageConfigId: cat.storageConfigId
        });
        if (cat.children && cat.children.length > 0) {
          result.push(...flattenCategories(cat.children, level + 1));
        }
      }
      return result;
    };
    categoryOptions.value = flattenCategories(data);
  }
}

const currentStorageConfigId = computed(() => {
  const selectedCategory = categoryOptions.value.find(opt => opt.value === model.value.categoryId);
  return selectedCategory?.storageConfigId || undefined;
});

function handleCategoryChange(categoryId: number) {
  const selectedCategory = categoryOptions.value.find(opt => opt.value === categoryId);
  if (selectedCategory) {
    model.value.contentType = selectedCategory.contentType;
  }
}

function initModel() {
  model.value = {
    categoryId: 0,
    title: '',
    titleColor: '#333333',
    coverImage: '',
    summary: '',
    content: '',
    contentType: 'richtext',
    author: '',
    source: '',
    keywords: '',
    tags: '',
    isTop: false,
    topSort: 0,
    isHot: false,
    isRecommend: false,
    allowComment: true,
    publishStatus: 'draft',
    scheduledAt: null
  };
}

const editorReady = ref(false);

async function handleInitModel() {
  initModel();
  editorReady.value = false;

  if (props.operateType === 'add') {
    await nextTick();
    editorReady.value = true;
    return;
  }

  if (props.rowData) {
    const { data, error } = await fetchGetArticle(props.rowData.id);
    if (!error && data) {
      // 1. First, prepare the model data
      const articleData = {
        ...(data as any),
        titleColor: (data as any).titleColor || '#333333'
      };

      // 2. Assign to model
      Object.assign(model.value, articleData);

      // Fix scheduledAt format for NDatePicker (needs timestamp)
      if (model.value.scheduledAt) {
        model.value.scheduledAt = dayjs(model.value.scheduledAt).format('YYYY-MM-DD HH:mm:ss') as any;
      }

      // 3. Wait for DOM and then show editor
      await nextTick();
      editorReady.value = true;
    } else {
      await nextTick();
      editorReady.value = true;
    }
  }
}

async function handleCoverUpload(options: { file: UploadFileInfo }) {
  if (!options.file.file) return;

  coverUploading.value = true;
  try {
    const { data, error } = await fetchGetUploadCredentials({
      configId: currentStorageConfigId.value,
      fileName: options.file.name,
      fileSize: options.file.file.size,
      contentType: options.file.file.type || 'image/jpeg',
      businessType: 'article_cover'
    });

    if (!error && data) {
      const fileUrl = await uploadWithPresignedUrl(data, options.file.file);
      model.value.coverImage = fileUrl;

      await fetchCreateUploadRecord({
        configId: data.configId,
        fileName: options.file.name,
        objectKey: data.objectKey,
        fileSize: options.file.file.size,
        mimeType: options.file.file.type || 'image/jpeg',
        businessType: 'article_cover'
      });

      window.$message?.success($t('common.updateSuccess'));
    }
  } catch {
    window.$message?.error?.($t('common.updateFailed'));
  } finally {
    coverUploading.value = false;
  }
}

function closeModal() {
  drawerVisible.value = false;
}

async function handleSubmit() {
  loading.value = true;

  const params = { ...model.value };

  // Format date if scheduled
  if (params.publishStatus === 'scheduled' && params.scheduledAt) {
    params.scheduledAt = dayjs(params.scheduledAt).toISOString() as any;
  } else {
    params.scheduledAt = null;
  }

  try {
    if (props.operateType === 'add') {
      const { error } = await fetchCreateArticle(params);
      if (!error) {
        window.$message?.success($t('common.addSuccess'));
        closeModal();
        emit('submitted');
      }
    } else {
      const { error } = await fetchUpdateArticle(props.rowData!.id, params);
      if (!error) {
        window.$message?.success($t('common.updateSuccess'));
        closeModal();
        emit('submitted');
      }
    }
  } finally {
    loading.value = false;
  }
}

watch(
  () => props.visible,
  val => {
    if (val) {
      loadCategories();
      handleInitModel();
    }
  },
  { immediate: false }
);
</script>

<template>
  <NModal
    v-model:show="drawerVisible"
    preset="card"
    :title="title"
    :style="{ width: '1400px' }"
    class="overflow-y-auto"
  >
    <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="100">
      <NGrid :cols="24" :x-gap="16">
        <NGridItem :span="12">
          <NFormItem :label="$t('page.content.article.categoryId')" path="categoryId">
            <NSelect
              v-model:value="model.categoryId"
              :placeholder="$t('page.content.article.form.categoryId')"
              :options="categoryOptions"
              @update:value="handleCategoryChange"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem :span="12">
          <NFormItem :label="$t('page.content.article.author')" path="author">
            <NInput v-model:value="model.author" :placeholder="$t('page.content.article.form.author')" maxlength="50" />
          </NFormItem>
        </NGridItem>
        <NGridItem :span="24">
          <NFormItem :label="$t('page.content.article.titleField')" path="title">
            <NInput v-model:value="model.title" :placeholder="$t('page.content.article.form.title')" maxlength="200">
              <template #prefix>
                <NColorPicker
                  v-model:value="model.titleColor"
                  :modes="['hex']"
                  :show-alpha="false"
                  :render-label="() => ''"
                  size="small"
                  class="w-34px"
                />
              </template>
            </NInput>
          </NFormItem>
        </NGridItem>
        <NGridItem :span="12">
          <NFormItem :label="$t('page.content.article.cover')" path="coverImage">
            <NUpload :custom-request="handleCoverUpload as any" :show-file-list="false" accept="image/*">
              <NButton :loading="coverUploading">{{ $t('common.upload') }}</NButton>
            </NUpload>
            <NImage
              v-if="model.coverImage"
              :src="model.coverImage"
              width="100"
              height="60"
              object-fit="cover"
              class="ml-8px"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem :span="12">
          <NFormItem :label="$t('page.content.article.source')" path="source">
            <NInput
              v-model:value="model.source"
              :placeholder="$t('page.content.article.form.source')"
              maxlength="100"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem :span="24">
          <NFormItem :label="$t('page.content.article.summary')" path="summary">
            <NInput
              v-model:value="model.summary"
              :placeholder="$t('page.content.article.form.summary')"
              type="textarea"
              :autosize="{ minRows: 2, maxRows: 4 }"
              maxlength="500"
              show-count
            />
          </NFormItem>
        </NGridItem>
        <NGridItem :span="24">
          <NFormItem label-placement="left">
            <NSpace :size="24">
              <NFormItem :label="$t('page.content.article.isTop')" label-placement="left" :show-feedback="false">
                <NSwitch v-model:value="model.isTop" />
              </NFormItem>
              <NFormItem :label="$t('page.content.article.isHot')" label-placement="left" :show-feedback="false">
                <NSwitch v-model:value="model.isHot" />
              </NFormItem>
              <NFormItem :label="$t('page.content.article.isRecommend')" label-placement="left" :show-feedback="false">
                <NSwitch v-model:value="model.isRecommend" />
              </NFormItem>
              <NFormItem :label="$t('page.content.article.allowComment')" label-placement="left" :show-feedback="false">
                <NSwitch v-model:value="model.allowComment" />
              </NFormItem>
            </NSpace>
          </NFormItem>
        </NGridItem>
        <NGridItem :span="24">
          <NFormItem :label="$t('page.content.article.content')" path="content">
            <div class="w-full">
              <NInput
                v-if="model.contentType === 'plaintext'"
                v-model:value="model.content"
                type="textarea"
                :placeholder="$t('page.content.article.form.content')"
                :autosize="{ minRows: 8, maxRows: 20 }"
              />
              <ToastUiEditor
                v-if="editorReady && model.contentType === 'richtext'"
                v-model="model.content"
                height="600px"
                :placeholder="$t('page.content.article.form.content')"
                :storage-config-id="currentStorageConfigId"
              />
            </div>
          </NFormItem>
        </NGridItem>
        <NGridItem :span="12">
          <NFormItem :label="$t('page.content.article.status')" path="publishStatus">
            <NSelect
              v-model:value="model.publishStatus"
              :placeholder="$t('page.content.article.form.status')"
              :options="[
                { label: $t('page.content.article.statusDraft'), value: 'draft' },
                { label: $t('page.content.article.publish'), value: 'published' },
                { label: $t('page.content.article.statusScheduled'), value: 'scheduled' }
              ]"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem v-if="model.publishStatus === 'scheduled'" :span="12">
          <NFormItem :label="$t('page.content.article.publishedAt')" path="scheduledAt">
            <NDatePicker
              v-model:value="model.scheduledAt"
              type="datetime"
              :placeholder="$t('common.select')"
              class="w-full"
              format="yyyy-MM-dd HH:mm:ss"
              value-format="yyyy-MM-dd HH:mm:ss"
              to="body"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem :span="12">
          <NFormItem :label="$t('page.content.article.keywords')" path="keywords">
            <NInput
              v-model:value="model.keywords"
              :placeholder="$t('page.content.article.form.keywords')"
              maxlength="200"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem :span="12">
          <NFormItem :label="$t('page.content.article.tags')" path="tags">
            <NInput v-model:value="model.tags" :placeholder="$t('page.content.article.form.tags')" maxlength="200" />
          </NFormItem>
        </NGridItem>
      </NGrid>
    </NForm>
    <template #footer>
      <NSpace :size="16">
        <NButton @click="closeModal">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="loading" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
