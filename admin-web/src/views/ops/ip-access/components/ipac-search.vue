<script setup lang="ts">
import { useNaiveForm } from '@/hooks/common/form';
import { $t } from '@/locales';
import type { SystemIPAC } from '@/typings/api/v1/system-ipac';

defineOptions({
  name: 'IPACSearch'
});

const emit = defineEmits<{
  search: [];
  reset: [];
}>();

const model = defineModel<SystemIPAC.IPACQueryParams>('model', { required: true });

const { formRef, restoreValidation } = useNaiveForm();

async function reset() {
  model.value.ipAddr = '';
  model.value.type = undefined;
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
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.ops.ipac.ipAddr')" path="ipAddr" class="pr-24px">
              <NInput v-model:value="model.ipAddr" :placeholder="$t('page.ops.ipac.form.ipAddrPlaceholder')" />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.ops.ipac.type')" path="type" class="pr-24px">
              <AppDictSelect
                v-model:value="model.type"
                dict-code="sys_ip_action_type"
                :placeholder="$t('page.ops.ipac.form.typePlaceholder')"
                clearable
              />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.ops.ipac.status')" path="status" class="pr-24px">
              <AppDictSelect
                v-model:value="model.status"
                dict-code="sys_status"
                :placeholder="$t('page.ops.ipac.form.statusPlaceholder')"
                clearable
              />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6">
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
