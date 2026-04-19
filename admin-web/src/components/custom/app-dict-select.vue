<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import type { SelectOption } from 'naive-ui';
import { useDictStore } from '@/store/modules/dict';
import { $t } from '@/locales';

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

const normalizedValue = computed(() => {
  if (props.value === null || props.value === undefined) return null;
  return String(props.value);
});

const options = computed<SelectOption[]>(() => {
  const data = dictStore.dictMap.get(props.dictCode);
  return (
    data?.map(item => ({
      label: item.label.includes('.') ? $t(item.label as any) : item.label,
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
  if (val === null || val === undefined) {
    emit('update:value', null);
    return;
  }
  const originalValue = props.value;
  if (typeof originalValue === 'number') {
    const num = Number(val);
    emit('update:value', Number.isNaN(num) ? val : num);
  } else {
    emit('update:value', String(val));
  }
}

onMounted(loadOptions);

watch(() => props.dictCode, loadOptions);
</script>

<template>
  <NSelect
    :value="normalizedValue"
    :options="options"
    :placeholder="placeholder || '请选择'"
    :loading="loading"
    :disabled="disabled"
    clearable
    @update:value="handleUpdateValue"
  />
</template>

<style scoped></style>
