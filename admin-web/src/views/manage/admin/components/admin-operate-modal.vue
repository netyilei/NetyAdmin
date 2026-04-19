<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import { fetchAddAdmin, fetchGetAllRoles, fetchUpdateAdmin } from '@/service/api/v1/system-manage';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import { useOperation } from '@/hooks/common/operation';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';
import AppDictRadioGroup from '@/components/custom/app-dict-radio-group.vue';

defineOptions({
  name: 'AdminOperateModal'
});

interface Props {
  operateType: NaiveUI.TableOperateType;
  rowData?: SystemManage.Admin | null;
}

const props = defineProps<Props>();

interface Emits {
  (e: 'submitted', userData?: SystemManage.Admin): void;
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
    add: $t('page.manage.admin.addAdmin'),
    edit: $t('page.manage.admin.editAdmin')
  };
  return titles[props.operateType];
});

type Model = SystemManage.EditAdmin;

const model = reactive(createDefaultModel());

function createDefaultModel(): Model {
  return {
    username: '',
    gender: null,
    nickname: '',
    phone: '',
    email: '',
    roles: [],
    status: null
  };
}

type RuleKey = Extract<keyof Model, 'username' | 'status'>;

const rules: Record<RuleKey, App.Global.FormRule> = {
  username: defaultRequiredRule,
  status: defaultRequiredRule
};

const roleOptions = ref<CommonType.Option<string>[]>([]);

async function getRoleOptions() {
  const { error, data } = await fetchGetAllRoles();

  if (!error) {
    roleOptions.value = data.map(item => ({
      label: item.name,
      value: item.code
    }));
  }
}

function handleInitModel() {
  Object.assign(model, createDefaultModel());

  if (props.operateType === 'edit' && props.rowData) {
    Object.assign(model, {
      id: props.rowData.id,
      username: props.rowData.userName,
      gender: props.rowData.userGender,
      nickname: props.rowData.nickName,
      phone: props.rowData.userPhone,
      email: props.rowData.userEmail,
      roles: props.rowData.userRoles,
      status: props.rowData.status
    });
  }
}

function closeModal() {
  visible.value = false;
}

async function handleSubmit() {
  await validate();

  await useOperation(props.operateType, loading, {
    editBeforeValidate: () => {
      return Boolean(model && model.id);
    },
    edit: () => fetchUpdateAdmin({ ...model, id: model.id! }),
    add: () => fetchAddAdmin(model as SystemManage.EditAdmin),
    onSuccess: () => {
      closeModal();
      emit('submitted', model as unknown as SystemManage.Admin);
    }
  });
}

watch(visible, val => {
  if (val) {
    handleInitModel();
    restoreValidation();
    getRoleOptions();
  }
});
</script>

<template>
  <NModal v-model:show="visible" preset="card" :title="title" class="max-w-95vw w-800px overflow-y-auto">
    <NForm ref="formRef" :model="model" :rules="rules">
      <NFormItem :label="$t('page.manage.admin.userName')" path="username">
        <NInput v-model:value="model.username" :placeholder="$t('page.manage.admin.form.userName')" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.admin.userGender')" path="gender">
        <AppDictRadioGroup v-model:value="model.gender" dict-code="sys_gender" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.admin.nickName')" path="nickname">
        <NInput v-model:value="model.nickname" :placeholder="$t('page.manage.admin.form.nickName')" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.admin.userPhone')" path="phone">
        <NInput v-model:value="model.phone" :placeholder="$t('page.manage.admin.form.userPhone')" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.admin.userEmail')" path="email">
        <NInput v-model:value="model.email" :placeholder="$t('page.manage.admin.form.userEmail')" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.admin.userStatus')" path="status">
        <AppDictRadioGroup v-model:value="model.status" dict-code="sys_status" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.admin.userRole')" path="roles">
        <NSelect
          v-model:value="model.roles"
          multiple
          :options="roleOptions"
          :placeholder="$t('page.manage.admin.form.userRole')"
        />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace :size="16">
        <NButton @click="closeModal">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
