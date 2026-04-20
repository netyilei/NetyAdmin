<script setup lang="ts">
import { useNaiveForm } from '@/hooks/common/form';
import { $t } from '@/locales';
import type { OpenApp } from '@/typings/api/v1/open-app';

defineOptions({
  name: 'AppSearch'
});

const emit = defineEmits<{
  search: [];
  reset: [];
}>();

const model = defineModel<OpenApp.AppQueryParams>('model', { required: true });

const { formRef, restoreValidation } = useNaiveForm();

async function reset() {
  model.value.name = '';
  model.value.appKey = '';
  model.value.status = undefined;
  await restoreValidation();
  emit('reset');
}

async function search() {
  emit('search');
}
</script>

<template>
  <NCard :bordered="false" size="small" class="card-wrapper">
    <NCollapse :default-expanded-names="['search']">
      <NCollapseItem :title="$t('common.search')" name="search">
        <NForm ref="formRef" :model="model" label-placement="left" :label-width="80">
          <NGrid responsive="screen" item-responsive>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.openPlatform.app.name')" path="name" class="pr-24px">
              <NInput v-model:value="model.name" :placeholder="$t('page.openPlatform.app.form.namePlaceholder')" />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.openPlatform.app.appKey')" path="appKey" class="pr-24px">
              <NInput v-model:value="model.appKey" :placeholder="$t('page.openPlatform.app.form.appKeyPlaceholder')" />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.openPlatform.app.status')" path="status" class="pr-24px">
              <AppDictSelect
                v-model:value="model.status"
                dict-code="sys_status"
                value-type="number"
                :placeholder="$t('page.openPlatform.app.form.statusPlaceholder')"
                clearable
              />
            </NFormItemGi>
            <NFormItemGi span="24">
              <NSpace class="w-full" justify="end">
                <NButton @click="reset">
                  <template #icon>
                    <icon-ic-round-refresh class="text-icon" />
                  </template>
                  {{ $t('common.reset') }}
                </NButton>
                <NButton type="primary" ghost @click="search">
                  <template #icon>
                    <icon-ic-round-search class="text-icon" />
                  </template>
                  {{ $t('common.search') }}
                </NButton>
              </NSpace>
            </NFormItemGi>
          </NGrid>
        </NForm>
      </NCollapseItem>
    </NCollapse>
  </NCard>
</template>

<style scoped></style>
