<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue';
import type { FormInst } from 'naive-ui';
import { fetchChangePassword, fetchGetProfile, fetchUpdateProfile } from '@/service/api/v1/auth';
import type { Auth } from '@/typings/api/v1/auth';
import { $t } from '@/locales';

defineOptions({ name: 'AdminProfileModal' });

const visible = defineModel<boolean>('show', { default: false });

const profileLoading = ref(false);
const passwordLoading = ref(false);
const activeTab = ref<'profile' | 'password'>('profile');

const profileFormRef = ref<FormInst | null>(null);
const passwordFormRef = ref<FormInst | null>(null);

const profileModel = reactive<Auth.UpdateProfileParams>({
  nickName: '',
  userPhone: '',
  userEmail: '',
  userGender: '1'
});

const passwordModel = reactive<Auth.ChangePasswordParams & { confirmPassword: string }>({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
});

const userGender = computed({
  get() {
    return profileModel.userGender as unknown as string;
  },
  set(val: string | number | null) {
    profileModel.userGender = String(val || '1') as Auth.UserGender;
  }
});

const passwordRules = {
  oldPassword: { required: true, message: $t('page.adminProfile.form.oldPassword'), trigger: 'blur' },
  newPassword: [
    { required: true, message: $t('page.adminProfile.form.newPassword'), trigger: 'blur' },
    { min: 6, message: $t('page.adminProfile.passwordRule'), trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: $t('page.adminProfile.form.confirmPassword'), trigger: 'blur' },
    {
      validator: (_rule: unknown, value: string) => value === passwordModel.newPassword,
      message: $t('page.adminProfile.passwordMismatch'),
      trigger: 'blur'
    }
  ]
};

async function loadProfile() {
  profileLoading.value = true;
  const { error, data } = await fetchGetProfile();
  profileLoading.value = false;
  if (error) return;

  profileModel.nickName = data.nickName || '';
  profileModel.userPhone = data.userPhone || '';
  profileModel.userEmail = data.userEmail || '';
  profileModel.userGender = data.userGender || '1';
}

async function handleSaveProfile() {
  await profileFormRef.value?.validate();
  profileLoading.value = true;
  const { error } = await fetchUpdateProfile(profileModel);
  profileLoading.value = false;
  if (error) return;
  window.$message?.success($t('common.updateSuccess'));
}

async function handleChangePassword() {
  await passwordFormRef.value?.validate();
  passwordLoading.value = true;
  const { error } = await fetchChangePassword({
    oldPassword: passwordModel.oldPassword,
    newPassword: passwordModel.newPassword
  });
  passwordLoading.value = false;
  if (error) return;

  passwordModel.oldPassword = '';
  passwordModel.newPassword = '';
  passwordModel.confirmPassword = '';
  window.$message?.success($t('page.adminProfile.passwordChangeSuccess'));
}

function handleClose() {
  visible.value = false;
}

watch(visible, val => {
  if (val) loadProfile();
});

onMounted(() => {
  if (visible.value) loadProfile();
});
</script>

<template>
  <NModal v-model:show="visible" preset="card" :title="$t('common.adminProfile')" class="w-520px" @close="handleClose">
    <NTabs v-model:value="activeTab" type="line" animated>
      <NTabPane name="profile" :tab="$t('page.adminProfile.profileTab')">
        <NSpin :show="profileLoading">
          <NForm ref="profileFormRef" :model="profileModel" label-placement="left" label-width="auto">
            <NFormItem :label="$t('page.manage.admin.nickName')" path="nickName">
              <NInput v-model:value="profileModel.nickName" :placeholder="$t('common.pleaseInput')" />
            </NFormItem>
            <NFormItem :label="$t('page.manage.admin.userPhone')" path="userPhone">
              <NInput v-model:value="profileModel.userPhone" :placeholder="$t('common.pleaseInput')" />
            </NFormItem>
            <NFormItem :label="$t('page.manage.admin.userEmail')" path="userEmail">
              <NInput v-model:value="profileModel.userEmail" :placeholder="$t('common.pleaseInput')" />
            </NFormItem>
            <NFormItem :label="$t('page.manage.admin.userGender')" path="userGender">
              <AppDictRadioGroup v-model:value="userGender" dict-code="user_gender" />
            </NFormItem>
          </NForm>
        </NSpin>
      </NTabPane>

      <NTabPane name="password" :tab="$t('page.adminProfile.passwordTab')">
        <NSpin :show="passwordLoading">
          <NForm
            ref="passwordFormRef"
            :model="passwordModel"
            :rules="passwordRules"
            label-placement="left"
            label-width="auto"
          >
            <NFormItem :label="$t('page.adminProfile.oldPassword')" path="oldPassword">
              <NInput v-model:value="passwordModel.oldPassword" type="password" show-password-on="mousedown" />
            </NFormItem>
            <NFormItem :label="$t('page.adminProfile.newPassword')" path="newPassword">
              <NInput v-model:value="passwordModel.newPassword" type="password" show-password-on="mousedown" />
            </NFormItem>
            <NFormItem :label="$t('page.adminProfile.confirmPassword')" path="confirmPassword">
              <NInput v-model:value="passwordModel.confirmPassword" type="password" show-password-on="mousedown" />
            </NFormItem>
          </NForm>
        </NSpin>
      </NTabPane>
    </NTabs>

    <template #footer>
      <NSpace justify="end">
        <NButton @click="handleClose">{{ $t('common.cancel') }}</NButton>
        <NButton v-if="activeTab === 'profile'" type="primary" :loading="profileLoading" @click="handleSaveProfile">
          {{ $t('common.save') }}
        </NButton>
        <NButton v-else type="primary" :loading="passwordLoading" @click="handleChangePassword">
          {{ $t('page.adminProfile.changePassword') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
