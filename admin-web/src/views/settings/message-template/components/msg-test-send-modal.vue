<script setup lang="ts">
import { reactive, ref, watch } from 'vue';
import { sendDirect } from '@/service/api/v1/message-hub';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import { $t } from '@/locales';
import type { MessageHub } from '@/typings/api/v1/message-hub';

defineOptions({
  name: 'MsgTestSendModal'
});

interface Props {
  /** the template data */
  template?: MessageHub.Template | null;
}

const props = defineProps<Props>();

const visible = defineModel<boolean>('visible', {
  default: false
});

const { formRef, validate, restoreValidation } = useNaiveForm();
const { defaultRequiredRule } = useFormRules();

const loading = ref(false);

const model = reactive({
  channel: '',
  receiver: '',
  title: '',
  content: ''
});

const rules = {
  receiver: [defaultRequiredRule],
  content: [defaultRequiredRule]
};

function handleInitModel() {
  if (props.template) {
    model.channel = props.template.channel;
    model.title = props.template.title || '';
    model.content = props.template.content;
    model.receiver = '';
  }
}

async function handleSubmit() {
  await validate();
  loading.value = true;
  const { error } = await sendDirect(model);
  loading.value = false;
  if (!error) {
    window.$message?.success($t('common.sendSuccess'));
    visible.value = false;
  }
}

watch(visible, val => {
  if (val) {
    handleInitModel();
    restoreValidation();
  }
});
</script>

<template>
  <NModal v-model:show="visible" :title="$t('page.messageHub.template.test')" preset="card" class="w-600px">
    <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="100">
      <NFormItem :label="$t('page.messageHub.template.channel')" path="channel">
        <NTag type="primary">{{ $t(`page.messageHub.channel.${model.channel}`) }}</NTag>
      </NFormItem>
      <NFormItem :label="$t('page.messageHub.record.receiver')" path="receiver">
        <NInput v-model:value="model.receiver" :placeholder="$t('common.pleaseInput')" />
      </NFormItem>
      <NFormItem
        v-if="model.channel === 'email' || model.channel === 'internal' || model.channel === 'push'"
        :label="$t('page.messageHub.template.msgTitle')"
        path="title"
      >
        <NInput v-model:value="model.title" :placeholder="$t('page.messageHub.template.form.titlePlaceholder')" />
      </NFormItem>
      <NFormItem :label="$t('page.messageHub.template.content')" path="content">
        <NInput
          v-model:value="model.content"
          type="textarea"
          :autosize="{ minRows: 3, maxRows: 6 }"
          :placeholder="$t('page.messageHub.template.form.contentPlaceholder')"
        />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="visible = false">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="loading" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
