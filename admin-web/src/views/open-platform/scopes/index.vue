<script setup lang="tsx">
import { computed, reactive, ref } from 'vue';
import {
  NButton,
  NCard,
  NCheckbox,
  NCheckboxGroup,
  NCollapse,
  NCollapseItem,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSpace,
  NSpin,
  NTag
} from 'naive-ui';
import { addScopeGroup, deleteScopeGroup, fetchScopeGroupList, updateScopeGroup } from '@/service/api/v1/open-app';
import { fetchGroupedApis, fetchScopeApis, updateScopeApis } from '@/service/api/v1/open-api';
import { useNaiveForm } from '@/hooks/common/form';
import { useDict } from '@/hooks/common/dict';
import { $t } from '@/locales';
import type { OpenApp } from '@/typings/api/v1/open-app';
import type { OpenApi } from '@/typings/api/v1/open-api';

const { renderDictTag } = useDict();
const { formRef, validate } = useNaiveForm();

const loading = ref(false);
const data = ref<OpenApp.ScopeGroup[]>([]);

async function getData() {
  loading.value = true;
  const { data: res, error } = await fetchScopeGroupList();
  loading.value = false;
  if (!error && res) {
    data.value = res;
  }
}

const columns = [
  { key: 'id', title: 'ID', align: 'center', width: 80 },
  { key: 'code', title: $t('page.openPlatform.scope.name'), align: 'center', width: 150 },
  {
    key: 'name',
    title: $t('page.openPlatform.scope.displayName'),
    align: 'center',
    width: 150,
    render: (row: OpenApp.ScopeGroup) => <span>{row.name}</span>
  },
  {
    key: 'status',
    title: $t('page.openPlatform.app.status'),
    align: 'center',
    width: 100,
    render: (row: OpenApp.ScopeGroup) => renderDictTag('sys_status', String(row.status))
  },
  {
    key: 'operate',
    title: $t('common.operate'),
    align: 'center',
    width: 250,
    render: (row: OpenApp.ScopeGroup) => (
      <NSpace justify="center">
        <NButton type="primary" ghost size="small" onClick={() => handleEdit(row)}>
          {$t('common.edit')}
        </NButton>
        <NButton type="info" ghost size="small" onClick={() => handleBindApis(row)}>
          {$t('page.openPlatform.scope.bindApis')}
        </NButton>
        <NPopconfirm onPositiveClick={() => handleDelete(row.id)}>
          {{
            default: () => $t('common.confirmDelete'),
            trigger: () => (
              <NButton type="error" ghost size="small">
                {$t('common.delete')}
              </NButton>
            )
          }}
        </NPopconfirm>
      </NSpace>
    )
  }
] as any[];

const visible = ref(false);
const operateType = ref<'add' | 'edit'>('add');
const model = reactive({
  id: 0,
  code: '',
  name: '',
  description: '',
  status: 1
});

const title = computed(() => (operateType.value === 'add' ? $t('common.add') : $t('common.edit')));

function handleAdd() {
  operateType.value = 'add';
  Object.assign(model, { id: 0, code: '', name: '', description: '', status: 1 });
  visible.value = true;
}

function handleEdit(row: OpenApp.ScopeGroup) {
  operateType.value = 'edit';
  Object.assign(model, row);
  visible.value = true;
}

async function handleDelete(id: number) {
  const { error } = await deleteScopeGroup(id);
  if (!error) {
    window.$message?.success($t('common.deleteSuccess'));
    getData();
  }
}

async function handleSubmit() {
  await validate();
  const { error } = operateType.value === 'add' ? await addScopeGroup(model) : await updateScopeGroup(model);
  if (!error) {
    window.$message?.success(operateType.value === 'add' ? $t('common.addSuccess') : $t('common.updateSuccess'));
    visible.value = false;
    getData();
  }
}

const apiBindVisible = ref(false);
const apiBindLoading = ref(false);
const currentScopeId = ref(0);
const currentScopeName = ref('');
const groupedApis = ref<OpenApi.GroupedApi[]>([]);
const selectedApiIds = ref<number[]>([]);

const methodColorMap: Record<string, 'default' | 'primary' | 'info' | 'success' | 'warning' | 'error'> = {
  GET: 'success',
  POST: 'info',
  PUT: 'warning',
  DELETE: 'error',
  PATCH: 'default'
};

function toggleApiSelection(apiId: number) {
  const newSelected = [...selectedApiIds.value];
  const idx = newSelected.indexOf(apiId);
  if (idx >= 0) {
    newSelected.splice(idx, 1);
  } else {
    newSelected.push(apiId);
  }
  selectedApiIds.value = newSelected;
}

function handleToggleGroup(apis: OpenApi.ApiItem[]) {
  const apiIds = apis.map(api => api.id);
  const allSelected = apiIds.every(id => selectedApiIds.value.includes(id));

  if (allSelected) {
    // Unselect all in this group
    selectedApiIds.value = selectedApiIds.value.filter(id => !apiIds.includes(id));
  } else {
    // Select all in this group
    const newSelected = [...selectedApiIds.value];
    apiIds.forEach(id => {
      if (!newSelected.includes(id)) {
        newSelected.push(id);
      }
    });
    selectedApiIds.value = newSelected;
  }
}

function isGroupAllSelected(apis: OpenApi.ApiItem[]) {
  return apis.length > 0 && apis.every(api => selectedApiIds.value.includes(api.id));
}

function isGroupIndeterminate(apis: OpenApi.ApiItem[]) {
  const selectedCount = apis.filter(api => selectedApiIds.value.includes(api.id)).length;
  return selectedCount > 0 && selectedCount < apis.length;
}

async function handleBindApis(row: OpenApp.ScopeGroup) {
  currentScopeId.value = row.id;
  currentScopeName.value = row.name;
  apiBindLoading.value = true;
  apiBindVisible.value = true;

  const [groupedRes, scopeRes] = await Promise.all([fetchGroupedApis(), fetchScopeApis(row.id)]);
  apiBindLoading.value = false;

  if (groupedRes.data) {
    groupedApis.value = groupedRes.data;
  }
  if (scopeRes.data) {
    selectedApiIds.value = scopeRes.data.map((api: OpenApi.Api) => api.id);
  }
}

async function handleSaveApis() {
  apiBindLoading.value = true;
  const { error } = await updateScopeApis({
    scopeId: currentScopeId.value,
    apiIds: selectedApiIds.value
  });
  apiBindLoading.value = false;
  if (!error) {
    window.$message?.success($t('common.updateSuccess'));
    apiBindVisible.value = false;
  }
}

getData();
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <NCard
      :title="$t('page.openPlatform.scope.title')"
      :bordered="false"
      size="small"
      class="card-wrapper sm:flex-1-hidden"
    >
      <template #header-extra>
        <NSpace>
          <NButton type="primary" size="small" @click="handleAdd">
            <template #icon>
              <icon-ic-round-add class="text-icon" />
            </template>
            {{ $t('common.add') }}
          </NButton>
          <NButton size="small" @click="getData">
            <template #icon>
              <icon-ic-round-refresh class="text-icon" />
            </template>
            {{ $t('common.refresh') }}
          </NButton>
        </NSpace>
      </template>
      <NDataTable
        remote
        striped
        size="small"
        class="sm:flex-1-hidden"
        :data="data"
        :columns="columns"
        :loading="loading"
        :single-line="false"
        :row-key="row => row.id"
      />
    </NCard>

    <NModal v-model:show="visible" :title="title" preset="card" class="w-500px">
      <NForm ref="formRef" :model="model" label-placement="left" :label-width="100">
        <NFormItem :label="$t('page.openPlatform.scope.name')" path="code">
          <NInput v-model:value="model.code" placeholder="例如: user_base" :disabled="operateType === 'edit'" />
        </NFormItem>
        <NFormItem :label="$t('page.openPlatform.scope.displayName')" path="name">
          <NInput v-model:value="model.name" :placeholder="$t('page.openPlatform.scope.form.displayNamePlaceholder')" />
        </NFormItem>
        <NFormItem :label="$t('page.openPlatform.app.status')" path="status">
          <AppDictSelect v-model:value="model.status" dict-code="sys_status" />
        </NFormItem>
        <NFormItem :label="$t('page.openPlatform.app.remark')" path="description">
          <NInput v-model:value="model.description" type="textarea" />
        </NFormItem>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="visible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
        </NSpace>
      </template>
    </NModal>

    <NModal
      v-model:show="apiBindVisible"
      :title="$t('page.openPlatform.scope.bindApis') + ' - ' + currentScopeName"
      preset="card"
      class="w-800px"
    >
      <NSpin :show="apiBindLoading">
        <NCollapse>
          <NCollapseItem v-for="group in groupedApis" :key="group.group" :name="group.group">
            <template #header>
              <NSpace align="center" @click.stop>
                <NCheckbox
                  :checked="isGroupAllSelected(group.apis)"
                  :indeterminate="isGroupIndeterminate(group.apis)"
                  @update:checked="handleToggleGroup(group.apis)"
                />
                <span class="font-bold">{{ group.group }}</span>
                <NTag size="small" :bordered="false" round type="primary">
                  {{ group.apis.filter(api => selectedApiIds.includes(api.id)).length }} / {{ group.apis.length }}
                </NTag>
              </NSpace>
            </template>
            <NCheckboxGroup v-model:value="selectedApiIds">
              <div class="grid grid-cols-2 gap-8px pt-8px">
                <NCard
                  v-for="api in group.apis"
                  :key="api.id"
                  size="small"
                  :bordered="true"
                  class="cursor-pointer transition-all hover:border-primary"
                  :class="{ 'border-primary! bg-primary/5': selectedApiIds.includes(api.id) }"
                  @click="toggleApiSelection(api.id)"
                >
                  <NSpace align="center" justify="space-between" :wrap="false">
                    <NSpace align="center" :wrap="false" class="overflow-hidden">
                      <NCheckbox :value="api.id" @click.stop />
                      <NTag :type="methodColorMap[api.method] || 'default'" size="small">{{ api.method }}</NTag>
                      <div class="flex-col overflow-hidden">
                        <div class="truncate text-12px font-mono" :title="api.path">{{ api.path }}</div>
                        <div class="truncate text-12px text-gray-400" :title="api.name">{{ api.name }}</div>
                      </div>
                    </NSpace>
                  </NSpace>
                </NCard>
              </div>
            </NCheckboxGroup>
          </NCollapseItem>
        </NCollapse>
      </NSpin>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="apiBindVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="apiBindLoading" @click="handleSaveApis">
            {{ $t('common.confirm') }}
          </NButton>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>

<style scoped></style>
