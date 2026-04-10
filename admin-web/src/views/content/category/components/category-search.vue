<script setup lang="ts">
import { computed } from 'vue';
import type { Content } from '@/typings/api/v1/content';
import { $t } from '@/locales';

defineOptions({
  name: 'CategorySearch'
});

interface Emits {
  (e: 'reset'): void;
  (e: 'search'): void;
}

const emit = defineEmits<Emits>();

const model = defineModel<Content.CategorySearchParams>('model', { required: true });

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
  <NCard :bordered="false" size="small" class="card-wrapper">
    <NForm :model="model" label-placement="left" :label-width="80">
      <NGrid responsive="screen" item-responsive>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :label="$t('page.content.category.categoryName')" path="name">
            <NInput v-model:value="model.name" :placeholder="$t('page.content.category.form.categoryName')" clearable />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :label="$t('page.content.category.categoryCode')" path="code">
            <NInput v-model:value="model.code" :placeholder="$t('page.content.category.form.categoryCode')" clearable />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :label="$t('common.status')" path="status">
            <NSelect
              v-model:value="model.status"
              :placeholder="$t('common.pleaseSelect')"
              clearable
              :options="[
                { label: $t('common.enable'), value: '1' },
                { label: $t('common.disable'), value: '0' }
              ]"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :label="$t('page.content.category.createdAt')" path="startTime">
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
