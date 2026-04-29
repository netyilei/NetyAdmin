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

const quotaRate = ref<number | null>(null);
const quotaCapacity = ref<number | null>(null);

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
  const { data } = await fetchIPACList({ current: 1, size: 500, appId });
  if (data) {
    selectedRuleIds.value = data.records.map(item => item.id);
  }
}

function parseQuotaConfig(quotaConfig: string | undefined) {
  if (!quotaConfig) {
    quotaRate.value = null;
    quotaCapacity.value = null;
    return;
  }
  try {
    const parsed = JSON.parse(quotaConfig);
    quotaRate.value = parsed.rate ?? null;
    quotaCapacity.value = parsed.capacity ?? null;
  } catch {
    quotaRate.value = null;
    quotaCapacity.value = null;
  }
}

function buildQuotaConfig(): string | undefined {
  if (quotaRate.value === null && quotaCapacity.value === null) {
    return undefined;
  }
  const quota: Record<string, number> = {};
  if (quotaRate.value !== null) {
    quota.rate = quotaRate.value;
  }
  if (quotaCapacity.value !== null) {
    quota.capacity = quotaCapacity.value;
  }
  return JSON.stringify(quota);
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
    rateLimitEnabled: true,
    remark: '',
    quotaConfig: '',
    cacheTTL: 0,
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

  model.quotaConfig = buildQuotaConfig();

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
        id: props.rowData.id,
        name: props.rowData.name,
        status: props.rowData.status,
        ipFilterEnabled: props.rowData.ipFilterEnabled,
        rateLimitEnabled: props.rowData.rateLimitEnabled,
        remark: props.rowData.remark,
        cacheTTL: props.rowData.cacheTTL,
        storageId: props.rowData.storageId,
        scopes: props.rowData.scopes || []
      });
      parseQuotaConfig(props.rowData.quotaConfig);
      if (props.rowData.id) {
        loadAppIPRules(props.rowData.id);
      }
    } else {
      Object.assign(model, createDefaultModel());
      parseQuotaConfig(undefined);
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
          value-type="number"
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
      <NFormItem :label="$t('page.openPlatform.app.rateLimitEnabled')" path="rateLimitEnabled">
        <NSwitch v-model:value="model.rateLimitEnabled" />
      </NFormItem>
      <NFormItem v-if="model.rateLimitEnabled" :label="$t('page.openPlatform.app.quotaConfig')" path="quotaConfig">
        <NCard size="small" :bordered="true" class="w-full">
          <NSpace vertical :size="14">
            <NFormItem
              :label="$t('page.openPlatform.app.quotaRate')"
              :show-feedback="false"
              label-placement="left"
              :label-width="120"
            >
              <NInputNumber
                v-model:value="quotaRate"
                :min="1"
                :max="10000"
                :placeholder="$t('page.openPlatform.app.form.quotaRatePlaceholder')"
                class="w-full"
              >
                <template #suffix>{{ $t('page.openPlatform.app.form.quotaRateSuffix') }}</template>
              </NInputNumber>
            </NFormItem>
            <NText depth="3" class="quota-tip">
              {{ $t('page.openPlatform.app.form.quotaRateTip') }}
            </NText>
            <NFormItem
              :label="$t('page.openPlatform.app.quotaCapacity')"
              :show-feedback="false"
              label-placement="left"
              :label-width="120"
            >
              <NInputNumber
                v-model:value="quotaCapacity"
                :min="1"
                :max="100000"
                :placeholder="$t('page.openPlatform.app.form.quotaCapacityPlaceholder')"
                class="w-full"
              >
                <template #suffix>{{ $t('page.openPlatform.app.form.quotaCapacitySuffix') }}</template>
              </NInputNumber>
            </NFormItem>
            <NText depth="3" class="quota-tip">
              {{ $t('page.openPlatform.app.form.quotaCapacityTip') }}
            </NText>
          </NSpace>
        </NCard>
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.app.cacheTTL')" path="cacheTTL">
        <NInputNumber
          v-model:value="model.cacheTTL"
          :min="0"
          :max="2592000"
          :placeholder="$t('page.openPlatform.app.form.cacheTTLPlaceholder')"
          class="w-full"
        >
          <template #suffix>{{ $t('page.openPlatform.app.form.cacheTTLSuffix') }}</template>
        </NInputNumber>
      </NFormItem>
      <NText depth="3" class="cache-ttl-tip">
        {{ $t('page.openPlatform.app.form.cacheTTLTip') }}
      </NText>
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

<style scoped>
.quota-tip {
  font-size: 12px;
  margin-top: -8px;
}
.cache-ttl-tip {
  font-size: 12px;
  margin-top: -16px;
  margin-left: 100px;
}
</style>
