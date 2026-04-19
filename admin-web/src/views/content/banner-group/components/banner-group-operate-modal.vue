<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import { fetchCreateBannerGroup, fetchUpdateBannerGroup } from '@/service/api/v1/content';
import { useFormRules } from '@/hooks/common/form';
import type { Content } from '@/typings/api/v1/content';
import { $t } from '@/locales';
import StorageConfigSelect from '@/components/custom/storage-config-select.vue';

defineOptions({
  name: 'BannerGroupOperateModal'
});

interface Props {
  visible: boolean;
  operateType: 'add' | 'edit';
  rowData?: Content.BannerGroup | null;
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
  return props.operateType === 'add'
    ? $t('page.content.bannerGroup.addGroup')
    : $t('page.content.bannerGroup.editGroup');
});

type Model = Pick<
  Content.CreateBannerGroupParams,
  | 'name'
  | 'code'
  | 'description'
  | 'position'
  | 'width'
  | 'height'
  | 'maxItems'
  | 'autoPlay'
  | 'interval'
  | 'sort'
  | 'storageConfigId'
  | 'status'
  | 'remark'
>;

const model: Model = reactive({
  name: '',
  code: '',
  description: '',
  position: '',
  width: 0,
  height: 0,
  maxItems: 10,
  autoPlay: true,
  interval: 5000,
  sort: 0,
  storageConfigId: null,
  status: '1',
  remark: ''
});

type RuleKey = Extract<keyof Model, 'name' | 'code'>;

const rules: Record<RuleKey, App.Global.FormRule> = {
  name: defaultRequiredRule,
  code: defaultRequiredRule
};

const loading = ref(false);

function initModel() {
  model.name = '';
  model.code = '';
  model.description = '';
  model.position = '';
  model.width = 0;
  model.height = 0;
  model.maxItems = 10;
  model.autoPlay = true;
  model.interval = 5000;
  model.sort = 0;
  model.storageConfigId = null;
  model.status = '1';
  model.remark = '';
}

async function handleInitModel() {
  initModel();

  if (props.operateType === 'add') {
    return;
  }

  if (props.rowData) {
    Object.assign(model, {
      name: props.rowData.name,
      code: props.rowData.code,
      description: props.rowData.description,
      position: props.rowData.position,
      width: props.rowData.width,
      height: props.rowData.height,
      maxItems: props.rowData.maxItems,
      autoPlay: props.rowData.autoPlay,
      interval: props.rowData.interval,
      sort: props.rowData.sort,
      storageConfigId: props.rowData.storageConfigId,
      status: props.rowData.status,
      remark: props.rowData.remark
    });
  }
}

function closeModal() {
  drawerVisible.value = false;
}

async function handleSubmit() {
  loading.value = true;

  const params = { ...model };

  try {
    if (props.operateType === 'add') {
      await fetchCreateBannerGroup(params);
      window.$message?.success($t('common.addSuccess'));
    } else {
      await fetchUpdateBannerGroup(props.rowData!.id, params);
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
  },
  { immediate: false }
);
</script>

<template>
  <NModal
    v-model:show="drawerVisible"
    preset="card"
    :title="title"
    :style="{ width: '800px', maxWidth: '95vw' }"
    class="overflow-y-auto"
  >
    <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="100">
      <NFormItem :label="$t('page.content.bannerGroup.groupName')" path="name">
        <NInput
          v-model:value="model.name"
          :placeholder="$t('page.content.bannerGroup.form.groupName')"
          maxlength="100"
        />
      </NFormItem>
      <NFormItem :label="$t('page.content.bannerGroup.groupCode')" path="code">
        <NInput
          v-model:value="model.code"
          :placeholder="$t('page.content.bannerGroup.form.groupCode')"
          maxlength="50"
        />
      </NFormItem>
      <NFormItem :label="$t('page.content.bannerGroup.description')" path="description">
        <NInput
          v-model:value="model.description"
          :placeholder="$t('page.content.bannerGroup.form.description')"
          maxlength="255"
        />
      </NFormItem>
      <NFormItem :label="$t('page.content.bannerGroup.position')" path="position">
        <NInput
          v-model:value="model.position"
          :placeholder="$t('page.content.bannerGroup.form.position')"
          maxlength="50"
        />
      </NFormItem>
      <NFormItem :label="$t('page.content.bannerGroup.storageConfigId')" path="storageConfigId">
        <StorageConfigSelect
          v-model:value="model.storageConfigId"
          :placeholder="$t('page.content.bannerGroup.form.storageConfigId')"
        />
      </NFormItem>
      <NGrid :cols="2" :x-gap="16">
        <NGridItem>
          <NFormItem :label="$t('page.content.bannerGroup.size')" path="width">
            <NInputNumber v-model:value="model.width" placeholder="W" :min="0" class="w-full" />
          </NFormItem>
        </NGridItem>
        <NGridItem>
          <NFormItem label=" " :show-label="true" path="height">
            <NInputNumber v-model:value="model.height" placeholder="H" :min="0" class="w-full" />
          </NFormItem>
        </NGridItem>
      </NGrid>
      <NFormItem :label="$t('page.content.bannerGroup.maxItems')" path="maxItems">
        <NInputNumber
          v-model:value="model.maxItems"
          :placeholder="$t('page.content.bannerGroup.maxItems')"
          :min="1"
          :max="50"
          class="w-full"
        />
      </NFormItem>
      <NFormItem :label="$t('page.content.bannerGroup.autoPlay')" path="autoPlay">
        <NSwitch v-model:value="model.autoPlay" />
      </NFormItem>
      <NFormItem v-if="model.autoPlay" :label="$t('page.content.bannerGroup.interval')" path="interval">
        <NInputNumber
          v-model:value="model.interval"
          :placeholder="$t('page.content.bannerGroup.interval')"
          :min="1000"
          :step="1000"
          class="w-full"
        >
          <template #suffix>ms</template>
        </NInputNumber>
      </NFormItem>
      <NFormItem :label="$t('page.content.bannerGroup.sort')" path="sort">
        <NInputNumber
          v-model:value="model.sort"
          :placeholder="$t('page.content.bannerGroup.form.sort')"
          :min="0"
          class="w-full"
        />
      </NFormItem>
      <NFormItem :label="$t('common.status')" path="status">
        <NRadioGroup v-model:value="model.status">
          <NRadio value="1">{{ $t('common.enable') }}</NRadio>
          <NRadio value="0">{{ $t('common.disable') }}</NRadio>
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
    <template #footer>
      <NSpace :size="16">
        <NButton @click="closeModal">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="loading" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
