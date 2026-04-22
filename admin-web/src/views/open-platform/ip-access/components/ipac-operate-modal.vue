<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import { addIPAC, updateIPAC } from '@/service/api/v1/system-ipac';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import { useOperation } from '@/hooks/common/operation';
import { $t } from '@/locales';
import type { SystemIPAC } from '@/typings/api/v1/system-ipac';

defineOptions({
  name: 'IPACOperateModal'
});

interface Props {
  /** the type of operation */
  operateType: NaiveUI.TableOperateType;
  /** the edit row data */
  rowData?: SystemIPAC.IPAC | null;
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

type Model = SystemIPAC.CreateIPACReq & { id?: number };

const model: Model = reactive(createDefaultModel());

function createDefaultModel(): Model {
  return {
    appId: undefined,
    ipAddr: '',
    type: 2, // Default Deny
    reason: '',
    expiredAt: undefined,
    status: 1
  };
}

const rules: Record<string, App.Global.FormRule[]> = {
  ipAddr: [defaultRequiredRule],
  type: [defaultRequiredRule],
  status: [defaultRequiredRule]
};

async function handleSubmit() {
  await validate();

  await useOperation(props.operateType, loading, {
    add: () => addIPAC(model),
    edit: () => updateIPAC(model as SystemIPAC.UpdateIPACReq),
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
      Object.assign(model, {
        ...props.rowData,
        expiredAt: props.rowData.expiredAt || undefined
      });
    } else {
      Object.assign(model, createDefaultModel());
    }
    restoreValidation();
  }
});
</script>

<template>
  <NModal v-model:show="visible" :title="title" preset="card" class="w-600px">
    <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="100">
      <NFormItem :label="$t('page.openPlatform.ipac.ipAddr')" path="ipAddr">
        <NInput v-model:value="model.ipAddr" :placeholder="$t('page.openPlatform.ipac.form.ipAddrPlaceholder')" />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.ipac.type')" path="type">
        <AppDictSelect
          v-model:value="model.type"
          dict-code="sys_ip_action_type"
          :placeholder="$t('page.openPlatform.ipac.form.typePlaceholder')"
        />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.ipac.status')" path="status">
        <AppDictSelect
          v-model:value="model.status"
          dict-code="sys_status"
          :placeholder="$t('page.openPlatform.ipac.form.statusPlaceholder')"
        />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.ipac.reason')" path="reason">
        <NInput
          v-model:value="model.reason"
          type="textarea"
          :placeholder="$t('page.openPlatform.ipac.form.reasonPlaceholder')"
        />
      </NFormItem>
      <NFormItem :label="$t('page.openPlatform.ipac.expiredAt')" path="expiredAt">
        <NDatePicker
          v-model:formatted-value="model.expiredAt"
          type="datetime"
          value-format="yyyy-MM-dd HH:mm:ss"
          :placeholder="$t('page.openPlatform.ipac.form.expiredAtPlaceholder')"
          clearable
          class="w-full"
        />
      </NFormItem>
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
