<script setup lang="ts">
import { ref, watch } from 'vue';
import type { FormRules, UploadFileInfo } from 'naive-ui';
import { fetchCreateUploadRecord, fetchGetUploadCredentials } from '@/service/api/v1/storage';
import { useNaiveForm } from '@/hooks/common/form';
import { uploadWithPresignedUrl } from '@/utils/upload';
import { $t } from '@/locales';

defineOptions({
  name: 'StorageTestUploadModal'
});

interface Props {
  configOptions: { label: string; value: number }[];
}

const props = defineProps<Props>();

const visible = defineModel<boolean>('visible', {
  default: false
});

const { formRef, validate, restoreValidation } = useNaiveForm();

const fileListRef = ref<UploadFileInfo[]>([]);

const model = ref({
  configId: 0
});

const rules: FormRules = {
  configId: {
    trigger: 'change',
    validator: (rule, value) => {
      if (!value || value <= 0) {
        return new Error($t('page.settings.storageTest.form.selectConfig'));
      }
      return true;
    }
  }
};

const loading = ref(false);
const uploadStatus = ref<'idle' | 'get-credential' | 'uploading' | 'success' | 'error'>('idle');
const uploadProgress = ref(0);
const resultUrl = ref('');
const errorMessage = ref('');

async function handleSubmit() {
  await validate();

  if (fileListRef.value.length === 0) {
    window.$message?.warning($t('page.settings.storageTest.form.selectFile'));
    return;
  }

  const nativeFile = fileListRef.value[0].file;
  if (!nativeFile) {
    window.$message?.warning($t('page.settings.storageTest.form.fileReading'));
    return;
  }

  loading.value = true;
  uploadStatus.value = 'get-credential';
  resultUrl.value = '';
  errorMessage.value = '';

  try {
    const file = nativeFile as File;

    const { data: credentials, error } = await fetchGetUploadCredentials({
      configId: model.value.configId,
      fileName: file.name,
      fileSize: file.size,
      contentType: file.type || 'application/octet-stream',
      businessType: 'storage_test'
    });

    if (error || !credentials) {
      throw new Error(error?.message || $t('page.settings.storageTest.getCredentialFailed'));
    }

    uploadStatus.value = 'uploading';
    uploadProgress.value = 0;
    const uploadUrl = await uploadWithPresignedUrl(credentials, file, progress => {
      uploadProgress.value = progress.percent;
    });

    await fetchCreateUploadRecord({
      configId: credentials.configId,
      fileName: file.name,
      objectKey: credentials.objectKey,
      fileSize: file.size,
      mimeType: file.type,
      businessType: 'storage_test'
    });

    resultUrl.value = uploadUrl;
    uploadStatus.value = 'success';
    window.$message?.success($t('page.settings.storageTest.verifySuccess'));
  } catch (err: any) {
    uploadStatus.value = 'error';
    errorMessage.value = err.message || $t('common.error');
  } finally {
    loading.value = false;
  }
}

function handleClose() {
  visible.value = false;
  uploadStatus.value = 'idle';
  resultUrl.value = '';
  errorMessage.value = '';
  fileListRef.value = [];
}

watch(visible, val => {
  if (val) {
    if (props.configOptions.length === 1) {
      model.value.configId = props.configOptions[0].value;
    }
    setTimeout(() => restoreValidation(), 50);
  }
});
</script>

<template>
  <NModal
    v-model:show="visible"
    :title="$t('page.settings.storageTest.title')"
    preset="card"
    display-directive="show"
    class="max-w-94vw w-520px"
  >
    <NSpin :show="loading">
      <NForm ref="formRef" :model="model" :rules="rules" label-placement="top">
        <NFormItem :label="$t('page.settings.storageTest.selectConfig')" path="configId">
          <NSelect v-model:value="model.configId" :options="configOptions" :placeholder="$t('common.pleaseSelect')" />
        </NFormItem>
        <NFormItem :label="$t('page.settings.storageTest.selectFile')">
          <NUpload
            v-model:file-list="fileListRef"
            :show-file-list="true"
            :max="1"
            accept="image/*,.txt,.pdf"
            :default-upload="false"
          >
            <NButton>
              <template #icon>
                <SvgIcon icon="mdi:upload" />
              </template>
              {{ $t('page.settings.storageTest.selectFile') }}
            </NButton>
          </NUpload>
        </NFormItem>

        <NFormItem v-if="uploadStatus === 'get-credential'">
          <div class="rounded-6px bg-blue-50 p-12px">
            <div class="text-info">🔑 {{ $t('page.settings.storageTest.getCredential') }}</div>
          </div>
        </NFormItem>

        <NFormItem v-if="uploadStatus === 'uploading'" :label="$t('page.settings.storageTest.uploading')">
          <div class="rounded-6px bg-blue-50 p-12px">
            <div class="mb-8px text-info">📤 {{ $t('page.settings.storageTest.uploading') }} {{ uploadProgress }}%</div>
            <NProgress type="line" :percentage="uploadProgress" indicator-placement="inside" :stroke-width="16" />
          </div>
        </NFormItem>

        <NFormItem v-if="uploadStatus === 'success'" :label="$t('page.settings.storageTest.result')">
          <div class="rounded-6px bg-green-50 p-12px">
            <div class="mb-8px text-success">✅ {{ $t('page.settings.storageTest.verifySuccess') }}</div>
            <div class="mb-8px text-sm text-gray">{{ $t('page.settings.storageTest.verifySuccessTips') }}</div>
            <a :href="resultUrl" target="_blank" rel="noopener noreferrer" class="break-all text-primary">
              {{ resultUrl }}
            </a>
          </div>
        </NFormItem>

        <NFormItem v-if="uploadStatus === 'error'" :label="$t('page.settings.storageTest.result')">
          <div class="rounded-6px bg-red-50 p-12px">
            <div class="mb-8px text-error">❌ {{ $t('page.settings.storageTest.verifyFailed') }}</div>
            <div class="text-sm text-error">{{ errorMessage }}</div>
          </div>
        </NFormItem>
      </NForm>
    </NSpin>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="handleClose">{{ $t('page.settings.storageTest.close') }}</NButton>
        <NButton type="primary" :loading="loading" @click="handleSubmit">
          {{ $t('page.settings.storageTest.startTest') }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
