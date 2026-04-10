<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue';
import { consola } from 'consola';
import {
  fetchGetMenuIdsByRole,
  fetchGetMenuTree,
  fetchUpdateMenuIdsByRole
} from '@/service/api/v1/system-manage';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

defineOptions({
  name: 'MenuAuthModal'
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

const title = computed(() => $t('common.edit') + $t('page.manage.role.menuAuth'));

const tree = shallowRef<SystemManage.MenuTree[]>([]);

async function getTree() {
  const { error, data } = await fetchGetMenuTree();

  if (!error) {
    tree.value = data;
  }
}

const checks = shallowRef<number[]>([]);

async function getChecks() {
  loading.value = true;
  const { error, data } = await fetchGetMenuIdsByRole(props.roleId);
  if (!error) {
    checks.value = data.menuIds;
  } else {
    checks.value = [];
  }
  loading.value = false;
}

async function handleSubmit() {
  consola.log(checks.value, props.roleId);
  loading.value = true;
  const { error } = await fetchUpdateMenuIdsByRole(props.roleId, {
    menuIds: checks.value,
    homeRouteName: ''
  });
  loading.value = false;
  if (!error) {
    window.$message?.success?.($t('common.modifySuccess'));
    closeModal();
  }
}

watch(visible, val => {
  if (val) {
    getTree();
    getChecks();
  }
});
</script>

<template>
  <NModal v-model:show="visible" :title="title" :loading preset="card" class="w-480px">
    <NTree
      v-model:checked-keys="checks"
      :data="tree"
      key-field="id"
      checkable
      expand-on-click
      virtual-scroll
      block-line
      class="h-280px"
    />
    <template #footer>
      <NSpace justify="end">
        <NButton size="small" class="mt-16px" @click="closeModal">
          {{ $t('common.cancel') }}
        </NButton>
        <NButton type="primary" size="small" :loading class="mt-16px" @click="handleSubmit">
          {{ $t('common.confirm') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
