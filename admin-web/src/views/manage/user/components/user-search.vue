<script setup lang="ts">
import { useNaiveForm } from '@/hooks/common/form';
import { $t } from '@/locales';
import AppDictSelect from '@/components/custom/app-dict-select.vue';

defineOptions({
  name: 'UserSearch'
});

interface Emits {
  (e: 'reset'): void;
  (e: 'search'): void;
}

const emit = defineEmits<Emits>();

const { formRef, validate, restoreValidation } = useNaiveForm();

const model = defineModel<any>('model', { required: true });

async function reset() {
  await restoreValidation();
  emit('reset');
}

async function search() {
  await validate();
  emit('search');
}
</script>

<template>
  <NCard :bordered="false" size="small" class="card-wrapper">
    <NCollapse :default-expanded-names="['user-search']">
      <NCollapseItem :title="$t('common.search')" name="user-search">
        <NForm ref="formRef" :model="model" label-placement="left" :label-width="80">
          <NGrid responsive="screen" item-responsive>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.manage.user.username')" path="username" class="pr-24px">
              <NInput v-model:value="model.username" :placeholder="$t('common.pleaseInput')" />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.manage.user.nickname')" path="nickname" class="pr-24px">
              <NInput v-model:value="model.nickname" :placeholder="$t('common.pleaseInput')" />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.manage.user.phone')" path="phone" class="pr-24px">
              <NInput v-model:value="model.phone" :placeholder="$t('common.pleaseInput')" />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.manage.user.email')" path="email" class="pr-24px">
              <NInput v-model:value="model.email" :placeholder="$t('common.pleaseInput')" />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.manage.user.gender')" path="gender" class="pr-24px">
              <AppDictSelect
                v-model:value="model.gender"
                dict-code="sys_gender"
                :placeholder="$t('common.pleaseSelect')"
                clearable
              />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.manage.user.status')" path="status" class="pr-24px">
              <AppDictSelect
                v-model:value="model.status"
                dict-code="sys_status"
                :placeholder="$t('common.pleaseSelect')"
                clearable
              />
            </NFormItemGi>
            <NFormItemGi span="24 m:24">
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
