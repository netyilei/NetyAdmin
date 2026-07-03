<script setup lang="tsx">
import { ref } from 'vue';
import { NButton, NCard, NDataTable, NPopconfirm, NTag } from 'naive-ui';
import {
  fetchDeleteDictData,
  fetchDeleteDictType,
  fetchGetDictData,
  fetchGetDictDataList,
  fetchGetDictTypeList
} from '@/service/api/v1/system-dict';
import { useDictStore } from '@/store/modules/dict';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useDict } from '@/hooks/common/dict';
import type { SystemDict } from '@/typings/api/v1/system-dict';
import { $t } from '@/locales';
import DictTypeModal from './components/dict-type-modal.vue';
import DictDataModal from './components/dict-data-modal.vue';

defineOptions({ name: 'ManageDict' });

const { loadDicts, renderDictTag } = useDict();
loadDicts(['sys_status']);

const selectedDictCode = ref('');
const selectedDictName = ref('');

const {
  columns: typeColumns,
  data: typeData,
  loading: typeLoading,
  getData: getTypeData,
  mobilePagination: typePagination
} = useTable({
  apiFn: fetchGetDictTypeList,
  apiParams: { current: 1, size: 20, name: null, code: null, status: null },
  showTotal: true,
  columns: () => [
    {
      key: 'name',
      title: $t('page.manage.dict.typeName'),
      minWidth: 100,
      ellipsis: { tooltip: true }
    },
    {
      key: 'code',
      title: $t('page.manage.dict.typeCode'),
      minWidth: 100,
      ellipsis: { tooltip: true }
    },
    {
      key: 'status',
      title: $t('common.status'),
      align: 'center',
      width: 100,
      render: row => renderDictTag('sys_status', row.status ?? '')
    },
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 120,
      render: row => (
        <div class="flex-center gap-8px" onClick={e => e.stopPropagation()}>
          <NButton type="primary" ghost size="small" onClick={() => handleEditType((row as SystemDict.DictType).id)}>
            {$t('common.edit')}
          </NButton>
          <NPopconfirm onPositiveClick={() => handleDeleteType((row as SystemDict.DictType).id)}>
            {{
              default: () => $t('common.confirmDelete'),
              trigger: () => (
                <NButton type="error" ghost size="small">
                  {$t('common.delete')}
                </NButton>
              )
            }}
          </NPopconfirm>
        </div>
      )
    }
  ]
});

const {
  columns: dataColumns,
  data: dictData,
  loading: dataLoading,
  mobilePagination: dataPagination
} = useTable({
  apiFn: fetchGetDictDataList,
  apiParams: { current: 1, size: 20, dictCode: null, label: null, status: null },
  showTotal: true,
  immediate: false,
  columns: () => [
    {
      key: 'label',
      title: $t('page.manage.dict.dataLabel'),
      minWidth: 80,
      ellipsis: { tooltip: true }
    },
    {
      key: 'value',
      title: $t('page.manage.dict.dataValue'),
      minWidth: 60,
      ellipsis: { tooltip: true }
    },
    {
      key: 'tagType',
      title: $t('page.manage.dict.tagType'),
      align: 'center',
      width: 100,
      render: row => (
        <NTag type={(row as SystemDict.DictData).tagType} size="small">
          {(row as SystemDict.DictData).tagType}
        </NTag>
      )
    },
    {
      key: 'orderBy',
      title: $t('page.manage.dict.orderBy'),
      align: 'center',
      width: 70
    },
    {
      key: 'status',
      title: $t('common.status'),
      align: 'center',
      width: 100,
      render: row => renderDictTag('sys_status', row.status ?? '')
    },
    {
      key: 'remark',
      title: $t('page.manage.dict.remark'),
      minWidth: 120,
      ellipsis: { tooltip: true }
    },
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 140,
      render: row => (
        <div class="flex-center gap-8px">
          <NButton type="primary" ghost size="small" onClick={() => handleEditData((row as SystemDict.DictData).id)}>
            {$t('common.edit')}
          </NButton>
          <NPopconfirm onPositiveClick={() => handleDeleteData((row as SystemDict.DictData).id)}>
            {{
              default: () => $t('common.confirmDelete'),
              trigger: () => (
                <NButton type="error" ghost size="small">
                  {$t('common.delete')}
                </NButton>
              )
            }}
          </NPopconfirm>
        </div>
      )
    }
  ]
});

function handleRowClick(row: any) {
  handleSelectType(row as SystemDict.DictType);
}

const typeRowProps = (row: any) => {
  return {
    style: 'cursor: pointer;',
    onClick: () => {
      handleRowClick(row);
    }
  };
};

// 字典类型表操作（统一走 useTableOperate，与项目其余列表视图保持一致）
const {
  drawerVisible: typeModalVisible,
  operateType: typeModalMode,
  editingData: editingType,
  handleAdd: handleAddTypeBase,
  handleEdit: handleEditTypeBase,
  onDeleted: onTypeDeleted
} = useTableOperate<SystemDict.DictType>(typeData, getTypeData);

// 字典数据表操作
const {
  drawerVisible: dataModalVisible,
  operateType: dataModalMode,
  editingData,
  handleAdd: handleAddDataBase,
  handleEdit: handleEditDataBase,
  onDeleted: onDataDeleted
} = useTableOperate<SystemDict.DictData>(dictData, async () => {
  await loadDictData(selectedDictCode.value);
});

// 列渲染中调用的句柄通过函数声明提升，避免 use-before-define
function handleEditType(id: SystemDict.DictType['id']) {
  handleEditTypeBase(id);
}
function handleEditData(id: SystemDict.DictData['id']) {
  handleEditDataBase(id);
}

function handleSelectType(row: SystemDict.DictType) {
  selectedDictCode.value = row.code;
  selectedDictName.value = row.name;
  loadDictData(row.code);
}

async function loadDictData(code: string) {
  dictData.value = [];
  const res: any = await fetchGetDictData(code);
  if (res.data) {
    dictData.value = res.data;
  }
}

/** 新增字典类型 */
function handleAddType() {
  handleAddTypeBase();
}

/** 新增字典数据前校验：必须先选中字典类型 */
function handleAddData() {
  if (!selectedDictCode.value) {
    window.$message?.warning($t('page.manage.dict.selectTypeFirst'));
    return;
  }
  handleAddDataBase();
}

async function handleDeleteType(id: number) {
  const { error } = await fetchDeleteDictType(id);
  if (!error) {
    const dictStore = useDictStore();
    dictStore.clearCache();
    await onTypeDeleted();
  }
}

async function handleDeleteData(id: number) {
  const { error } = await fetchDeleteDictData(id);
  if (!error) {
    const dictStore = useDictStore();
    dictStore.removeDict(selectedDictCode.value);
    await onDataDeleted();
  }
}

async function handleTypeSubmitted() {
  const dictStore = useDictStore();
  dictStore.clearCache();
  await getTypeData();
}

async function handleDataSubmitted() {
  const dictStore = useDictStore();
  dictStore.removeDict(selectedDictCode.value);
  await loadDictData(selectedDictCode.value);
}
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <div class="h-full flex gap-16px">
      <NCard
        :title="$t('page.manage.dict.typeTitle')"
        :bordered="false"
        size="small"
        class="w-500px flex-shrink-0 card-wrapper"
      >
        <template #header-extra>
          <NButton type="primary" size="small" ghost @click="handleAddType">
            {{ $t('common.add') }}
          </NButton>
        </template>
        <NDataTable
          :columns="(typeColumns as any)"
          :data="typeData"
          :loading="typeLoading"
          size="small"
          :pagination="typePagination"
          :row-key="row => row.id"
          :row-props="typeRowProps"
          :row-class-name="(row: any) => row.code === selectedDictCode ? 'n-data-table-tr--selected' : ''"
          class="sm:h-full"
        />
      </NCard>

      <NCard
        :bordered="false"
        size="small"
        class="flex-1-hidden card-wrapper"
        :title="
          selectedDictCode
            ? `${$t('page.manage.dict.dataTitle')} - ${selectedDictName} [${selectedDictCode}]`
            : $t('page.manage.dict.dataTitle')
        "
      >
        <template #header-extra>
          <NButton type="primary" size="small" ghost :disabled="!selectedDictCode" @click="handleAddData">
            {{ $t('common.add') }}
          </NButton>
        </template>
        <div v-if="!selectedDictCode" class="h-200px flex-center text-gray-400">
          {{ $t('page.manage.dict.selectTypeFirst') }}
        </div>
        <NDataTable
          v-else
          :columns="(dataColumns as any)"
          :data="dictData"
          :loading="dataLoading"
          size="small"
          :pagination="dataPagination"
          :row-key="row => row.id"
          class="sm:h-full"
        />
      </NCard>
    </div>

    <DictTypeModal
      v-model:visible="typeModalVisible"
      :mode="typeModalMode"
      :row-data="editingType"
      @submitted="handleTypeSubmitted"
    />
    <DictDataModal
      v-model:visible="dataModalVisible"
      :mode="dataModalMode"
      :row-data="editingData"
      :dict-code="selectedDictCode"
      @submitted="handleDataSubmitted"
    />
  </div>
</template>

<style scoped>
:deep(.n-data-table-tr--selected td) {
  background-color: var(--n-td-color-hover) !important;
}
</style>
