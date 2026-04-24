<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue';
import {
  fetchGetButtonIdsByRole,
  fetchGetButtonTree,
  fetchUpdateButtonIdsByRole
} from '@/service/api/v1/system-manage';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

defineOptions({
  name: 'ButtonAuthModal'
});

interface Props {
  /** the roleId */
  roleId: number;
}

const props = defineProps<Props>();
const loading = ref(false);

const visible = defineModel<boolean>('visible', {
  default: false
});

function closeModal() {
  visible.value = false;
}

const title = computed(() => $t('common.edit') + $t('page.manage.role.buttonAuth'));

const tree = shallowRef<SystemManage.MenuButtonTree[]>([]);

async function getAllButtons() {
  loading.value = true;
  const { error, data } = await fetchGetButtonTree();
  if (error) {
    tree.value = [];
  } else {
    tree.value = data;
  }
  loading.value = false;
}

const checks = shallowRef<string[]>([]);

async function getChecks() {
  loading.value = true;
  const { error, data } = await fetchGetButtonIdsByRole(props.roleId);
  if (error) {
    checks.value = [];
  } else {
    // Transform numeric button IDs to string IDs with 'b_' prefix
    checks.value = data.map(id => `b_${id}`);
  }
  loading.value = false;
}

async function handleSubmit() {
  // Extract number IDs from 'b_{id}' strings
  const buttonIds = checks.value.filter(key => key.startsWith('b_')).map(key => Number(key.replace('b_', '')));

  loading.value = true;
  const { error } = await fetchUpdateButtonIdsByRole(props.roleId, buttonIds);
  loading.value = false;
  if (!error) {
    window.$message?.success?.($t('common.modifySuccess'));
    closeModal();
  }
}

watch(visible, val => {
  if (val) {
    getAllButtons();
    getChecks();
  }
});
</script>

<template>
  <NModal v-model:show="visible" :loading :title="title" preset="card" class="w-480px">
    <NTree
      v-model:checked-keys="checks"
      :data="tree"
      key-field="id"
      block-line
      checkable
      expand-on-click
      virtual-scroll
      class="h-280px"
    />
    <template #footer>
      <NSpace justify="end">
        <NButton size="small" class="mt-16px" @click="closeModal">
          {{ $t('common.cancel') }}
        </NButton>
        <NButton type="primary" size="small" class="mt-16px" @click="handleSubmit">
          {{ $t('common.confirm') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
