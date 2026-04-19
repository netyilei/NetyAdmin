<script setup lang="ts">
import { reactive, ref, watch } from 'vue';
import { NButton, NCard, NForm, NFormItem, NInput, NRadio, NRadioGroup, NSelect, NSpace, NSwitch } from 'naive-ui';
import { fetchTemplateList, sendDirect } from '@/service/api/v1/message-hub';
import { fetchGetAdminList } from '@/service/api/v1/system-manage';
import { useFormRules, useNaiveForm } from '@/hooks/common/form';
import { $t } from '@/locales';
import type { MessageHub } from '@/typings/api/v1/message-hub';

defineOptions({
  name: 'SendInternal'
});

const { formRef, validate, restoreValidation } = useNaiveForm();
const { defaultRequiredRule } = useFormRules();

const loading = ref(false);
const templates = ref<MessageHub.Template[]>([]);
const admins = ref<any[]>([]);
const templateLoading = ref(false);
const adminLoading = ref(false);
let adminSearchTimer: ReturnType<typeof setTimeout> | null = null;

const model = reactive({
  channel: 'internal',
  sendType: 'direct' as 'direct' | 'broadcast',
  receiver: '',
  templateCode: null as string | null,
  isCustom: false,
  title: '',
  content: ''
});

const rules = {
  receiver: [
    {
      required: true,
      message: '请选择接收人',
      trigger: ['blur', 'change'],
      condition: () => model.sendType === 'direct'
    }
  ],
  title: [defaultRequiredRule],
  content: [defaultRequiredRule]
};

async function getTemplates() {
  templateLoading.value = true;
  const { data } = await fetchTemplateList({ current: 1, size: 100, channel: 'internal', status: 1 });
  if (data) {
    templates.value = data.records;
  }
  templateLoading.value = false;
}

async function getAdmins(query: string) {
  if (!query || query.trim().length === 0) {
    admins.value = [];
    return;
  }
  adminLoading.value = true;
  const { data } = await fetchGetAdminList({ current: 1, size: 20, userName: query.trim() });
  if (data) {
    admins.value = data.records;
  }
  adminLoading.value = false;
}

function handleAdminSearch(query: string) {
  if (adminSearchTimer) clearTimeout(adminSearchTimer);
  adminSearchTimer = setTimeout(() => {
    getAdmins(query);
  }, 300);
}

function handleTemplateChange(code: string | null) {
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

async function handleSubmit() {
  await validate();
  loading.value = true;
  const { error } = await sendDirect({
    channel: model.channel,
    receiver: model.sendType === 'broadcast' ? 'all' : model.receiver,
    title: model.title,
    content: model.content
  });
  loading.value = false;
  if (!error) {
    window.$message?.success($t('common.sendSuccess'));
    resetForm();
  }
}

function resetForm() {
  model.receiver = '';
  model.templateCode = null;
  model.title = '';
  model.content = '';
  model.isCustom = false;
  model.sendType = 'direct';
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
    <NCard :title="$t('route.message_send_internal')" :bordered="false" size="small" class="card-wrapper">
      <div class="mx-auto max-w-800px pt-40px">
        <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="120">
          <NFormItem label="发送类型" path="sendType">
            <NRadioGroup v-model:value="model.sendType">
              <NSpace>
                <NRadio value="direct">私信 (指定用户)</NRadio>
                <NRadio value="broadcast">公告 (全员广播)</NRadio>
              </NSpace>
            </NRadioGroup>
          </NFormItem>
          <NFormItem v-if="model.sendType === 'direct'" :label="$t('page.messageHub.record.receiver')" path="receiver">
            <NSelect
              v-model:value="model.receiver"
              filterable
              placeholder="输入用户名搜索"
              :options="
                admins.map(item => ({
                  label: item.nickname ? `${item.username} (${item.nickname})` : item.username,
                  value: String(item.id)
                }))
              "
              :loading="adminLoading"
              @search="handleAdminSearch"
            />
          </NFormItem>
          <NFormItem :label="$t('page.messageHub.template.title')" path="templateCode">
            <NSelect
              v-model:value="model.templateCode"
              :options="templates.map(item => ({ label: item.name, value: item.code }))"
              :loading="templateLoading"
              placeholder="请选择模板"
              clearable
              @update:value="handleTemplateChange"
            />
          </NFormItem>
          <NFormItem label="手动编写" path="isCustom">
            <NSwitch v-model:value="model.isCustom" />
          </NFormItem>
          <NFormItem :label="$t('page.messageHub.template.msgTitle')" path="title">
            <NInput v-model:value="model.title" :disabled="!model.isCustom" placeholder="请输入消息标题" />
          </NFormItem>
          <NFormItem :label="$t('page.messageHub.template.content')" path="content">
            <NInput
              v-model:value="model.content"
              type="textarea"
              :disabled="!model.isCustom"
              :autosize="{ minRows: 4, maxRows: 8 }"
              :placeholder="model.isCustom ? '请输入消息内容' : '选择模板后自动填充'"
            />
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
