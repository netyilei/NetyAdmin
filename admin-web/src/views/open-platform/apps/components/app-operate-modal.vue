<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { addApp, fetchAvailableScopes, linkAppIPRules, updateApp } from '@/service/api/v1/open-app';
import { fetchIPACList } from '@/service/api/v1/system-ipac';
import { fetchGetAllEnabledStorageConfigs } from '@/service/api/v1/storage';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import { $t } from '@/locales';
import type { OpenApp } from '@/typings/api/v1/open-app';
import type { Storage } from '@/typings/api/v1/storage';

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

const availableScopes = ref<{ name: string; code: string }[]>([]);
const ipRuleOptions = ref<{ label: string; value: number }[]>([]);
const selectedRuleIds = ref<number[]>([]);
const ipRulesLoading = ref(false);
const storageConfigOptions = ref<{ label: string; value: number }[]>([]);
const storageConfigLoading = ref(false);

const scopeOptions = computed(() => {
  return availableScopes.value.map(item => ({
    label: item.name,
    value: item.code
  }));
});

async function getAvailableScopes() {
  const { data } = await fetchAvailableScopes();
  if (data) {
    availableScopes.value = data;
  }
}

async function loadIPRules() {
  ipRulesLoading.value = true;
  const { data } = await fetchIPACList({ current: 1, size: 500 });
  if (data) {
    ipRuleOptions.value = data.records.map(item => ({
      label: `${item.ipAddr} (${item.type === 1 ? $t('page.openPlatform.ipac.typeAllow') : $t('page.openPlatform.ipac.typeDeny')})`,
      value: item.id
    }));
  }
  ipRulesLoading.value = false;
}

async function loadStorageConfigs() {
  storageConfigLoading.value = true;
  const { data } = await fetchGetAllEnabledStorageConfigs();
  if (data) {
    storageConfigOptions.value = [
      { label: $t('page.openPlatform.app.form.storageIdDefault'), value: 0 },
      ...data.map((item: Storage.StorageConfig) => ({
        label: item.name,
        value: item.id
      }))
    ];
  }
  storageConfigLoading.value = false;
}

async function loadAppIPRules(appId: string) {
  const { data } = await fetchIPACList({ current: 1, size: 500, appId: Number(appId) || undefined });
  if (data) {
    selectedRuleIds.value = data.records.map(item => item.id);
  }
}

onMounted(() => {
  getAvailableScopes();
  loadIPRules();
  loadStorageConfigs();
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
    ipFilterEnabled: false,
    remark: '',
    storageId: 0,
    scopes: []
  };
}

const rules: Record<string, App.Global.FormRule[]> = {
  name: [defaultRequiredRule],
  status: [defaultRequiredRule]
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
      if (model.ipFilterEnabled) {
        await linkAppIPRules({ id: model.id, ruleIds: selectedRuleIds.value });
      }
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
      if (props.rowData.id) {
        loadAppIPRules(props.rowData.id);
      }
    } else {
      Object.assign(model, createDefaultModel());
      selectedRuleIds.value = [];
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
      <NFormItem :label="$t('page.openPlatform.app.ipFilterEnabled')" path="ipFilterEnabled">
        <NSwitch v-model:value="model.ipFilterEnabled" />
      </NFormItem>
      <NFormItem v-if="model.ipFilterEnabled" :label="$t('page.openPlatform.app.ipRules')">
        <NSelect
          v-model:value="selectedRuleIds"
          multiple
          :options="ipRuleOptions"
          :loading="ipRulesLoading"
          :placeholder="$t('page.openPlatform.app.form.ipRulesPlaceholder')"
        />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.app.remark')" path="remark">
        <NInput
          v-model:value="model.remark"
          type="textarea"
          :placeholder="$t('page.openPlatform.app.form.remarkPlaceholder')"
        />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.app.storageId')" path="storageId">
        <NSelect
          v-model:value="model.storageId"
          :options="storageConfigOptions"
          :loading="storageConfigLoading"
          :placeholder="$t('page.openPlatform.app.form.storageIdPlaceholder')"
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
