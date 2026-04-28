<script setup lang="ts">
import { computed, reactive, watch } from 'vue';
import { addApi, updateApi } from '@/service/api/v1/open-api';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import { $t } from '@/locales';
import type { OpenApi } from '@/typings/api/v1/open-api';

defineOptions({
  name: 'ApiOperateModal'
});

interface Props {
  operateType: NaiveUI.TableOperateType;
  rowData?: OpenApi.Api | null;
}

const props = defineProps<Props>();

interface Emits {
  (e: 'submitted'): void;
}

const emit = defineEmits<Emits>();

const visible = defineModel<boolean>('visible', {
  default: false
});

const { formRef, validate, restoreValidation } = useNaiveForm();
const { defaultRequiredRule } = useFormRules();

const title = computed(() => {
  const titles: Record<NaiveUI.TableOperateType, string> = {
    add: $t('common.add'),
    edit: $t('common.edit')
  };
  return titles[props.operateType];
});

type Model = OpenApi.CreateApiReq & { id?: number };

const model: Model = reactive(createDefaultModel());

function createDefaultModel(): Model {
  return {
    method: 'GET',
    path: '',
    name: '',
    group: 'default',
    description: '',
    status: 1
  };
}

const rules: Record<string, App.Global.FormRule[]> = {
  method: [defaultRequiredRule],
  path: [defaultRequiredRule],
  name: [defaultRequiredRule]
};

async function handleSubmit() {
  await validate();

  if (props.operateType === 'add') {
    const { error } = await addApi(model);
    if (!error) {
      window.$message?.success($t('common.addSuccess'));
      closeModal();
      emit('submitted');
    }
  } else if (props.operateType === 'edit' && model.id) {
    const { error } = await updateApi(model as OpenApi.UpdateApiReq);
    if (!error) {
      window.$message?.success($t('common.updateSuccess'));
      closeModal();
      emit('submitted');
    }
  }
}

function closeModal() {
  visible.value = false;
}

watch(visible, () => {
  if (visible.value) {
    if (props.operateType === 'edit' && props.rowData) {
      Object.assign(model, { ...props.rowData });
    } else {
      Object.assign(model, createDefaultModel());
    }
    restoreValidation();
  }
});
</script>

<template>
  <NModal v-model:show="visible" :title="title" preset="card" class="w-600px">
    <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="100">
      <NFormItem :label="$t('page.openPlatform.api.method')" path="method">
        <NSelect
          v-model:value="model.method"
          :options="[
            { label: 'GET', value: 'GET' },
            { label: 'POST', value: 'POST' },
            { label: 'PUT', value: 'PUT' },
            { label: 'DELETE', value: 'DELETE' },
            { label: 'PATCH', value: 'PATCH' }
          ]"
        />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.api.path')" path="path">
        <NInput v-model:value="model.path" :placeholder="$t('page.openPlatform.api.form.pathPlaceholder')" />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.api.name')" path="name">
        <NInput v-model:value="model.name" :placeholder="$t('page.openPlatform.api.form.namePlaceholder')" />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.api.group')" path="group">
        <NInput v-model:value="model.group" :placeholder="$t('page.openPlatform.api.form.groupPlaceholder')" />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.api.status')" path="status">
        <AppDictSelect v-model:value="model.status" dict-code="sys_status" value-type="number" />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.api.description')" path="description">
        <NInput v-model:value="model.description" type="textarea" />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="closeModal">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
