<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import type { UploadFileInfo } from 'naive-ui';
import { fetchAddUser, fetchGetSysConfigs, fetchUpdateUser } from '@/service/api/v1/system-manage';
import { fetchCreateUploadRecord, fetchGetUploadCredentials } from '@/service/api/v1/storage';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import { useOperation } from '@/hooks/common/operation';
import { uploadWithPresignedUrl } from '@/utils/upload';
import { $t } from '@/locales';
import AppDictSelect from '@/components/custom/app-dict-select.vue';

defineOptions({
  name: 'UserOperateModal'
});

interface Props {
  operateType: NaiveUI.TableOperateType;
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
const avatarUploading = ref(false);
const passwordRules = reactive({
  minLength: 8,
  requireTypes: 2
});
const storageConfigId = ref<number | undefined>(undefined);

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

async function loadUserConfigs() {
  const { data } = await fetchGetSysConfigs('user_config');
  if (data) {
    const minLength = data.find(item => item.configKey === 'password_min_length');
    const requireTypes = data.find(item => item.configKey === 'password_require_types');
    if (minLength) passwordRules.minLength = Number(minLength.configValue);
    if (requireTypes) passwordRules.requireTypes = Number(requireTypes.configValue);

    const storageModule = data.find(item => item.configKey === 'storage_module');
    if (storageModule && storageModule.configValue) {
      storageConfigId.value = Number(storageModule.configValue) || undefined;
    } else {
      storageConfigId.value = undefined;
    }
  }
}

const rules: Record<string, App.Global.FormRule[]> = {
  username: [defaultRequiredRule],
  password: [
    {
      validator: (rule, value) => {
        if (props.operateType === 'edit' && !value) {
          return true;
        }
        if (props.operateType === 'add' && !value) {
          return new Error($t('form.password.required'));
        }

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
    model.password = '';
  }
}

async function handleAvatarUpload(options: { file: UploadFileInfo }) {
  if (!options.file.file) return;

  avatarUploading.value = true;
  try {
    const { data, error } = await fetchGetUploadCredentials({
      configId: storageConfigId.value,
      fileName: options.file.name,
      fileSize: options.file.file.size,
      contentType: options.file.file.type || 'image/jpeg',
      businessType: 'user_avatar'
    });

    if (!error && data) {
      const fileUrl = await uploadWithPresignedUrl(data, options.file.file);
      model.avatar = fileUrl;

      await fetchCreateUploadRecord({
        configId: data.configId,
        fileName: options.file.name,
        objectKey: data.objectKey,
        fileSize: options.file.file.size,
        mimeType: options.file.file.type || 'image/jpeg',
        businessType: 'user_avatar'
      });

      window.$message?.success($t('common.updateSuccess'));
    }
  } catch {
    window.$message?.error?.($t('common.updateFailed'));
  } finally {
    avatarUploading.value = false;
  }
}

function closeModal() {
  visible.value = false;
}

async function handleSubmit() {
  await validate();

  const submitData = { ...model };
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
    loadUserConfigs();
  }
});
</script>

<template>
  <NModal v-model:show="visible" :title="title" preset="card" class="w-600px">
    <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="100">
      <NFormItem :label="$t('page.manage.user.avatar')" path="avatar">
        <div class="flex items-center gap-8px">
          <NUpload :custom-request="handleAvatarUpload as any" :show-file-list="false" accept="image/*">
            <NButton :loading="avatarUploading">{{ $t('common.upload') }}</NButton>
          </NUpload>
          <NImage
            v-if="model.avatar"
            :src="model.avatar"
            width="48"
            height="48"
            object-fit="cover"
            class="rounded-full"
          />
        </div>
      </NFormItem>
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
