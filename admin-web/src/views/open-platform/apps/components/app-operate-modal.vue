<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { addApp, fetchAvailableScopes, updateApp } from '@/service/api/v1/open-app';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import { $t } from '@/locales';
import type { OpenApp } from '@/typings/api/v1/open-app';

defineOptions({
  name: 'AppOperateModal'
});

interface Props {
  operateType: NaiveUI.TableOperateType;
  rowData?: OpenApp.App | null;
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

const availableScopes = ref<{ label: string; value: string }[]>([]);

const scopeOptions = computed(() => {
  return availableScopes.value.map(item => ({
    label: item.label,
    value: item.value
  }));
});

async function getAvailableScopes() {
  const { data } = await fetchAvailableScopes();
  if (data) {
    availableScopes.value = data;
  }
}

onMounted(() => {
  getAvailableScopes();
});

const title = computed(() => {
  const titles: Record<NaiveUI.TableOperateType, string> = {
    add: $t('common.add'),
    edit: $t('common.edit')
  };
  return titles[props.operateType];
});

type Model = OpenApp.CreateAppReq & { id?: string };

const model: Model = reactive(createDefaultModel());

function createDefaultModel(): Model {
  return {
    name: '',
    status: 1,
    ipStrategy: 1,
    remark: '',
    scopes: []
  };
}

const rules: Record<string, App.Global.FormRule[]> = {
  name: [defaultRequiredRule],
  status: [defaultRequiredRule],
  ipStrategy: [defaultRequiredRule]
};

async function handleSubmit() {
  await validate();

  if (props.operateType === 'add') {
    const { error } = await addApp(model);
    if (!error) {
      window.$message?.success($t('common.addSuccess'));
      closeModal();
      emit('submitted');
    }
  } else if (props.operateType === 'edit' && model.id) {
    const { error } = await updateApp(model as OpenApp.UpdateAppReq);
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
      Object.assign(model, {
        ...props.rowData
      });
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
      <NFormItem :label="$t('page.openPlatform.app.name')" path="name">
        <NInput v-model:value="model.name" :placeholder="$t('page.openPlatform.app.form.namePlaceholder')" />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.app.status')" path="status">
        <AppDictSelect
          v-model:value="model.status"
          dict-code="sys_status"
          :placeholder="$t('page.openPlatform.app.form.statusPlaceholder')"
        />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.app.ipStrategy')" path="ipStrategy">
        <AppDictSelect
          v-model:value="model.ipStrategy"
          dict-code="sys_app_ip_strategy"
          :placeholder="$t('page.openPlatform.app.form.ipStrategyPlaceholder')"
        />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.app.remark')" path="remark">
        <NInput
          v-model:value="model.remark"
          type="textarea"
          :placeholder="$t('page.openPlatform.app.form.remarkPlaceholder')"
        />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.app.scopes')" path="scopes">
        <NSelect
          v-model:value="model.scopes"
          multiple
          :options="scopeOptions"
          :placeholder="$t('page.openPlatform.app.form.scopesPlaceholder')"
        />
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
