<script setup lang="ts">
import { computed, onMounted, watch } from 'vue';
import { useDictStore } from '@/store/modules/dict';
import { $t } from '@/locales';

defineOptions({ name: 'AppDictRadioGroup' });

interface Props {
  dictCode: string;
  value?: string | number | null;
  disabled?: boolean;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  (e: 'update:value', val: string | number | null): void;
}>();

const dictStore = useDictStore();

const options = computed(() => dictStore.dictMap.get(props.dictCode) || []);

async function loadOptions() {
  if (!props.dictCode) return;
  await dictStore.getDict(props.dictCode);
}

function handleUpdateValue(val: string | number | null) {
  emit('update:value', val);
}

function formatLabel(label: string) {
  return label.includes('.') ? $t(label) : label;
}

onMounted(loadOptions);
watch(() => props.dictCode, loadOptions);
</script>

<template>
  <NRadioGroup :value="value" :disabled="disabled" @update:value="handleUpdateValue">
    <NRadio v-for="item in options" :key="item.value" :value="item.value" :label="formatLabel(item.label)" />
  </NRadioGroup>
</template>
