<!-- eslint-disable vue/no-mutating-props -->
<script setup lang="ts">
/* eslint-disable vue/no-mutating-props */
import { NAlert, NCard, NDivider, NGrid, NGridItem, NSpin, NSwitch } from 'naive-ui';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

defineProps<{
  loading: boolean;
  updating: string;
  systemCaches: SystemManage.SysConfig[];
  moduleCaches: SystemManage.SysConfig[];
}>();

const emit = defineEmits<{
  (e: 'update', item: SystemManage.SysConfig): void;
}>();
</script>

<template>
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
                  @update:value="emit('update', item)"
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
                  @update:value="emit('update', item)"
                />
              </div>
            </NCard>
          </NGridItem>
        </NGrid>
      </div>
    </NSpin>
  </div>
</template>

<style scoped>
.flex-y-center {
  display: flex;
  align-items: center;
}
</style>
