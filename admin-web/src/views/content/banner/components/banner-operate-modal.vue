<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import type { UploadFileInfo } from 'naive-ui';
import dayjs from 'dayjs';
import {
  fetchCreateBannerItem,
  fetchGetAllBannerGroups,
  fetchGetBannerItem,
  fetchUpdateBannerItem
} from '@/service/api/v1/content';
import { fetchCreateUploadRecord, fetchGetUploadCredentials } from '@/service/api/v1/storage';
import { useFormRules } from '@/hooks/common/form';
import { uploadWithPresignedUrl } from '@/utils/upload';
import type { Content } from '@/typings/api/v1/content';
import { $t } from '@/locales';

defineOptions({
  name: 'BannerOperateModal'
});

interface Props {
  visible: boolean;
  operateType: 'add' | 'edit';
  rowData?: Content.Banner | null;
  groupId?: number;
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
  return props.operateType === 'add' ? '新增Banner' : '编辑Banner';
});

type Model = Pick<
  Content.CreateBannerParams,
  | 'groupId'
  | 'title'
  | 'subtitle'
  | 'imageUrl'
  | 'imageAlt'
  | 'linkType'
  | 'linkUrl'
  | 'linkArticleId'
  | 'content'
  | 'customParams'
  | 'sort'
  | 'startTime'
  | 'endTime'
  | 'status'
>;

const model: Model = reactive({
  groupId: 0,
  title: '',
  subtitle: '',
  imageUrl: '',
  imageAlt: '',
  linkType: 'none',
  linkUrl: '',
  linkArticleId: undefined,
  content: '',
  customParams: '',
  sort: 0,
  startTime: null,
  endTime: null,
  status: '1'
});

type RuleKey = Extract<keyof Model, 'groupId' | 'title' | 'imageUrl'>;

const rules: Record<RuleKey, App.Global.FormRule> = {
  groupId: defaultRequiredRule,
  title: defaultRequiredRule,
  imageUrl: defaultRequiredRule
};

const loading = ref(false);
const groupOptions = ref<{ label: string; value: number }[]>([]);
const imageUploading = ref(false);

async function loadGroups() {
  const { data, error } = await fetchGetAllBannerGroups();
  if (!error && data) {
    groupOptions.value = data.map(g => ({ label: g.name, value: g.id }));
  }
}

function initModel() {
  model.groupId = props.groupId || 0;
  model.title = '';
  model.subtitle = '';
  model.imageUrl = '';
  model.imageAlt = '';
  model.linkType = 'none';
  model.linkUrl = '';
  model.linkArticleId = undefined;
  model.content = '';
  model.customParams = '';
  model.sort = 0;
  model.startTime = undefined;
  model.endTime = undefined;
  model.status = '1';
}

async function handleInitModel() {
  initModel();

  if (props.operateType === 'add') {
    return;
  }

  if (props.rowData) {
    const { data, error } = await fetchGetBannerItem(props.rowData.id);
    if (!error && data) {
      Object.assign(model, {
        groupId: data.groupId,
        title: data.title,
        subtitle: data.subtitle,
        imageUrl: data.imageUrl,
        imageAlt: data.imageAlt,
        linkType: data.linkType,
        linkUrl: data.linkUrl,
        linkArticleId: data.linkArticleId,
        content: data.content,
        customParams: data.customParams,
        sort: data.sort,
        startTime: data.startTime ? dayjs(data.startTime).format('YYYY-MM-DD HH:mm:ss') : undefined,
        endTime: data.endTime ? dayjs(data.endTime).format('YYYY-MM-DD HH:mm:ss') : undefined,
        status: data.status
      });
    }
  }
}

async function handleImageUpload(options: { file: UploadFileInfo }) {
  if (!options.file.file) return;

  imageUploading.value = true;
  try {
    const { data, error } = await fetchGetUploadCredentials({
      fileName: options.file.name,
      fileSize: options.file.file.size,
      contentType: options.file.file.type || 'image/jpeg',
      businessType: 'banner_image'
    });

    if (!error && data) {
      const fileUrl = await uploadWithPresignedUrl(data, options.file.file);
      model.imageUrl = fileUrl;

      await fetchCreateUploadRecord({
        configId: data.configId,
        fileName: options.file.name,
        objectKey: data.objectKey,
        fileSize: options.file.file.size,
        mimeType: options.file.file.type || 'image/jpeg',
        businessType: 'banner_image'
      });

      window.$message?.success('图片上传成功');
    }
  } catch {
    window.$message?.error('图片上传失败');
  } finally {
    imageUploading.value = false;
  }
}

function closeModal() {
  drawerVisible.value = false;
}

async function handleSubmit() {
  loading.value = true;

  const params: any = {
    ...model,
    startTime: model.startTime ? dayjs(model.startTime).toISOString() : undefined,
    endTime: model.endTime ? dayjs(model.endTime).toISOString() : undefined
  };

  try {
    if (props.operateType === 'add') {
      await fetchCreateBannerItem(params);
      window.$message?.success('新增成功');
    } else {
      await fetchUpdateBannerItem(props.rowData!.id, params);
      window.$message?.success('更新成功');
    }

    closeModal();
    emit('submitted');
  } finally {
    loading.value = false;
  }
}

watch(
  () => props.visible,
  val => {
    if (val) {
      loadGroups();
      handleInitModel();
    }
  },
  { immediate: false }
);
</script>

<template>
  <NModal v-model:show="drawerVisible" preset="card" :title="title" :style="{ width: '600px' }" class="overflow-y-auto">
    <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="100">
      <NFormItem label="Banner组" path="groupId">
        <NSelect
          v-model:value="model.groupId"
          placeholder="请选择Banner组"
          :options="groupOptions"
          :disabled="!!groupId"
        />
      </NFormItem>
      <NFormItem label="标题" path="title">
        <NInput v-model:value="model.title" placeholder="请输入标题" maxlength="200" />
      </NFormItem>
      <NFormItem label="副标题" path="subtitle">
        <NInput v-model:value="model.subtitle" placeholder="请输入副标题" maxlength="200" />
      </NFormItem>
      <NFormItem label="图片" path="imageUrl">
        <NSpace align="center">
          <NUpload :custom-request="handleImageUpload as any" :show-file-list="false" accept="image/*">
            <NButton :loading="imageUploading">上传图片</NButton>
          </NUpload>
          <NImage v-if="model.imageUrl" :src="model.imageUrl" width="120" height="80" object-fit="cover" />
        </NSpace>
      </NFormItem>
      <NFormItem label="图片描述" path="imageAlt">
        <NInput v-model:value="model.imageAlt" placeholder="请输入图片描述" maxlength="200" />
      </NFormItem>
      <NFormItem label="链接类型" path="linkType">
        <NSelect
          v-model:value="model.linkType"
          placeholder="请选择链接类型"
          :options="[
            { label: '无链接', value: 'none' },
            { label: '内部链接', value: 'internal' },
            { label: '外部链接', value: 'external' },
            { label: '文章', value: 'article' }
          ]"
        />
      </NFormItem>
      <NFormItem v-if="model.linkType === 'external'" label="外部链接" path="linkUrl">
        <NInput v-model:value="model.linkUrl" placeholder="请输入外部链接地址" maxlength="500" />
      </NFormItem>
      <NFormItem v-if="model.linkType === 'article'" label="文章ID" path="linkArticleId">
        <NInputNumber v-model:value="model.linkArticleId" placeholder="请输入文章ID" :min="1" class="w-full" />
      </NFormItem>
      <NFormItem label="纯文本内容" path="content">
        <NInput
          v-model:value="model.content"
          type="textarea"
          placeholder="请输入纯文本内容"
          :autosize="{ minRows: 3, maxRows: 6 }"
        />
      </NFormItem>
      <NFormItem label="自定义参数" path="customParams">
        <NInput
          v-model:value="model.customParams"
          type="textarea"
          placeholder="JSON格式的自定义参数"
          :autosize="{ minRows: 2, maxRows: 4 }"
        />
      </NFormItem>
      <NFormItem label="排序" path="sort">
        <NInputNumber v-model:value="model.sort" :min="0" class="w-full" />
      </NFormItem>
      <NGrid :cols="2" :x-gap="16">
        <NGridItem>
          <NFormItem label="开始时间" path="startTime">
            <NDatePicker
              v-model:value="model.startTime"
              type="datetime"
              placeholder="开始显示时间"
              class="w-full"
              format="yyyy-MM-dd HH:mm:ss"
              value-format="yyyy-MM-dd HH:mm:ss"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem>
          <NFormItem label="结束时间" path="endTime">
            <NDatePicker
              v-model:value="model.endTime"
              type="datetime"
              placeholder="结束显示时间"
              class="w-full"
              format="yyyy-MM-dd HH:mm:ss"
              value-format="yyyy-MM-dd HH:mm:ss"
            />
          </NFormItem>
        </NGridItem>
      </NGrid>
      <NFormItem label="状态" path="status">
        <NRadioGroup v-model:value="model.status">
          <NRadio value="1">启用</NRadio>
          <NRadio value="0">禁用</NRadio>
        </NRadioGroup>
      </NFormItem>
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
