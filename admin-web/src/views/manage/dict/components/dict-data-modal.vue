<script setup lang="ts">
import { h, ref, watch } from 'vue';
import type { VNode } from 'vue';
import { NTag } from 'naive-ui';
import type { SelectOption } from 'naive-ui';
import { fetchCreateDictData, fetchUpdateDictData } from '@/service/api/v1/system-dict';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import type { SystemDict } from '@/typings/api/v1/system-dict';
import { $t } from '@/locales';
import AppDictRadioGroup from '@/components/custom/app-dict-radio-group.vue';

defineOptions({ name: 'DictDataModal' });

interface Props {
  mode: 'add' | 'edit';
  rowData?: SystemDict.DictData | null;
  dictCode: string;
}

const props = defineProps<Props>();
const emit = defineEmits<{ (e: 'submitted'): void }>();
const visible = defineModel<boolean>('visible', { default: false });

const { formRef, validate, restoreValidation } = useNaiveForm();
const { defaultRequiredRule } = useFormRules();
const loading = ref(false);

const TAG_TYPE_OPTIONS: SelectOption[] = [
  { label: 'default', value: 'default' },
  { label: 'success', value: 'success' },
  { label: 'info', value: 'info' },
  { label: 'warning', value: 'warning' },
  { label: 'error', value: 'error' },
  { label: 'primary', value: 'primary' }
];

interface Model {
  dictCode: string;
  label: string;
  value: string;
  tagType: NaiveUI.ThemeColor;
  orderBy: number;
  status: string;
  remark: string;
}

const model = ref<Model>(createDefault());

function createDefault(): Model {
  return {
    dictCode: props.dictCode,
    label: '',
    value: '',
    tagType: 'default',
    orderBy: 0,
    status: '1',
    remark: ''
  };
}

const rules = {
  label: defaultRequiredRule,
  value: defaultRequiredRule,
  status: defaultRequiredRule,
  tagType: defaultRequiredRule
};

function renderOption({ option }: { node: VNode; option: SelectOption }) {
  return h(
    NTag,
    {
      type: option.value as NaiveUI.ThemeColor,
      size: 'small',
      bordered: false
    },
    { default: () => option.label }
  );
}

watch(visible, val => {
  if (val) {
    restoreValidation();
    if (props.mode === 'edit' && props.rowData) {
      Object.assign(model.value, props.rowData);
    } else {
      model.value = createDefault();
      model.value.dictCode = props.dictCode;
    }
  }
});

async function handleSubmit() {
  await validate();
  loading.value = true;
  try {
    if (props.mode === 'edit') {
      await fetchUpdateDictData({ ...model.value, id: props.rowData!.id } as SystemDict.DictData);
    } else {
      await fetchCreateDictData(model.value);
    }
    visible.value = false;
    emit('submitted');
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <NModal
    v-model:show="visible"
    preset="card"
    :title="mode === 'add' ? $t('page.manage.dict.addData') : $t('page.manage.dict.editData')"
    class="w-540px"
  >
    <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="90">
      <NFormItem :label="$t('page.manage.dict.dataLabel')" path="label">
        <NInput v-model:value="model.label" :placeholder="$t('page.manage.dict.dataLabel')" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.dict.dataValue')" path="value">
        <NInput v-model:value="model.value" :placeholder="$t('page.manage.dict.dataValue')" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.dict.tagType')" path="tagType">
        <NSelect v-model:value="model.tagType" :options="TAG_TYPE_OPTIONS" :render-option="renderOption" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.dict.orderBy')">
        <NInputNumber v-model:value="model.orderBy" class="w-full" :min="0" />
      </NFormItem>
      <NFormItem :label="$t('common.status')" path="status">
        <AppDictRadioGroup v-model:value="model.status" dict-code="sys_status" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.dict.remark')">
        <NInput v-model:value="model.remark" type="textarea" :rows="2" :placeholder="$t('page.manage.dict.remark')" />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="visible = false">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="loading" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>
