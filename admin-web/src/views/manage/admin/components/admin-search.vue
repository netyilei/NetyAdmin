<script setup lang="ts">
import { computed } from 'vue';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';
import AppDictSelect from '@/components/custom/app-dict-select.vue';

defineOptions({
  name: 'AdminSearch'
});

interface Emits {
  (e: 'reset'): void;
  (e: 'search'): void;
}

const emit = defineEmits<Emits>();

const { formRef, validate, restoreValidation } = useNaiveForm();

const model = defineModel<SystemManage.AdminSearchParams>('model', { required: true });

type RuleKey = Extract<keyof SystemManage.AdminSearchParams, 'email' | 'phone'>;

const rules = computed<Record<RuleKey, App.Global.FormRule>>(() => {
  const { patternRules } = useFormRules();

  return {
    email: patternRules.email,
    phone: patternRules.phone
  };
});

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
    <NCollapse>
      <NCollapseItem :title="$t('common.search')" name="admin-search">
        <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="80">
          <NGrid responsive="screen" item-responsive>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.manage.admin.userName')" path="username" class="pr-24px">
              <NInput v-model:value="model.username" :placeholder="$t('page.manage.admin.form.userName')" />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.manage.admin.userGender')" path="gender" class="pr-24px">
              <AppDictSelect
                v-model:value="model.gender"
                dict-code="sys_gender"
                :placeholder="$t('page.manage.admin.form.userGender')"
              />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.manage.admin.nickName')" path="nickname" class="pr-24px">
              <NInput v-model:value="model.nickname" :placeholder="$t('page.manage.admin.form.nickName')" />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.manage.admin.userPhone')" path="phone" class="pr-24px">
              <NInput v-model:value="model.phone" :placeholder="$t('page.manage.admin.form.userPhone')" />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.manage.admin.userEmail')" path="email" class="pr-24px">
              <NInput v-model:value="model.email" :placeholder="$t('page.manage.admin.form.userEmail')" />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.manage.admin.userStatus')" path="status" class="pr-24px">
              <AppDictSelect
                v-model:value="model.status"
                dict-code="sys_status"
                :placeholder="$t('page.manage.admin.form.userStatus')"
              />
            </NFormItemGi>
            <NFormItemGi span="24 m:12" class="pr-24px">
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
