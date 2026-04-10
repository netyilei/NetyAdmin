<script lang="ts" setup>
import { computed, ref, watch } from 'vue';
import { NEmpty, NInput, NPagination, NPopover, NTabPane, NTabs } from 'naive-ui';
import { getGroupedIconifyIcons } from '@/utils/iconify-icons';
import SvgIcon from './svg-icon.vue';

defineOptions({ name: 'IconifyIconSelect' });

interface Props {
  /** Selected icon */
  value: string;
  /** Icon for when nothing is selected */
  emptyIcon?: string;
}

const props = withDefaults(defineProps<Props>(), {
  emptyIcon: 'mdi:apps'
});

interface Emits {
  (e: 'update:value', val: string): void;
}

const emit = defineEmits<Emits>();

const modelValue = computed({
  get() {
    return props.value;
  },
  set(val: string) {
    emit('update:value', val);
  }
});

const selectedIcon = computed(() => modelValue.value || props.emptyIcon);

const groupedIcons = getGroupedIconifyIcons();
const activeTab = ref(groupedIcons[0].prefix);
const searchValue = ref('');
const currentPage = ref(1);
const pageSize = 72; // 9 columns * 8 rows

// Reset page when tab or search changes
watch([activeTab, searchValue], () => {
  currentPage.value = 1;
});

const filteredIcons = computed(() => {
  if (!searchValue.value) {
    const tab = groupedIcons.find(g => g.prefix === activeTab.value);
    return tab ? tab.icons : [];
  }
  // Global search across all collections
  return groupedIcons
    .flatMap(g => g.icons)
    .filter(icon => icon.toLowerCase().includes(searchValue.value.toLowerCase()));
});

const displayIcons = computed(() => {
  const start = (currentPage.value - 1) * pageSize;
  const end = start + pageSize;
  return filteredIcons.value.slice(start, end);
});

function handleChange(iconItem: string) {
  modelValue.value = iconItem;
}
</script>

<template>
  <NPopover placement="bottom" trigger="click" :width="700" scrollable :z-index="5000">
    <template #trigger>
      <NInput v-model:value="modelValue" readonly :placeholder="$t('icon.select.placeholder')">
        <template #suffix>
          <SvgIcon :icon="selectedIcon" class="p-5px text-30px" />
        </template>
      </NInput>
    </template>
    <template #header>
      <div class="p-8px pb-0">
        <NInput v-model:value="searchValue" :placeholder="$t('icon.select.searchPlaceholder')" clearable />
      </div>
    </template>

    <div class="h-450px flex overflow-hidden">
      <!-- Sidebar Categories -->
      <div v-if="!searchValue" class="w-160px flex-shrink-0 border-r border-gray-100">
        <NTabs v-model:value="activeTab" type="line" placement="left" class="h-full">
          <NTabPane
            v-for="group in groupedIcons"
            :key="group.prefix"
            :name="group.prefix"
            :tab="$t(`icon.select.collection.${group.prefix}`)"
          />
        </NTabs>
      </div>

      <!-- Main Content -->
      <div class="flex flex-col flex-1 overflow-hidden">
        <div v-if="searchValue" class="border-b border-gray-50 px-12px py-8px text-12px text-gray-400">
          {{ $t('icon.select.searchResult', { count: filteredIcons.length }) }}
        </div>

        <div class="flex-1 overflow-y-auto bg-gray-50/30 p-12px">
          <div v-if="displayIcons.length > 0" class="grid grid-cols-10 gap-8px">
            <div
              v-for="iconItem in displayIcons"
              :key="iconItem"
              class="aspect-square flex cursor-pointer items-center justify-center border-1px border-transparent rounded-8px bg-white/50 transition-all hover:border-primary hover:bg-white hover:shadow-sm"
              :class="{ '!border-primary !bg-white shadow-sm': modelValue === iconItem }"
              :title="iconItem"
              @click="handleChange(iconItem)"
            >
              <SvgIcon :icon="iconItem" class="text-24px" />
            </div>
          </div>
          <NEmpty v-else :description="$t('icon.select.empty')" class="mt-40px" />
        </div>

        <div v-if="filteredIcons.length > pageSize" class="flex justify-center border-t border-gray-100 bg-white p-8px">
          <NPagination
            v-model:page="currentPage"
            :page-size="pageSize"
            :item-count="filteredIcons.length"
            size="small"
            simple
          />
        </div>
      </div>
    </div>
  </NPopover>
</template>

<style lang="scss" scoped>
:deep(.n-input-wrapper) {
  padding-right: 0;
}

:deep(.n-input__suffix) {
  border: 1px solid #d9d9d9;
}

:deep(.n-tabs-nav) {
  padding: 0 4px;
}
</style>
