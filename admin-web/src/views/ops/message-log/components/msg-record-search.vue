<script setup lang="ts">
import { useNaiveForm } from '@/hooks/common/form';
import type { MessageHub } from '@/typings/api/v1/message-hub';
import { $t } from '@/locales';
import AppDictSelect from '@/components/custom/app-dict-select.vue';

defineOptions({
  name: 'MsgRecordSearch'
});

interface Emits {
  (e: 'reset'): void;
  (e: 'search'): void;
}

const emit = defineEmits<Emits>();

const { formRef, validate, restoreValidation } = useNaiveForm();

const model = defineModel<MessageHub.RecordQueryParams>('model', { required: true });

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
    <NCollapse :default-expanded-names="['msg-record-search']">
      <NCollapseItem :title="$t('common.search')" name="msg-record-search">
        <NForm ref="formRef" :model="model" label-placement="left" :label-width="80">
          <NGrid responsive="screen" item-responsive>
            <NFormItemGi
              span="24 s:12 m:6"
              :label="$t('page.messageHub.record.receiver')"
              path="receiver"
              class="pr-24px"
            >
              <NInput v-model:value="model.receiver" :placeholder="$t('common.pleaseInput')" />
            </NFormItemGi>
            <NFormItemGi
              span="24 s:12 m:6"
              :label="$t('page.messageHub.record.channel')"
              path="channel"
              class="pr-24px"
            >
              <AppDictSelect
                v-model:value="model.channel"
                dict-code="sys_msg_channel"
                :placeholder="$t('common.pleaseSelect')"
              />
            </NFormItemGi>
            <NFormItemGi span="24 s:12 m:6" :label="$t('page.messageHub.record.status')" path="status" class="pr-24px">
              <AppDictSelect
                v-model:value="model.status"
                dict-code="sys_msg_status"
                :placeholder="$t('common.pleaseSelect')"
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
