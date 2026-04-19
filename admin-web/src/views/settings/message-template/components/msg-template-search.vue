<script setup lang="ts">
import { useNaiveForm } from '@/hooks/common/form';
import type { MessageHub } from '@/typings/api/v1/message-hub';
import { $t } from '@/locales';
import AppDictSelect from '@/components/custom/app-dict-select.vue';

defineOptions({
  name: 'MsgTemplateSearch'
});

interface Emits {
  (e: 'reset'): void;
  (e: 'search'): void;
}

const emit = defineEmits<Emits>();

const { formRef, validate, restoreValidation } = useNaiveForm();

const model = defineModel<MessageHub.TemplateQueryParams>('model', { required: true });

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
    <NCollapse :default-expanded-names="['msg-template-search']">
      <NCollapseItem :title="$t('common.search')" name="msg-template-search">
        <NForm ref="formRef" :model="model" label-placement="left" :label-width="80">
          <NGrid responsive="screen" item-responsive>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.messageHub.template.code')" path="code" class="pr-24px">
              <NInput v-model:value="model.code" :placeholder="$t('page.messageHub.template.form.codePlaceholder')" />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.messageHub.template.name')" path="name" class="pr-24px">
              <NInput v-model:value="model.name" :placeholder="$t('page.messageHub.template.form.namePlaceholder')" />
            </NFormItemGi>
            <NFormItemGi
              span="24 s:12 m:6"
              :label="$t('page.messageHub.template.channel')"
              path="channel"
              class="pr-24px"
            >
              <AppDictSelect
                v-model:value="model.channel"
                dict-code="sys_msg_channel"
                :placeholder="$t('page.messageHub.template.form.channelPlaceholder')"
              />
            </NFormItemGi>
            <NFormItemGi
              span="24 s:12 m:6"
              :label="$t('page.messageHub.template.status')"
              path="status"
              class="pr-24px"
            >
              <AppDictSelect
                v-model:value="model.status"
                dict-code="sys_status"
                :placeholder="$t('common.pleaseSelect')"
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

<style scoped></style>
