<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { fetchGetAllEnabledStorageConfigs } from '@/service/api/v1/storage';
import { useRouterPush } from '@/hooks/common/router';
import { $t } from '@/locales';

defineOptions({
  name: 'StorageConfigSelect'
});

interface Props {
  value: number | null | undefined;
  placeholder?: string;
  disabled?: boolean;
}

interface Emits {
  (e: 'update:value', value: number | null | undefined): void;
}

withDefaults(defineProps<Props>(), {
  placeholder: '',
  disabled: false
});

const emit = defineEmits<Emits>();

const { routerPushByKey } = useRouterPush();

const loading = ref(false);
const options = ref<{ label: string; value: number }[]>([]);

async function getOptions() {
  loading.value = true;
  try {
    const { data, error } = await fetchGetAllEnabledStorageConfigs();
    if (!error && data) {
      options.value = data.map(item => ({
        label: `${item.name}${item.isDefault ? ` (${$t('common.default')})` : ''}`,
        value: item.id
      }));
    }
  } finally {
    loading.value = false;
  }
}

function handleUpdateValue(val: number | null) {
  emit('update:value', val);
}

onMounted(() => {
  getOptions();
});
</script>

<template>
  <NSelect
    :value="value"
    :placeholder="placeholder || $t('common.select')"
    :options="options"
    :loading="loading"
    :disabled="disabled"
    clearable
    @update:value="handleUpdateValue"
  >
    <template #empty>
      <div class="flex-col-center gap-12px pb-12px pt-12px">
        <NEmpty :description="$t('common.noData')" />
        <NButton type="primary" ghost size="small" @click="routerPushByKey('settings_storage-config')">
          {{ $t('common.config') }}
        </NButton>
      </div>
    </template>
  </NSelect>
</template>

<style scoped></style>
