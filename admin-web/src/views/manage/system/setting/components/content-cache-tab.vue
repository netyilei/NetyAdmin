<!-- eslint-disable vue/no-mutating-props -->
<script setup lang="ts">
/* eslint-disable vue/no-mutating-props */
import { NAlert, NFormItem, NInputNumber, NSpace, NSpin } from 'naive-ui';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { $t } from '@/locales';

export interface ConfigItem extends SystemManage.SysConfig {
  numValue?: number;
}

defineProps<{
  loading: boolean;
  contentCacheConfigs: Record<string, ConfigItem | undefined>;
}>();

const emit = defineEmits<{
  (e: 'numberUpdate', item: ConfigItem): void;
}>();
</script>

<template>
  <div class="max-w-600px pt-4">
    <NAlert type="info" class="mb-6">
      {{ $t('page.manage.setting.contentCache.description') }}
    </NAlert>
    <NSpin :show="loading">
      <NSpace vertical :size="20">
        <NFormItem :label="$t('page.manage.setting.contentCache.bannerCacheTTL')" label-placement="left">
          <NInputNumber
            v-if="contentCacheConfigs.banner_cache_ttl"
            v-model:value="contentCacheConfigs.banner_cache_ttl.numValue"
            :min="1"
            :max="1440"
            class="w-240px"
            @blur="emit('numberUpdate', contentCacheConfigs.banner_cache_ttl)"
          />
          <span class="ml-2 text-12px text-gray-400">{{ $t('page.manage.setting.contentCache.minutesUnit') }}</span>
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.contentCache.categoryCacheTTL')" label-placement="left">
          <NInputNumber
            v-if="contentCacheConfigs.category_cache_ttl"
            v-model:value="contentCacheConfigs.category_cache_ttl.numValue"
            :min="1"
            :max="1440"
            class="w-240px"
            @blur="emit('numberUpdate', contentCacheConfigs.category_cache_ttl)"
          />
          <span class="ml-2 text-12px text-gray-400">{{ $t('page.manage.setting.contentCache.minutesUnit') }}</span>
        </NFormItem>
        <NFormItem :label="$t('page.manage.setting.contentCache.articleCacheTTL')" label-placement="left">
          <NInputNumber
            v-if="contentCacheConfigs.article_cache_ttl"
            v-model:value="contentCacheConfigs.article_cache_ttl.numValue"
            :min="1"
            :max="1440"
            class="w-240px"
            @blur="emit('numberUpdate', contentCacheConfigs.article_cache_ttl)"
          />
          <span class="ml-2 text-12px text-gray-400">{{ $t('page.manage.setting.contentCache.minutesUnit') }}</span>
        </NFormItem>
      </NSpace>
    </NSpin>
  </div>
</template>
