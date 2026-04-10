<script setup lang="ts">
import { computed, onMounted, watch } from 'vue';
import { useDictStore } from '@/store/modules/dict';
import { $t } from '@/locales';

defineOptions({ name: 'AppDictRadioGroup' });

interface Props {
  dictCode: string;
  value?: string | number | boolean | null;
  disabled?: boolean;
  type?: 'string' | 'number' | 'boolean';
}

const props = withDefaults(defineProps<Props>(), {
  type: 'string'
});

const emit = defineEmits<{
  (e: 'update:value', val: string | number | boolean | null): void;
}>();

const dictStore = useDictStore();

const options = computed(() => dictStore.dictMap.get(props.dictCode) || []);

const internalValue = computed(() => {
  if (props.type === 'boolean') {
    if (props.value === null || props.value === undefined) return null;
    return props.value ? '1' : '0';
  }
  return props.value as string | number | null;
});

async function loadOptions() {
  if (!props.dictCode) return;
  await dictStore.getDict(props.dictCode);
}

function handleUpdateValue(val: string | number | null) {
  let emitVal: string | number | boolean | null = val;
  if (props.type === 'boolean') {
    if (val === null) {
      emitVal = null;
    } else {
      emitVal = val === '1';
    }
  } else if (props.type === 'number') {
    emitVal = val !== null ? Number(val) : null;
  }
  emit('update:value', emitVal);
}

function formatLabel(label: string) {
  return label.includes('.') ? $t(label) : label;
}

onMounted(loadOptions);
watch(() => props.dictCode, loadOptions);
</script>

<template>
  <NRadioGroup :value="internalValue" :disabled="disabled" @update:value="handleUpdateValue">
    <NRadio v-for="item in options" :key="item.value" :value="item.value" :label="formatLabel(item.label)" />
  </NRadioGroup>
</template>
