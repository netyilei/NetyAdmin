<script setup lang="ts">
import { computed } from 'vue';
import type { Storage } from '@/typings/api/v1/storage';
import { $t } from '@/locales';

defineOptions({
  name: 'UploadRecordSearch'
});

interface Emits {
  (e: 'reset'): void;
  (e: 'search'): void;
}

const emit = defineEmits<Emits>();

const model = defineModel<Storage.UploadRecordSearchParams>('model', { required: true });

const sourceOptions = [
  { label: $t('page.manage.upload.sourceAdmin'), value: 'admin' },
  { label: $t('page.manage.upload.sourceClient'), value: 'client' },
  { label: $t('page.manage.upload.sourceApi'), value: 'api' },
  { label: $t('page.manage.upload.sourceSystem'), value: 'system' }
];

const dateRange = computed({
  get() {
    if (model.value.startTime && model.value.endTime) {
      return [new Date(model.value.startTime).getTime(), new Date(model.value.endTime).getTime()] as [number, number];
    }
    return null;
  },
  set(val) {
    if (val && val.length === 2) {
      const [start, end] = val;
      model.value.startTime = new Date(start).toISOString().replace('T', ' ').split('.')[0];
      model.value.endTime = new Date(end).toISOString().replace('T', ' ').split('.')[0];
      // Note: Backend might expect YYYY-MM-DD HH:mm:ss.
      // JavaScript's toISOString returns Z at the end.
      // Let's use a better formatter if possible, but let's see.
    } else {
      model.value.startTime = undefined;
      model.value.endTime = undefined;
    }
  }
});

function reset() {
  emit('reset');
}

function search() {
  emit('search');
}
</script>

<template>
  <NCard :title="$t('common.search')" :bordered="false" size="small" class="card-wrapper">
    <NForm :model="model" label-placement="left" :label-width="80">
      <NGrid responsive="screen" item-responsive>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :label="$t('page.manage.upload.fileName')" path="fileName">
            <NInput v-model:value="model.fileName" :placeholder="$t('common.pleaseInput')" clearable />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :label="$t('page.manage.upload.source')" path="source">
            <NSelect
              v-model:value="model.source"
              :options="sourceOptions"
              :placeholder="$t('common.pleaseSelect')"
              clearable
            />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :label="$t('page.manage.upload.businessType')" path="businessType">
            <NInput v-model:value="model.businessType" :placeholder="$t('common.pleaseInput')" clearable />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:12">
          <NFormItem :label="$t('common.createdAt')" path="startTime">
            <NDatePicker
              v-model:value="dateRange"
              type="datetimerange"
              clearable
              class="w-full"
              format="yyyy-MM-dd HH:mm:ss"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :show-label="false">
            <NSpace class="w-full" justify="end">
              <NButton @click="reset">
                {{ $t('common.reset') }}
              </NButton>
              <NButton type="primary" @click="search">
                {{ $t('common.search') }}
              </NButton>
            </NSpace>
          </NFormItem>
        </NGridItem>
      </NGrid>
    </NForm>
  </NCard>
</template>

<style scoped></style>
