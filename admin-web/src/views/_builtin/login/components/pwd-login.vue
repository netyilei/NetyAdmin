<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { isTruthyConfigValue } from '@/constants/business';
import { fetchGetCaptcha } from '@/service/api/v1/auth';
import { fetchGetSysConfigs } from '@/service/api/v1/system-manage';
import { useAuthStore } from '@/store/modules/auth';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import { $t } from '@/locales';

defineOptions({
  name: 'PwdLogin'
});

const authStore = useAuthStore();
const { formRef, validate } = useNaiveForm();

interface FormModel {
  userName: string;
  password: string;
  captchaId: string;
  captchaValue: string;
}

const model: FormModel = reactive({
  userName: '',
  password: '',
  captchaId: '',
  captchaValue: ''
});

const captchaEnabled = ref(false);
const captchaImg = ref('');

const rules = computed<Record<keyof FormModel, App.Global.FormRule[]>>(() => {
  const { formRules } = useFormRules();
  const rule: Record<keyof FormModel, App.Global.FormRule[]> = {
    userName: formRules.userName,
    password: formRules.pwd,
    captchaId: [{ required: captchaEnabled.value, message: $t('page.login.common.captchaPlaceholder') }],
    captchaValue: [{ required: captchaEnabled.value, message: $t('page.login.common.captchaPlaceholder') }]
  };
  return rule;
});

async function getCaptcha() {
  const { data, error } = await fetchGetCaptcha();
  if (!error) {
    model.captchaId = data.captchaId;
    captchaImg.value = data.captchaImg;
  }
}

async function checkCaptchaEnabled() {
  const { data, error } = await fetchGetSysConfigs('captcha_config');
  if (!error) {
    const config = data.find(item => item.configKey === 'admin_login_enabled');
    captchaEnabled.value = isTruthyConfigValue(config?.configValue);
    if (captchaEnabled.value) {
      getCaptcha();
    }
  }
}

async function handleSubmit() {
  await validate();
  await authStore.login({
    username: model.userName,
    password: model.password,
    captchaId: model.captchaId,
    captchaValue: model.captchaValue
  });
  // 如果登录失败且开启了验证码，刷新验证码
  if (!authStore.token && captchaEnabled.value) {
    getCaptcha();
    model.captchaValue = '';
  }
}

onMounted(() => {
  checkCaptchaEnabled();
});
</script>

<template>
  <NForm ref="formRef" :model="model" :rules="rules" size="large" :show-label="false" @keyup.enter="handleSubmit">
    <NFormItem path="userName">
      <NInput v-model:value="model.userName" :placeholder="$t('page.login.common.userNamePlaceholder')" />
    </NFormItem>
    <NFormItem path="password">
      <NInput
        v-model:value="model.password"
        type="password"
        show-password-on="click"
        :placeholder="$t('page.login.common.passwordPlaceholder')"
      />
    </NFormItem>
    <NFormItem v-if="captchaEnabled" path="captchaValue">
      <div class="w-full flex-y-center gap-12px">
        <NInput v-model:value="model.captchaValue" :placeholder="$t('page.login.common.captchaPlaceholder')" />
        <div class="h-38px w-120px cursor-pointer border border-gray-200 rounded-4px" @click="getCaptcha">
          <img v-if="captchaImg" :src="captchaImg" class="h-full w-full" />
        </div>
      </div>
    </NFormItem>
    <NButton type="primary" size="large" round block :loading="authStore.loginLoading" @click="handleSubmit">
      {{ $t('common.confirm') }}
    </NButton>
  </NForm>
</template>

<style scoped></style>
