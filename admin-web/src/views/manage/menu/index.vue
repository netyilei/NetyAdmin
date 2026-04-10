<script setup lang="tsx">
import { computed, onMounted, ref } from 'vue';
import type { Ref } from 'vue';
import { NButton, NDataTable, NPopconfirm, NTag } from 'naive-ui';
import { useBoolean } from '@sa/hooks';
import TableHeaderOperation from '@/components/advanced/table-header-operation.vue';
import {
  fetchBatchDeleteMenu,
  fetchDeleteMenu,
  fetchGetAllPages,
  fetchGetMenu,
  fetchGetMenuList
} from '@/service/api/v1/system-manage';
import { useAppStore } from '@/store/modules/app';
import { useTable, useTableOperate } from '@/hooks/common/table';
import { useDict } from '@/hooks/common/dict';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';
import SvgIcon from '@/components/custom/svg-icon.vue';
import MenuOperateModal, { type OperateType } from './components/menu-operate-modal.vue';

const appStore = useAppStore();
const { loadDicts, renderDictTag } = useDict();

loadDicts(['sys_status', 'menu_type', 'sys_yes_no']);

const { bool: visible, setTrue: openModal } = useBoolean();

const wrapperRef = ref<HTMLElement | null>(null);

/** 将平级菜单转换为树形结构 */
function transformToTree(menus: SystemManage.Menu[]) {
  const menuMap = new Map<number, SystemManage.Menu>();
  const result: SystemManage.Menu[] = [];

  menus.forEach(menu => {
    menuMap.set(menu.id, { ...menu, children: [] });
  });

  menus.forEach(menu => {
    const menuWithChildren = menuMap.get(menu.id)!;
    if (menu.parentId === 0) {
      result.push(menuWithChildren);
    } else {
      const parent = menuMap.get(menu.parentId);
      if (parent) {
        parent.children ||= [];
        parent.children.push(menuWithChildren);
      }
    }
  });

  return result;
}

const { columns, columnChecks, data, loading, pagination, getData, updateSearchParams } = useTable({
  apiFn: fetchGetMenuList,
  apiParams: {
    current: 1,
    size: 100
  },
  columns: () => [
    {
      type: 'selection',
      align: 'center',
      width: 48
    },
    {
      key: 'id',
      title: $t('page.manage.menu.id'),
      align: 'center',
      width: 60
    },
    {
      key: 'type',
      title: $t('page.manage.menu.menuType'),
      align: 'center',
      width: 80,
      render: row => renderDictTag('menu_type', row.type ?? '')
    },
    {
      key: 'name',
      title: $t('page.manage.menu.menuName'),
      minWidth: 150,
      render: row => {
        const { i18nKey, name } = row;

        const label = i18nKey ? $t(i18nKey) : name;

        return <span>{label}</span>;
      }
    },
    {
      key: 'icon',
      title: $t('page.manage.menu.icon'),
      align: 'center',
      width: 60,
      render: row => {
        const icon = row.iconType === '1' ? row.icon : undefined;

        const localIcon = row.iconType === '2' ? row.icon : undefined;

        return (
          <div class="flex-center">
            <SvgIcon icon={icon} localIcon={localIcon} class="text-icon" />
          </div>
        );
      }
    },
    {
      key: 'routeName',
      title: $t('page.manage.menu.routeName'),
      align: 'center',
      minWidth: 120
    },
    {
      key: 'routePath',
      title: $t('page.manage.menu.routePath'),
      align: 'center',
      minWidth: 120
    },
    {
      key: 'status',
      title: $t('page.manage.menu.menuStatus'),
      align: 'center',
      width: 80,
      render: row => renderDictTag('sys_status', row.status ?? '')
    },
    {
      key: 'hideInMenu',
      title: $t('page.manage.menu.hideInMenu'),
      align: 'center',
      width: 80,
      render: row => renderDictTag('sys_yes_no', row.hideInMenu ? '1' : '0')
    },
    {
      key: 'order',
      title: $t('page.manage.menu.order'),
      align: 'center',
      width: 60
    },
    {
      key: 'operate',
      title: $t('common.operate'),
      align: 'center',
      width: 230,
      render: row => (
        <div class="flex-center justify-end gap-8px">
          {row.type === '1' && (
            <NButton type="primary" ghost size="small" onClick={() => handleAddChildMenu(row)}>
              {$t('page.manage.menu.addChildMenu')}
            </NButton>
          )}
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
        </div>
      )
    }
  ]
});

// 转换为树形数据
const treeData = computed(() => {
  return transformToTree(data.value as SystemManage.Menu[]);
});

// 设置分页大小
Object.assign(pagination, { pageSize: 100 });

const { checkedRowKeys, onBatchDeleted, onDeleted } = useTableOperate(data, getData);

const operateType = ref<OperateType>('add');

function handleAdd() {
  operateType.value = 'add';
  openModal();
}

async function handleBatchDelete() {
  const ids = checkedRowKeys.value as unknown as number[];
  if (!ids.length) return;

  loading.value = true;
  const { error } = await fetchBatchDeleteMenu(ids);
  loading.value = false;
  if (!error) {
    await onBatchDeleted();
  }
}

async function handleDelete(id: number) {
  loading.value = true;
  const { error } = await fetchDeleteMenu(id);
  loading.value = false;
  if (!error) {
    await onDeleted();
  }
}

/** the edit menu data or the parent menu data when adding a child menu */
const editingData: Ref<SystemManage.Menu | null> = ref(null);

async function handleEdit(item: SystemManage.Menu) {
  operateType.value = 'edit';
  const { data: menuData } = await fetchGetMenu(item.id);
  editingData.value = menuData || { ...item };

  openModal();
}

function handleAddChildMenu(item: SystemManage.Menu) {
  operateType.value = 'addChild';

  editingData.value = { ...item };

  openModal();
}

const allPages = ref<string[]>([]);

async function getAllPages() {
  const { data: pages } = await fetchGetAllPages();
  allPages.value = pages || [];
}

async function handleSubmit(_?: SystemManage.Menu) {
  await getData();
}

async function loadMenuData() {
  updateSearchParams({ current: 1, size: 100 });
  await getData();
}

function init() {
  loadDicts(['menu_type', 'sys_status']);
  getAllPages();
  loadMenuData();
}

// init
onMounted(() => {
  init();
});
</script>

<template>
  <div ref="wrapperRef" class="flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <NCard :title="$t('page.manage.menu.title')" :bordered="false" size="small" class="card-wrapper sm:flex-1-hidden">
      <template #header-extra>
        <TableHeaderOperation
          v-model:columns="columnChecks"
          :disabled-delete="checkedRowKeys.length === 0"
          :loading="loading"
          @add="handleAdd"
          @delete="handleBatchDelete"
          @refresh="loadMenuData"
        />
      </template>
      <NDataTable
        v-model:checked-row-keys="checkedRowKeys"
        :columns="columns"
        :data="treeData"
        size="small"
        :flex-height="!appStore.isMobile"
        :scroll-x="1088"
        :loading="loading"
        :row-key="row => row.id"
        :indent="24"
        :pagination="pagination"
        class="sm:h-full"
      />
      <MenuOperateModal
        v-model:visible="visible"
        :operate-type="operateType"
        :row-data="editingData"
        :all-pages="allPages"
        @submitted="handleSubmit"
      />
    </NCard>
  </div>
</template>

<style scoped></style>
