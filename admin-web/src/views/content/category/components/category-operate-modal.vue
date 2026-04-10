<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import {
  NButton,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NRadio,
  NRadioGroup,
  NSpace,
  NTreeSelect
} from 'naive-ui';
import { fetchCreateCategory, fetchUpdateCategory } from '@/service/api/v1/content';
import { useFormRules } from '@/hooks/common/form';
import { getAllIconifyIcons } from '@/utils/iconify-icons';
import type { Content } from '@/typings/api/v1/content';
import { $t } from '@/locales';
import IconifyIconSelect from '@/components/custom/iconify-icon-select.vue';

defineOptions({
  name: 'CategoryOperateModal'
});

interface Props {
  visible: boolean;
  operateType: 'add' | 'edit';
  rowData?: Content.Category | null;
  allCategories?: Content.CategoryTree[];
}

interface Emits {
  (e: 'update:visible', visible: boolean): void;
  (e: 'submitted'): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const allIconifyIcons = getAllIconifyIcons();

const treeOptions = computed(() => {
  const options = [
    {
      label: $t('page.content.category.topLevel'),
      key: 0
    }
  ];

  function transform(nodes: Content.CategoryTree[], shouldDisable = false): any[] {
    return nodes.map(node => {
      const isCurrentNode = props.operateType === 'edit' && node.id === props.rowData?.id;
      const disabled = shouldDisable || isCurrentNode;

      return {
        label: node.name,
        key: node.id,
        children: node.children?.length ? transform(node.children, disabled) : undefined,
        disabled
      };
    });
  }

  if (props.allCategories) {
    options.push(...transform(props.allCategories));
  }

  return options;
});

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
  return props.operateType === 'add'
    ? $t('page.content.category.addCategory')
    : $t('page.content.category.editCategory');
});

type Model = Pick<
  Content.CreateCategoryParams,
  'parentId' | 'name' | 'code' | 'icon' | 'sort' | 'contentType' | 'status' | 'remark'
>;

function createDefaultModel(): Model {
  return {
    parentId: 0,
    name: '',
    code: '',
    icon: '',
    sort: 0,
    contentType: 'richtext',
    status: '1',
    remark: ''
  };
}

const model = reactive<Model>(createDefaultModel());

const rules: Record<string, App.Global.FormRule | App.Global.FormRule[]> = {
  name: defaultRequiredRule,
  code: defaultRequiredRule,
  contentType: defaultRequiredRule
};

const loading = ref(false);

const formRef = ref<import('naive-ui').FormInst | null>(null);

async function handleInitModel() {
  Object.assign(model, createDefaultModel());

  if (props.operateType === 'edit' && props.rowData) {
    Object.assign(model, {
      parentId: props.rowData.parentId,
      name: props.rowData.name,
      code: props.rowData.code,
      icon: props.rowData.icon,
      sort: props.rowData.sort,
      contentType: props.rowData.contentType,
      status: props.rowData.status,
      remark: props.rowData.remark
    });
  }
}

function closeModal() {
  drawerVisible.value = false;
}

async function handleSubmit() {
  await formRef.value?.validate();

  loading.value = true;
  try {
    if (props.operateType === 'add') {
      await fetchCreateCategory(model);
      window.$message?.success($t('common.addSuccess'));
    } else {
      await fetchUpdateCategory(props.rowData!.id, model);
      window.$message?.success($t('common.updateSuccess'));
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
      handleInitModel();
    }
  }
);
</script>

<template>
  <NModal v-model:show="drawerVisible" preset="card" :title="title" :style="{ width: '800px', maxWidth: '95vw' }">
    <NScrollbar class="max-h-600px pr-20px">
      <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="100">
        <NFormItem :label="$t('page.content.category.parentId')" path="parentId">
          <NTreeSelect
            v-model:value="model.parentId"
            :options="treeOptions"
            :placeholder="$t('page.content.category.form.parentId')"
            class="w-full"
          />
        </NFormItem>
        <NFormItem :label="$t('page.content.category.categoryName')" path="name">
          <NInput
            v-model:value="model.name"
            :placeholder="$t('page.content.category.form.categoryName')"
            maxlength="50"
          />
        </NFormItem>
        <NFormItem :label="$t('page.content.category.categoryCode')" path="code">
          <NInput
            v-model:value="model.code"
            :placeholder="$t('page.content.category.form.categoryCode')"
            maxlength="50"
          />
        </NFormItem>
        <NFormItem :label="$t('page.content.category.icon')" path="icon">
          <IconifyIconSelect v-model:value="model.icon as string" :icons="allIconifyIcons" />
        </NFormItem>
        <NFormItem :label="$t('page.content.category.sort')" path="sort">
          <NInputNumber
            v-model:value="model.sort"
            :placeholder="$t('page.content.category.form.sort')"
            :min="0"
            class="w-full"
          />
        </NFormItem>
        <NFormItem :label="$t('page.content.category.contentType')" path="contentType">
          <NRadioGroup v-model:value="model.contentType">
            <NSpace>
              <NRadio value="plaintext">{{ $t('page.content.category.contentTypePlain') }}</NRadio>
              <NRadio value="richtext">{{ $t('page.content.category.contentTypeRich') }}</NRadio>
            </NSpace>
          </NRadioGroup>
        </NFormItem>
        <NFormItem :label="$t('page.content.category.status')" path="status">
          <NRadioGroup v-model:value="model.status">
            <NSpace>
              <NRadio value="1">{{ $t('common.enable') }}</NRadio>
              <NRadio value="0">{{ $t('common.disable') }}</NRadio>
            </NSpace>
          </NRadioGroup>
        </NFormItem>
        <NFormItem :label="$t('page.content.category.remark')" path="remark">
          <NInput
            v-model:value="model.remark"
            :placeholder="$t('page.content.category.form.remark')"
            type="textarea"
            :autosize="{ minRows: 3, maxRows: 5 }"
          />
        </NFormItem>
      </NForm>
    </NScrollbar>
    <template #footer>
      <NSpace :size="16" justify="end">
        <NButton @click="closeModal">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="loading" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
