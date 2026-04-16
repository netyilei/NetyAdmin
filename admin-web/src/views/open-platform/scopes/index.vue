<script setup lang="tsx">
import { computed, reactive, ref } from 'vue';
import { NButton, NCard, NDataTable, NForm, NFormItem, NInput, NModal, NPopconfirm, NSpace, NTag } from 'naive-ui';
import { addScopeGroup, deleteScopeGroup, fetchScopeGroupList, updateScopeGroup } from '@/service/api/v1/open-app';
import { useNaiveForm } from '@/hooks/common/form';
import { $t } from '@/locales';
import { useDict } from '@/hooks/common/dict';
import type { OpenApp } from '@/typings/api/v1/open-app';

const { renderDictTag } = useDict();
const { formRef, validate, restoreValidation } = useNaiveForm();

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
    key: 'i18nKey',
    title: 'I18n Key',
    align: 'center',
    width: 200,
    render: (row: OpenApp.ScopeGroup) => <NTag size="small">{row.i18nKey}</NTag>
  },
  {
    key: 'label',
    title: '当前显示',
    align: 'center',
    width: 150,
    render: (row: OpenApp.ScopeGroup) => <span>{$t(row.i18nKey as any) || row.name}</span>
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
    width: 150,
    render: (row: OpenApp.ScopeGroup) => (
      <NSpace justify="center">
        <NButton type="primary" ghost size="small" onClick={() => handleEdit(row)}>
          {$t('common.edit')}
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

// --- Modal ---
const visible = ref(false);
const operateType = ref<'add' | 'edit'>('add');
const model = reactive({
  id: 0,
  code: '',
  name: '',
  i18nKey: '',
  description: '',
  status: 1
});

const title = computed(() => (operateType.value === 'add' ? $t('common.add') : $t('common.edit')));

function handleAdd() {
  operateType.value = 'add';
  Object.assign(model, { id: 0, code: '', name: '', i18nKey: '', description: '', status: 1 });
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

getData();
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <NCard :title="$t('page.openPlatform.scope.title')" :bordered="false" size="small" class="sm:flex-1-hidden card-wrapper">
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
        <NFormItem label="默认名称" path="name">
          <NInput v-model:value="model.name" placeholder="降级显示名称" />
        </NFormItem>
        <NFormItem label="I18n Key" path="i18nKey">
          <NInput v-model:value="model.i18nKey" placeholder="例如: page.openPlatform.scope.userBase" />
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
  </div>
</template>

<style scoped></style>
