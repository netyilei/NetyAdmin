<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import { NButton, NCard, NForm, NFormItem, NInput, NSelect, NSpace, NSwitch } from 'naive-ui';
import { fetchTemplateList, sendDirect } from '@/service/api/v1/message-hub';
import { fetchUserAutocomplete } from '@/service/api/v1/system-manage';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import { $t } from '@/locales';
import type { MessageHub } from '@/typings/api/v1/message-hub';
import type { ClientUser } from '@/typings/api/v1/client-user';
import ToastUiEditor from '@/components/custom/toast-ui-editor.vue';

defineOptions({
  name: 'SendEmail'
});

const { formRef, validate, restoreValidation } = useNaiveForm();
const { defaultRequiredRule } = useFormRules();

const loading = ref(false);
const templates = ref<MessageHub.Template[]>([]);
const users = ref<ClientUser.UserInfo[]>([]);
const templateLoading = ref(false);
const userLoading = ref(false);
let userSearchTimer: ReturnType<typeof setTimeout> | null = null;

const model = reactive({
  channel: 'email',
  receivers: [] as string[],
  templateCode: null as string | null,
  isCustom: false,
  title: '',
  content: '',
  templateVars: {} as Record<string, string>
});

const rules = {
  receivers: [defaultRequiredRule],
  title: [defaultRequiredRule],
  content: [defaultRequiredRule]
};

const templateVarKeys = computed(() => {
  if (!model.templateCode || model.isCustom) return [];
  const tpl = templates.value.find(item => item.code === model.templateCode);
  if (!tpl) return [];
  const matches = tpl.content.matchAll(/\{\{(.*?)\}\}/g);
  const keys: string[] = [];
  for (const match of matches) {
    const key = match[1].trim();
    if (key && !keys.includes(key)) {
      keys.push(key);
    }
  }
  return keys;
});

const userEmailOptions = computed(() =>
  users.value
    .filter(item => item.email)
    .map(item => ({
      label: `${item.nickName || item.userName} (${item.email})`,
      value: item.email
    }))
);

async function getTemplates() {
  templateLoading.value = true;
  const { data } = await fetchTemplateList({ current: 1, size: 100, channel: 'email', status: 1 });
  if (data) {
    templates.value = data.records;
  }
  templateLoading.value = false;
}

async function getUsers(query: string) {
  if (!query || query.trim().length === 0) {
    users.value = [];
    return;
  }
  userLoading.value = true;
  const { data } = await fetchUserAutocomplete(query.trim());
  if (data) {
    users.value = data;
  }
  userLoading.value = false;
}

function handleUserSearch(query: string) {
  if (userSearchTimer) clearTimeout(userSearchTimer);
  userSearchTimer = setTimeout(() => {
    getUsers(query);
  }, 300);
}

function handleTemplateChange(code: string | null) {
  model.templateVars = {};
  if (!code) {
    if (!model.isCustom) {
      model.title = '';
      model.content = '';
    }
    return;
  }
  const tpl = templates.value.find(item => item.code === code);
  if (tpl) {
    model.title = tpl.title || '';
    model.content = tpl.content;
  }
}

function renderContent() {
  if (!model.templateCode || model.isCustom) return model.content;
  let content = model.content;
  for (const [key, val] of Object.entries(model.templateVars)) {
    content = content.replaceAll(`{{${key}}}`, val || `{{${key}}}`);
  }
  return content;
}

function renderTitle() {
  if (!model.templateCode || model.isCustom) return model.title;
  let title = model.title;
  for (const [key, val] of Object.entries(model.templateVars)) {
    title = title.replaceAll(`{{${key}}}`, val || `{{${key}}}`);
  }
  return title;
}

async function handleSubmit() {
  await validate();
  loading.value = true;
  const content = renderContent();
  const title = renderTitle();
  let successCount = 0;
  let failCount = 0;
  const results = await Promise.allSettled(
    model.receivers.map(receiver => sendDirect({ channel: model.channel, receiver, title, content }))
  );
  for (const r of results) {
    if (r.status === 'fulfilled' && !r.value.error) {
      successCount += 1;
    } else {
      failCount += 1;
    }
  }
  loading.value = false;
  if (successCount > 0) {
    window.$message?.success(
      `${$t('common.sendSuccess')} (${successCount}${failCount > 0 ? `/${$t('common.fail')}${failCount}` : ''})`
    );
    resetForm();
  }
}

function resetForm() {
  model.receivers = [];
  model.templateCode = null;
  model.title = '';
  model.content = '';
  model.isCustom = false;
  model.templateVars = {};
  restoreValidation();
}

watch(
  () => model.isCustom,
  val => {
    if (!val && model.templateCode) {
      handleTemplateChange(model.templateCode);
    }
  }
);

getTemplates();
</script>

<template>
  <div class="h-full flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <NCard :title="$t('route.message_send_email')" :bordered="false" size="small" class="card-wrapper">
      <div class="mx-auto max-w-900px pt-20px">
        <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="120">
          <NFormItem :label="$t('page.messageHub.record.receiver')" path="receivers">
            <div class="w-full">
              <NSelect
                v-model:value="model.receivers"
                multiple
                filterable
                tag
                :options="userEmailOptions"
                :loading="userLoading"
                :placeholder="$t('page.messageHub.send.emailPlaceholder')"
                @search="handleUserSearch"
              />
              <div class="mt-4px text-12px text-gray-400">
                {{ $t('page.messageHub.send.emailHint') }}
              </div>
            </div>
          </NFormItem>
          <NFormItem :label="$t('page.messageHub.send.contentMode')" path="isCustom">
            <NSwitch v-model:value="model.isCustom">
              <template #checked>{{ $t('page.messageHub.send.customContent') }}</template>
              <template #unchecked>{{ $t('page.messageHub.send.templateContent') }}</template>
            </NSwitch>
          </NFormItem>
          <NFormItem v-if="!model.isCustom" :label="$t('page.messageHub.template.title')" path="templateCode">
            <NSelect
              v-model:value="model.templateCode"
              :options="templates.map(item => ({ label: item.name, value: item.code }))"
              :loading="templateLoading"
              :placeholder="$t('page.messageHub.send.selectTemplate')"
              clearable
              @update:value="handleTemplateChange"
            />
          </NFormItem>
          <template v-if="templateVarKeys.length > 0 && !model.isCustom">
            <NFormItem v-for="varKey in templateVarKeys" :key="varKey" :label="varKey" :path="`templateVars.${varKey}`">
              <NInput
                v-model:value="model.templateVars[varKey]"
                :placeholder="`${$t('common.pleaseInput')}${varKey}`"
              />
            </NFormItem>
          </template>
          <NFormItem :label="$t('page.messageHub.template.msgTitle')" path="title">
            <NInput
              v-model:value="model.title"
              :disabled="!model.isCustom"
              :placeholder="$t('page.messageHub.send.emailTitlePlaceholder')"
            />
          </NFormItem>
          <NFormItem :label="$t('page.messageHub.template.content')" path="content">
            <div class="w-full">
              <ToastUiEditor v-if="model.isCustom" v-model="model.content" height="400px" initial-edit-type="wysiwyg" />
              <NInput
                v-else
                v-model:value="model.content"
                type="textarea"
                disabled
                :autosize="{ minRows: 10, maxRows: 20 }"
                :placeholder="$t('page.messageHub.send.templateAutoFill')"
              />
            </div>
          </NFormItem>
          <NFormItem>
            <NSpace justify="end" class="w-full">
              <NButton @click="resetForm">{{ $t('common.reset') }}</NButton>
              <NButton type="primary" :loading="loading" @click="handleSubmit">
                {{ $t('common.confirm') }}
              </NButton>
            </NSpace>
          </NFormItem>
        </NForm>
      </div>
    </NCard>
  </div>
</template>
