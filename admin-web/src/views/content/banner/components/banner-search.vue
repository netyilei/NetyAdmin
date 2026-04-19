<script setup lang="ts">
import type { Content } from '@/typings/api/v1/content';
import { $t } from '@/locales';
import AppDictSelect from '@/components/custom/app-dict-select.vue';

defineOptions({
  name: 'BannerSearch'
});

interface Emits {
  (e: 'reset'): void;
  (e: 'search'): void;
}

const emit = defineEmits<Emits>();

const model = defineModel<Content.BannerSearchParams>('model', { required: true });

function reset() {
  emit('reset');
}

function search() {
  emit('search');
}
</script>

<template>
  <NForm :model="model" label-placement="left" :label-width="80">
    <NGrid responsive="screen" item-responsive>
      <NGridItem span="24 s:12 m:6">
        <NFormItem :label="$t('page.content.bannerItem.titleField')" path="title">
          <NInput v-model:value="model.title" :placeholder="$t('page.content.bannerItem.form.title')" clearable />
        </NFormItem>
      </NGridItem>
      <NGridItem span="24 s:12 m:6">
        <NFormItem :label="$t('common.status')" path="status">
          <AppDictSelect
            v-model:value="model.status"
            dict-code="sys_status"
            :placeholder="$t('common.pleaseSelect')"
            clearable
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
</template>

<style scoped></style>
