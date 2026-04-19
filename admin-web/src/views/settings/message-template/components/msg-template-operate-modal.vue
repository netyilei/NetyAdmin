<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import { addTemplate, updateTemplate } from '@/service/api/v1/message-hub';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import { useOperation } from '@/hooks/common/operation';
import { $t } from '@/locales';

defineOptions({
  name: 'MsgTemplateOperateModal'
});

interface Props {
  /** the type of operation */
  operateType: NaiveUI.TableOperateType;
  /** the edit row data */
  rowData?: any | null;
}

const props = defineProps<Props>();

interface Emits {
  (e: 'submitted'): void;
}

const emit = defineEmits<Emits>();

const visible = defineModel<boolean>('visible', {
  default: false
});

const { formRef, validate, restoreValidation } = useNaiveForm();
const { defaultRequiredRule } = useFormRules();

const loading = ref(false);

const title = computed(() => {
  const titles: Record<NaiveUI.TableOperateType, string> = {
    add: $t('common.add'),
    edit: $t('common.edit')
  };
  return titles[props.operateType];
});

const model = reactive(createDefaultModel());

function createDefaultModel() {
  return {
    id: undefined,
    code: '',
    name: '',
    channel: 'sms',
    title: '',
    content: '',
    providerTplId: '',
    status: 1
  };
}

const rules: Record<string, App.Global.FormRule[]> = {
  code: [defaultRequiredRule],
  name: [defaultRequiredRule],
  channel: [defaultRequiredRule],
  content: [defaultRequiredRule]
};

async function handleSubmit() {
  await validate();

  await useOperation(props.operateType, loading, {
    add: () => addTemplate(model),
    edit: () => updateTemplate(model),
    onSuccess: () => {
      closeModal();
      emit('submitted');
    }
  });
}

function closeModal() {
  visible.value = false;
}

watch(visible, () => {
  if (visible.value) {
    if (props.operateType === 'edit' && props.rowData) {
      Object.assign(model, props.rowData);
    } else {
      Object.assign(model, createDefaultModel());
    }
    restoreValidation();
  }
});
</script>

<template>
  <NModal v-model:show="visible" :title="title" preset="card" class="w-800px">
    <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="120">
      <NGrid :cols="24" :x-gap="18">
        <NFormItemGi :span="12" :label="$t('page.messageHub.template.code')" path="code">
          <NInput v-model:value="model.code" :placeholder="$t('page.messageHub.template.form.codePlaceholder')" />
        </NFormItemGi>
        <NFormItemGi :span="12" :label="$t('page.messageHub.template.name')" path="name">
          <NInput v-model:value="model.name" :placeholder="$t('page.messageHub.template.form.namePlaceholder')" />
        </NFormItemGi>
        <NFormItemGi :span="12" :label="$t('page.messageHub.template.channel')" path="channel">
          <AppDictSelect
            v-model:value="model.channel"
            dict-code="sys_msg_channel"
            :placeholder="$t('page.messageHub.template.form.channelPlaceholder')"
          />
        </NFormItemGi>
        <NFormItemGi :span="12" :label="$t('page.messageHub.template.status')" path="status">
          <AppDictSelect v-model:value="model.status" dict-code="sys_status" />
        </NFormItemGi>
        <NFormItemGi :span="24" :label="$t('page.messageHub.template.msgTitle')" path="title">
          <NInput v-model:value="model.title" :placeholder="$t('page.messageHub.template.form.titlePlaceholder')" />
        </NFormItemGi>
        <NFormItemGi :span="24" :label="$t('page.messageHub.template.content')" path="content">
          <NInput
            v-model:value="model.content"
            type="textarea"
            :autosize="{ minRows: 3, maxRows: 6 }"
            :placeholder="$t('page.messageHub.template.form.contentPlaceholder')"
          />
        </NFormItemGi>
        <NFormItemGi :span="24" :label="$t('page.messageHub.template.providerTplId')" path="providerTplId">
          <NInput
            v-model:value="model.providerTplId"
            :placeholder="$t('page.messageHub.template.form.providerTplIdPlaceholder')"
          />
        </NFormItemGi>
      </NGrid>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="closeModal">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="loading" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
