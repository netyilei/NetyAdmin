<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import {
  NAlert,
  NCard,
  NDivider,
  NFormItem,
  NGrid,
  NGridItem,
  NInputNumber,
  NSpace,
  NSpin,
  NSwitch,
  NTabPane,
  NTabs
} from 'naive-ui';
import { fetchGetSysConfigs, fetchUpdateSysConfig } from '@/service/api/v1/system-manage';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

const loading = ref(false);
const updating = ref<string>('');

const cacheConfigs = ref<SystemManage.SysConfig[]>([]);

interface ConfigItem extends SystemManage.SysConfig {
  numValue?: number;
}

const systemCaches = computed(() => cacheConfigs.value.filter(c => c.isSystem));
const moduleCaches = computed(() => cacheConfigs.value.filter(c => !c.isSystem));

const logConfigs = reactive<Record<string, ConfigItem>>({
  task_log_enabled: {} as ConfigItem,
  task_retention: {} as ConfigItem,
  ops_retention: {} as ConfigItem,
  err_retention: {} as ConfigItem
});

async function init() {
  loading.value = true;

  // 1. 加载所有相关配置
  const results = await Promise.all([
    fetchGetSysConfigs('cache_switches'),
    fetchGetSysConfigs('task_config'),
    fetchGetSysConfigs('ops_config'),
    fetchGetSysConfigs('error_config')
  ]);

  if (!results[0].error) cacheConfigs.value = results[0].data;

  if (!results[1].error) {
    results[1].data.forEach(item => {
      if (item.configKey === 'log_enabled') logConfigs.task_log_enabled = item;
      if (item.configKey === 'retention_days') {
        logConfigs.task_retention = { ...item, numValue: Number.parseInt(item.configValue, 10) || 0 };
      }
    });
  }

  if (!results[2].error) {
    results[2].data.forEach(item => {
      if (item.configKey === 'retention_days') {
        logConfigs.ops_retention = { ...item, numValue: Number.parseInt(item.configValue, 10) || 0 };
      }
    });
  }

  if (!results[3].error) {
    results[3].data.forEach(item => {
      if (item.configKey === 'retention_days') {
        logConfigs.err_retention = { ...item, numValue: Number.parseInt(item.configValue, 10) || 0 };
      }
    });
  }

  loading.value = false;
}

async function handleUpdate(item: SystemManage.SysConfig) {
  if (!item || !item.configKey) return;
  updating.value = item.configKey;
  const { error } = await fetchUpdateSysConfig(item);
  updating.value = '';
  if (!error) {
    window.$message?.success($t('common.updateSuccess'));
  }
}

async function handleNumberUpdate(item: ConfigItem) {
  if (!item || item.numValue === undefined || item.numValue === Number.parseInt(item.configValue, 10)) return;
  item.configValue = item.numValue.toString();
  await handleUpdate(item);
}

onMounted(init);
</script>

<template>
  <div class="h-full">
    <NCard :bordered="false" class="h-full rounded-8px shadow-sm">
      <NTabs type="line" animated>
        <!-- Tab 1: Cache Management -->
        <NTabPane name="cache" :tab="$t('page.manage.setting.tabs.cache')">
          <div class="pt-4">
            <NAlert type="info" class="mb-6">
              {{ $t('page.manage.setting.cache.description') }}
            </NAlert>
            <NSpin :show="loading">
              <div v-if="systemCaches.length > 0">
                <NDivider title-placement="left">
                  <span class="text-13px text-gray-500 font-bold tracking-wider uppercase">
                    {{ $t('page.manage.setting.cache.systemGroup') }}
                  </span>
                </NDivider>
                <NGrid :x-gap="16" :y-gap="16" cols="1 s:2 m:3" responsive="screen">
                  <NGridItem v-for="item in systemCaches" :key="item.configKey">
                    <NCard size="small" class="cursor-default transition-colors hover:border-primary">
                      <div class="flex-y-center justify-between">
                        <div>
                          <div class="mb-1 flex items-center text-16px font-bold">
                            {{ $t(`page.manage.setting.cache.${item.configKey}`) }}
                          </div>
                          <div class="text-12px text-gray-400">Key: {{ item.configKey }}</div>
                        </div>
                        <NSwitch
                          v-model:value="item.configValue"
                          checked-value="true"
                          unchecked-value="false"
                          :loading="updating === item.configKey"
                          @update:value="handleUpdate(item)"
                        />
                      </div>
                    </NCard>
                  </NGridItem>
                </NGrid>
              </div>

              <div v-if="moduleCaches.length > 0" class="mt-8">
                <NDivider title-placement="left">
                  <span class="text-13px text-gray-500 font-bold tracking-wider uppercase">
                    {{ $t('page.manage.setting.cache.moduleGroup') }}
                  </span>
                </NDivider>
                <NGrid :x-gap="16" :y-gap="16" cols="1 s:2 m:3" responsive="screen">
                  <NGridItem v-for="item in moduleCaches" :key="item.configKey">
                    <NCard size="small" class="cursor-default transition-colors hover:border-primary">
                      <div class="flex-y-center justify-between">
                        <div>
                          <div class="mb-1 text-16px font-bold">
                            {{ $t(`page.manage.setting.cache.${item.configKey}`) }}
                          </div>
                          <div class="text-12px text-gray-400">Key: {{ item.configKey }}</div>
                        </div>
                        <NSwitch
                          v-model:value="item.configValue"
                          checked-value="true"
                          unchecked-value="false"
                          :loading="updating === item.configKey"
                          @update:value="handleUpdate(item)"
                        />
                      </div>
                    </NCard>
                  </NGridItem>
                </NGrid>
              </div>
            </NSpin>
          </div>
        </NTabPane>

        <!-- Tab 2: Task Configuration -->
        <NTabPane name="task" :tab="$t('page.manage.setting.tabs.task')">
          <div class="max-w-600px pt-4">
            <NCard :title="$t('page.manage.setting.log.taskLog')" size="small">
              <NSpace vertical :size="20">
                <NFormItem :label="$t('page.manage.setting.log.enabled')" label-placement="left">
                  <NSwitch
                    v-if="logConfigs.task_log_enabled"
                    v-model:value="logConfigs.task_log_enabled.configValue"
                    checked-value="true"
                    unchecked-value="false"
                    :loading="updating === logConfigs.task_log_enabled?.configKey"
                    @update:value="handleUpdate(logConfigs.task_log_enabled)"
                  />
                </NFormItem>
                <NFormItem :label="$t('page.manage.setting.log.retentionDays')" label-placement="left">
                  <NInputNumber
                    v-if="logConfigs.task_retention"
                    v-model:value="logConfigs.task_retention.numValue"
                    :min="0"
                    :max="3650"
                    class="w-200px"
                    @blur="handleNumberUpdate(logConfigs.task_retention)"
                  >
                    <template #suffix>{{ $t('page.manage.setting.log.daysUnit') }}</template>
                  </NInputNumber>
                </NFormItem>
                <NAlert type="info" size="small">
                  {{ $t('page.manage.setting.log.description') }}
                </NAlert>
              </NSpace>
            </NCard>
          </div>
        </NTabPane>

        <!-- Tab 3: Log Maintenance -->
        <NTabPane name="log" :tab="$t('page.manage.setting.tabs.log')">
          <div class="max-w-1000px pt-4">
            <NGrid :x-gap="24" :y-gap="24" cols="1 s:1 m:2" responsive="screen">
              <NGridItem>
                <!-- Ops Logs -->
                <NCard :title="$t('page.manage.setting.log.operationLog')" size="small">
                  <NFormItem :label="$t('page.manage.setting.log.retentionDays')">
                    <NInputNumber
                      v-if="logConfigs.ops_retention"
                      v-model:value="logConfigs.ops_retention.numValue"
                      :min="0"
                      :max="3650"
                      class="w-full"
                      @blur="handleNumberUpdate(logConfigs.ops_retention)"
                    >
                      <template #suffix>{{ $t('page.manage.setting.log.daysUnit') }}</template>
                    </NInputNumber>
                  </NFormItem>
                </NCard>
              </NGridItem>

              <NGridItem>
                <!-- Error Logs -->
                <NCard :title="$t('page.manage.setting.log.errorLog')" size="small">
                  <NFormItem :label="$t('page.manage.setting.log.retentionDays')">
                    <NInputNumber
                      v-if="logConfigs.err_retention"
                      v-model:value="logConfigs.err_retention.numValue"
                      :min="0"
                      :max="3650"
                      class="w-full"
                      @blur="handleNumberUpdate(logConfigs.err_retention)"
                    >
                      <template #suffix>{{ $t('page.manage.setting.log.daysUnit') }}</template>
                    </NInputNumber>
                  </NFormItem>
                </NCard>
              </NGridItem>
            </NGrid>
          </div>
        </NTabPane>
      </NTabs>
    </NCard>
  </div>
</template>

<style scoped>
.flex-y-center {
  display: flex;
  align-items: center;
}
</style>
