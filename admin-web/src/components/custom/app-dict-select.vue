<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import type { SelectOption } from 'naive-ui';
import { useDictStore } from '@/store/modules/dict';

defineOptions({ name: 'AppDictSelect' });

interface Props {
  dictCode: string;
  value?: string | number | null;
  placeholder?: string;
  disabled?: boolean;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  (e: 'update:value', val: string | number | null): void;
}>();

const dictStore = useDictStore();
const loading = ref(false);

const options = computed<SelectOption[]>(() => {
  const data = dictStore.dictMap.get(props.dictCode);
  return (
    data?.map(item => ({
      label: item.label,
      value: item.value,
      tagType: item.tagType
    })) || []
  );
});

async function loadOptions() {
  if (!props.dictCode) return;
  loading.value = true;
  await dictStore.getDict(props.dictCode);
  loading.value = false;
}

function handleUpdateValue(val: string | number | null) {
  emit('update:value', val);
}

onMounted(loadOptions);

// 如果 dictCode 动态变化
watch(() => props.dictCode, loadOptions);
</script>

<template>
  <NSelect
    :value="value"
    :options="options"
    :placeholder="placeholder || '请选择'"
    :loading="loading"
    :disabled="disabled"
    clearable
    @update:value="handleUpdateValue"
  />
</template>

<style scoped></style>
