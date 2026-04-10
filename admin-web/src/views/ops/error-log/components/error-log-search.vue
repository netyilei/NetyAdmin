<script setup lang="ts">
import { computed } from 'vue';
import type { Log } from '@/typings/api/v1/log';
import { $t } from '@/locales';

defineOptions({
  name: 'ErrorLogSearch'
});

interface Emits {
  (e: 'reset'): void;
  (e: 'search'): void;
}

const emit = defineEmits<Emits>();

const model = defineModel<Log.ErrorLogSearchParams>('model', { required: true });

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

const resolved = computed({
  get() {
    if (typeof model.value.resolved === 'boolean') {
      return model.value.resolved ? 1 : 0;
    }
    return null;
  },
  set(val) {
    if (val === 1) {
      model.value.resolved = true;
    } else if (val === 0) {
      model.value.resolved = false;
    } else {
      model.value.resolved = undefined;
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
          <NFormItem :label="$t('page.ops.errorLog.level')" path="level">
            <NSelect
              v-model:value="model.level"
              :placeholder="$t('common.pleaseSelect')"
              clearable
              :options="[
                { label: $t('page.ops.errorLog.levelError'), value: 'error' },
                { label: $t('page.ops.errorLog.levelPanic'), value: 'panic' },
                { label: $t('page.ops.errorLog.levelWarn'), value: 'warn' }
              ]"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:6">
          <NFormItem :label="$t('page.ops.errorLog.status')" path="resolved">
            <NSelect
              v-model:value="resolved"
              :placeholder="$t('common.pleaseSelect')"
              clearable
              :options="[
                { label: $t('page.ops.errorLog.statusResolved'), value: 1 },
                { label: $t('page.ops.errorLog.statusPending'), value: 0 }
              ]"
            />
          </NFormItem>
        </NGridItem>
        <NGridItem span="24 s:12 m:6">
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
