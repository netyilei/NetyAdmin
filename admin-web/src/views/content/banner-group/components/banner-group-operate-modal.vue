<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import { fetchCreateBannerGroup, fetchUpdateBannerGroup } from '@/service/api/v1/content';
import { useFormRules } from '@/hooks/common/form';
import type { Content } from '@/typings/api/v1/content';
import { $t } from '@/locales';

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
  return props.operateType === 'add' ? '新增Banner组' : '编辑Banner组';
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
      window.$message?.success('新增成功');
    } else {
      await fetchUpdateBannerGroup(props.rowData!.id, params);
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
      <NFormItem label="组名称" path="name">
        <NInput v-model:value="model.name" placeholder="请输入Banner组名称" maxlength="100" />
      </NFormItem>
      <NFormItem label="编码" path="code">
        <NInput v-model:value="model.code" placeholder="请输入唯一编码" maxlength="50" />
      </NFormItem>
      <NFormItem label="描述" path="description">
        <NInput v-model:value="model.description" placeholder="请输入描述" maxlength="255" />
      </NFormItem>
      <NFormItem label="位置标识" path="position">
        <NInput v-model:value="model.position" placeholder="如: home_top, sidebar" maxlength="50" />
      </NFormItem>
      <NGrid :cols="2" :x-gap="16">
        <NGridItem>
          <NFormItem label="宽度" path="width">
            <NInputNumber v-model:value="model.width" placeholder="建议宽度" :min="0" class="w-full" />
          </NFormItem>
        </NGridItem>
        <NGridItem>
          <NFormItem label="高度" path="height">
            <NInputNumber v-model:value="model.height" placeholder="建议高度" :min="0" class="w-full" />
          </NFormItem>
        </NGridItem>
      </NGrid>
      <NFormItem label="最大数量" path="maxItems">
        <NInputNumber v-model:value="model.maxItems" placeholder="最大Banner数量" :min="1" :max="50" class="w-full" />
      </NFormItem>
      <NFormItem label="自动播放" path="autoPlay">
        <NSwitch v-model:value="model.autoPlay" />
      </NFormItem>
      <NFormItem v-if="model.autoPlay" label="轮播间隔" path="interval">
        <NInputNumber v-model:value="model.interval" placeholder="毫秒" :min="1000" :step="1000" class="w-full">
          <template #suffix>ms</template>
        </NInputNumber>
      </NFormItem>
      <NFormItem label="排序" path="sort">
        <NInputNumber v-model:value="model.sort" placeholder="排序" :min="0" class="w-full" />
      </NFormItem>
      <NFormItem label="状态" path="status">
        <NRadioGroup v-model:value="model.status">
          <NRadio value="1">启用</NRadio>
          <NRadio value="0">禁用</NRadio>
        </NRadioGroup>
      </NFormItem>
      <NFormItem label="备注" path="remark">
        <NInput
          v-model:value="model.remark"
          placeholder="请输入备注"
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
