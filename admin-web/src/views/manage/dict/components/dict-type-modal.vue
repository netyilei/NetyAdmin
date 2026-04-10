<script setup lang="ts">
import { ref, watch } from 'vue';
import { fetchCreateDictType, fetchUpdateDictType } from '@/service/api/v1/system-dict';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import type { SystemDict } from '@/typings/api/v1/system-dict';
import { $t } from '@/locales';
import AppDictRadioGroup from '@/components/custom/app-dict-radio-group.vue';

defineOptions({ name: 'DictTypeModal' });

interface Props {
  mode: 'add' | 'edit';
  rowData?: SystemDict.DictType | null;
}

const props = defineProps<Props>();
const emit = defineEmits<{ (e: 'submitted'): void }>();
const visible = defineModel<boolean>('visible', { default: false });

const { formRef, validate, restoreValidation } = useNaiveForm();
const { defaultRequiredRule } = useFormRules();
const loading = ref(false);

interface Model {
  name: string;
  code: string;
  status: string;
  description: string;
}

const model = ref<Model>(createDefault());

function createDefault(): Model {
  return { name: '', code: '', status: '1', description: '' };
}

const rules = {
  name: defaultRequiredRule,
  code: defaultRequiredRule,
  status: defaultRequiredRule
};

watch(visible, val => {
  if (val) {
    restoreValidation();
    if (props.mode === 'edit' && props.rowData) {
      Object.assign(model.value, props.rowData);
    } else {
      model.value = createDefault();
    }
  }
});

async function handleSubmit() {
  await validate();
  loading.value = true;
  try {
    if (props.mode === 'edit') {
      await fetchUpdateDictType({ ...model.value, id: props.rowData!.id } as SystemDict.DictType);
    } else {
      await fetchCreateDictType(model.value);
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
    :title="mode === 'add' ? $t('page.manage.dict.addType') : $t('page.manage.dict.editType')"
    class="w-500px"
  >
    <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="90">
      <NFormItem :label="$t('page.manage.dict.typeName')" path="name">
        <NInput v-model:value="model.name" :placeholder="$t('page.manage.dict.typeName')" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.dict.typeCode')" path="code">
        <NInput v-model:value="model.code" :placeholder="$t('page.manage.dict.typeCode')" :disabled="mode === 'edit'" />
      </NFormItem>
      <NFormItem :label="$t('common.status')" path="status">
        <AppDictRadioGroup v-model:value="model.status" dict-code="sys_status" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.dict.description')">
        <NInput
          v-model:value="model.description"
          type="textarea"
          :rows="2"
          :placeholder="$t('page.manage.dict.description')"
        />
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
