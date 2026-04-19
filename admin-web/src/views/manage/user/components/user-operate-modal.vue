<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import { fetchAddUser, fetchGetSysConfigs, fetchUpdateUser } from '@/service/api/v1/system-manage';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import { useOperation } from '@/hooks/common/operation';
import { $t } from '@/locales';
import AppDictSelect from '@/components/custom/app-dict-select.vue';

defineOptions({
  name: 'UserOperateModal'
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
const passwordRules = reactive({
  minLength: 8,
  requireTypes: 2
});

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
    id: '',
    username: '',
    password: '',
    nickname: '',
    avatar: '',
    gender: '0',
    phone: '',
    email: '',
    status: '1'
  };
}

async function getPasswordConfig() {
  const { data } = await fetchGetSysConfigs('user_config');
  if (data) {
    const minLength = data.find(item => item.configKey === 'password_min_length');
    const requireTypes = data.find(item => item.configKey === 'password_require_types');
    if (minLength) passwordRules.minLength = Number(minLength.configValue);
    if (requireTypes) passwordRules.requireTypes = Number(requireTypes.configValue);
  }
}

const rules: Record<string, App.Global.FormRule[]> = {
  username: [defaultRequiredRule],
  password: [
    {
      validator: (rule, value) => {
        // 编辑模式下，如果不输入密码则跳过校验（不修改密码）
        if (props.operateType === 'edit' && !value) {
          return true;
        }
        // 新增模式下密码必填
        if (props.operateType === 'add' && !value) {
          return new Error($t('form.password.required'));
        }

        // 强度校验
        if (value.length < passwordRules.minLength) {
          return new Error(`密码长度不能少于 ${passwordRules.minLength} 位`);
        }

        let types = 0;
        if (/[a-z]/.test(value)) types += 1;
        if (/[A-Z]/.test(value)) types += 1;
        if (/[0-9]/.test(value)) types += 1;
        if (/[^a-zA-Z0-9]/.test(value)) types += 1;

        if (types < passwordRules.requireTypes) {
          return new Error(`密码必须包含数字、大小写字母、特殊符号中的至少 ${passwordRules.requireTypes} 种`);
        }
        return true;
      },
      trigger: 'blur'
    }
  ],
  status: [defaultRequiredRule]
};

function handleInitModel() {
  Object.assign(model, createDefaultModel());
  if (props.operateType === 'edit' && props.rowData) {
    Object.assign(model, props.rowData);
    // 编辑模式下清空密码占位，不回显
    model.password = '';
  }
}

function closeModal() {
  visible.value = false;
}

async function handleSubmit() {
  await validate();

  // 构造提交数据
  const submitData = { ...model };
  // 如果是编辑模式且密码为空，则删除密码字段，不进行更新
  if (props.operateType === 'edit' && !submitData.password) {
    delete (submitData as any).password;
  }

  await useOperation(props.operateType, loading, {
    add: () => fetchAddUser(submitData),
    edit: () => fetchUpdateUser(model.id, submitData),
    onSuccess: () => {
      closeModal();
      emit('submitted');
    }
  });
}

watch(visible, val => {
  if (val) {
    handleInitModel();
    restoreValidation();
    getPasswordConfig();
  }
});
</script>

<template>
  <NModal v-model:show="visible" :title="title" preset="card" class="w-600px">
    <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="100">
      <NFormItem :label="$t('page.manage.user.username')" path="username">
        <NInput
          v-model:value="model.username"
          :disabled="operateType === 'edit'"
          :placeholder="$t('common.pleaseInput')"
        />
      </NFormItem>
      <NFormItem :label="$t('page.manage.user.password')" path="password">
        <NInput
          v-model:value="model.password"
          type="password"
          show-password-on="mousedown"
          :placeholder="operateType === 'edit' ? '留空表示不修改密码' : $t('common.pleaseInput')"
        />
      </NFormItem>
      <NFormItem :label="$t('page.manage.user.nickname')" path="nickname">
        <NInput v-model:value="model.nickname" :placeholder="$t('common.pleaseInput')" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.user.phone')" path="phone">
        <NInput v-model:value="model.phone" :placeholder="$t('common.pleaseInput')" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.user.email')" path="email">
        <NInput v-model:value="model.email" :placeholder="$t('common.pleaseInput')" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.user.gender')" path="gender">
        <AppDictSelect v-model:value="model.gender" dict-code="sys_gender" />
      </NFormItem>
      <NFormItem :label="$t('page.manage.user.status')" path="status">
        <AppDictSelect v-model:value="model.status" dict-code="sys_status" />
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
