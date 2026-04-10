<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import type { FormRules } from 'naive-ui';
import { fetchCreateStorageConfig, fetchGetStorageConfig, fetchUpdateStorageConfig } from '@/service/api/v1/storage';
import { useNaiveForm } from '@/hooks/common/form';
import type { Storage } from '@/typings/api/v1/storage';
import { $t } from '@/locales';

defineOptions({
  name: 'StorageConfigOperateModal'
});

interface Props {
  operateType: NaiveUI.TableOperateType;
  rowData?: Storage.StorageConfig | null;
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

const title = computed(() => {
  const titles: Record<NaiveUI.TableOperateType, string> = {
    add: $t('page.manage.storage.addConfig'),
    edit: $t('page.manage.storage.editConfig')
  };
  return titles[props.operateType];
});

const model = reactive<Storage.CreateStorageConfigParams & { id?: number }>({
  name: '',
  provider: 'aliyun',
  endpoint: '',
  region: '',
  bucket: '',
  accessKey: '',
  secretKey: '',
  domain: '',
  pathPrefix: '',
  isDefault: false,
  status: '1',
  maxFileSize: 104857600,
  allowedTypes: '',
  stsExpireTime: 3600,
  remark: ''
});

const providerOptions = [
  { label: '阿里云 OSS', value: 'aliyun' },
  { label: '腾讯云 COS', value: 'tencent' },
  { label: '华为云 OBS', value: 'huawei' },
  { label: '七牛云', value: 'qiniu' },
  { label: 'MinIO', value: 'minio' },
  { label: 'AWS S3', value: 'aws' },
  { label: 'Cloudflare R2', value: 'cloudflare' },
  { label: '自定义', value: 'custom' }
];

const isEdit = computed(() => props.operateType === 'edit');

const rules = computed<FormRules>(() => ({
  name: {
    required: true,
    message: $t('common.pleaseInput') + $t('page.manage.storage.configName'),
    trigger: 'blur'
  },
  provider: {
    required: true,
    message: $t('common.pleaseSelect') + $t('page.manage.storage.provider'),
    trigger: 'change'
  },
  endpoint: {
    required: true,
    message: $t('common.pleaseInput') + $t('page.manage.storage.endpoint'),
    trigger: 'blur'
  },
  bucket: {
    required: true,
    message: $t('common.pleaseInput') + $t('page.manage.storage.bucket'),
    trigger: 'blur'
  },
  accessKey: {
    required: true,
    message: $t('common.pleaseInput') + $t('page.manage.storage.accessKey'),
    trigger: 'blur'
  },
  secretKey: {
    required: !isEdit.value,
    message: isEdit.value
      ? $t('page.manage.storage.secretKeyPlaceholder')
      : $t('common.pleaseInput') + $t('page.manage.storage.secretKey'),
    trigger: 'blur'
  }
}));

const loading = ref(false);

function handleUpdateProvider(value: Storage.StorageProvider) {
  const endpointMap: Record<Storage.StorageProvider, string> = {
    aliyun: 'https://oss-cn-hangzhou.aliyuncs.com',
    tencent: 'https://cos.ap-guangzhou.myqcloud.com',
    huawei: 'https://obs.cn-north-4.myhuaweicloud.com',
    qiniu: 'https://s3-cn-south-1.qiniucs.com',
    minio: '',
    aws: 'https://s3.us-east-1.amazonaws.com',
    cloudflare: '',
    custom: ''
  };
  model.endpoint = endpointMap[value] || '';
}

async function handleSubmit() {
  await validate();

  loading.value = true;
  try {
    if (props.operateType === 'add') {
      const { error } = await fetchCreateStorageConfig(model);
      if (!error) {
        window.$message?.success($t('common.addSuccess'));
        closeModal();
        emit('submitted');
      }
    } else {
      const { error } = await fetchUpdateStorageConfig({ ...model, id: model.id! });
      if (!error) {
        window.$message?.success($t('common.updateSuccess'));
        closeModal();
        emit('submitted');
      }
    }
  } finally {
    loading.value = false;
  }
}

function closeModal() {
  visible.value = false;
}

function initModel() {
  Object.assign(model, {
    name: '',
    provider: 'aliyun',
    endpoint: 'https://oss-cn-hangzhou.aliyuncs.com',
    region: '',
    bucket: '',
    accessKey: '',
    secretKey: '',
    domain: '',
    pathPrefix: '',
    isDefault: false,
    status: '1',
    maxFileSize: 104857600,
    allowedTypes: '',
    stsExpireTime: 3600,
    remark: ''
  });
}

async function getDetail(id: number) {
  loading.value = true;
  try {
    const { data, error } = await fetchGetStorageConfig(id);
    if (!error && data) {
      Object.assign(model, {
        ...data,
        secretKey: ''
      });
    }
  } finally {
    loading.value = false;
  }
}

watch(visible, val => {
  if (val) {
    if (props.operateType === 'add') {
      initModel();
    } else if (props.rowData?.id) {
      getDetail(props.rowData.id);
    }
    restoreValidation();
  }
});
</script>

<template>
  <NModal
    v-model:show="visible"
    :title="title"
    preset="card"
    display-directive="show"
    :style="{ width: '800px', maxWidth: '94vw' }"
    class="storage-config-modal"
  >
    <NSpin :show="loading">
      <NForm ref="formRef" :model="model" :rules="rules" label-placement="left" :label-width="120">
        <NFormItem :label="$t('page.manage.storage.configName')" path="name">
          <NInput v-model:value="model.name" :placeholder="$t('common.pleaseInput')" />
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.provider')" path="provider">
          <NSelect
            v-model:value="model.provider"
            :options="providerOptions"
            :placeholder="$t('common.pleaseSelect')"
            :disabled="operateType === 'edit'"
            @update:value="handleUpdateProvider"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.endpoint')" path="endpoint">
          <NInput v-model:value="model.endpoint" :placeholder="$t('common.pleaseInput')" />
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.region')" path="region">
          <NInput v-model:value="model.region" :placeholder="$t('common.pleaseInput')" />
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.bucket')" path="bucket">
          <NInput v-model:value="model.bucket" :placeholder="$t('common.pleaseInput')" />
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.accessKey')" path="accessKey">
          <NInput v-model:value="model.accessKey" :placeholder="$t('common.pleaseInput')" />
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.secretKey')" path="secretKey">
          <NInput
            v-model:value="model.secretKey"
            type="password"
            show-password-on="click"
            :placeholder="
              operateType === 'edit' ? $t('page.manage.storage.secretKeyPlaceholder') : $t('common.pleaseInput')
            "
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.domain')" path="domain">
          <NInput v-model:value="model.domain" :placeholder="$t('common.pleaseInput')" />
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.pathPrefix')" path="pathPrefix">
          <NInput v-model:value="model.pathPrefix" :placeholder="$t('common.pleaseInput')" />
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.maxFileSize')" path="maxFileSize">
          <NInputNumber v-model:value="model.maxFileSize" :min="1" :max="10737418240" class="w-full">
            <template #suffix>bytes</template>
          </NInputNumber>
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.allowedTypes')" path="allowedTypes">
          <NInput
            v-model:value="model.allowedTypes"
            type="textarea"
            :placeholder="$t('page.manage.storage.allowedTypesPlaceholder')"
            :autosize="{ minRows: 2, maxRows: 4 }"
          />
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.stsExpireTime')" path="stsExpireTime">
          <NInputNumber v-model:value="model.stsExpireTime" :min="60" :max="86400" class="w-full">
            <template #suffix>秒</template>
          </NInputNumber>
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.isDefault')" path="isDefault">
          <NSwitch v-model:value="model.isDefault" />
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.status')" path="status">
          <AppDictRadioGroup v-model:value="model.status" dict-code="sys_status" />
        </NFormItem>
        <NFormItem :label="$t('page.manage.storage.remark')" path="remark">
          <NInput
            v-model:value="model.remark"
            type="textarea"
            :placeholder="$t('common.pleaseInput')"
            :autosize="{ minRows: 2, maxRows: 4 }"
          />
        </NFormItem>
      </NForm>
    </NSpin>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="closeModal">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="loading" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
:deep(.n-card) {
  max-height: 90vh;
  display: flex;
  flex-direction: column;
}

:deep(.n-card__content) {
  overflow-y: auto;
  padding-right: 4px;
}

:deep(.n-card__content::-webkit-scrollbar) {
  width: 6px;
}

:deep(.n-card__content::-webkit-scrollbar-thumb) {
  background-color: #c0c4cc;
  border-radius: 3px;
}

@media (max-width: 768px) {
  :deep(.n-card) {
    max-height: 94vh;
    margin: 0 8px;
  }

  :deep(.n-form-item-label) {
    width: 100% !important;
    text-align: left !important;
    padding-bottom: 4px;
  }

  :deep(.n-form-item-label-label) {
    white-space: normal;
  }
}
</style>
