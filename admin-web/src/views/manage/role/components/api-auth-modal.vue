<script setup lang="tsx">
import { computed, ref, shallowRef, watch } from 'vue';
import { NTag } from 'naive-ui';
import { fetchGetApiIdsByRole, fetchGetApiTree, fetchUpdateApiIdsByRole } from '@/service/api/v1/system-manage';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

defineOptions({
  name: 'ApiAuthModal'
});

interface Props {
  /** the roleId */
  roleId: number;
}

const loading = ref(false);

const props = defineProps<Props>();

const visible = defineModel<boolean>('visible', {
  default: false
});

function closeModal() {
  visible.value = false;
}

const title = computed(() => $t('common.edit') + $t('page.manage.role.apiAuth'));

const treeData = shallowRef<SystemManage.MenuApiTree[]>([]);

const checks = shallowRef<string[]>([]);

async function getTreeData() {
  loading.value = true;
  const { error, data } = await fetchGetApiTree();
  treeData.value = !error ? data : [];
  loading.value = false;
}

async function getChecks() {
  loading.value = true;
  const { error, data } = await fetchGetApiIdsByRole(props.roleId);
  // Add 'a_' prefix for API IDs to match tree structure
  checks.value = !error ? data.map(id => `a_${id}`) : [];
  loading.value = false;
}

async function handleSubmit() {
  loading.value = true;
  // Extract only API IDs (stripping 'a_' prefix and ignoring 'm_' menu IDs)
  const apiIds = checks.value.filter(key => key.startsWith('a_')).map(key => Number(key.replace('a_', '')));

  const { error } = await fetchUpdateApiIdsByRole(props.roleId, apiIds);
  loading.value = false;
  if (!error) {
    window.$message?.success?.($t('common.modifySuccess'));
    closeModal();
  }
}

function renderLabel({ option }: { option: any }) {
  const node = option as SystemManage.MenuApiTree;
  if (node.type === 'api') {
    const color = getMethodColor(node.method || '');
    return (
      <div class="flex-y-center gap-8px">
        <NTag type={color} size="small" bordered={false}>
          {node.method}
        </NTag>
        <span class="text-12px text-warm-gray-400">{node.path}</span>
        <span class="ml-4px">{node.label}</span>
      </div>
    );
  }
  return <span>{node.label}</span>;
}

watch(visible, val => {
  if (val) {
    getTreeData();
    getChecks();
  }
});

function getMethodColor(method: string) {
  switch (method) {
    case 'GET':
      return 'info';
    case 'POST':
      return 'success';
    case 'PUT':
      return 'warning';
    case 'DELETE':
      return 'error';
    default:
      return 'default';
  }
}
</script>

<template>
  <NModal v-model:show="visible" :title="title" :loading="loading" preset="card" class="w-800px">
    <div class="flex-col gap-16px">
      <div class="max-h-480px overflow-y-auto pr-12px">
        <NTree
          v-model:checked-keys="checks"
          block-line
          cascade
          checkable
          expand-on-click
          :data="treeData"
          label-field="label"
          key-field="id"
          :render-label="renderLabel"
          default-expand-all
        />
      </div>
    </div>
    <template #footer>
      <NSpace justify="end">
        <NButton size="small" class="mt-16px" @click="closeModal">
          {{ $t('common.cancel') }}
        </NButton>
        <NButton type="primary" size="small" :loading="loading" class="mt-16px" @click="handleSubmit">
          {{ $t('common.confirm') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
:deep(.n-tree-node-content__label) {
  width: 100%;
}
</style>
