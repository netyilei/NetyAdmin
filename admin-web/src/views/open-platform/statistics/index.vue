<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { NButton, NCard, NDatePicker, NDescriptions, NDescriptionsItem, NGi, NGrid, NRadioButton, NRadioGroup, NSelect, NSpace } from 'naive-ui';
import dayjs from 'dayjs';
import { useAppStore } from '@/store/modules/app';
import { useEcharts } from '@/hooks/common/echarts';
import { $t } from '@/locales';
import { fetchGetOpenLogStatistics } from '@/service/api/v1/log';
import { fetchAppList } from '@/service/api/v1/open-app';
import type { Log } from '@/typings/api/v1/log';

defineOptions({
  name: 'OpenPlatformStatistics'
});

const appStore = useAppStore();

const loading = ref(false);
const dateRange = ref<[number, number]>([dayjs().subtract(7, 'day').valueOf(), dayjs().valueOf()]);
const granularity = ref<'day' | 'week' | 'month'>('day');
const appId = ref<string | null>(null);
const appOptions = ref<{ label: string; value: string }[]>([]);

const overview = ref<Log.OverviewStats | null>(null);
const trend = ref<Log.TrendItem[]>([]);
const topApps = ref<Log.AppStatItem[]>([]);
const topApis = ref<Log.ApiStatItem[]>([]);
const statusDist = ref<Log.StatusDistItem[]>([]);
const latencyStats = ref<Log.LatencyStats | null>(null);

const { domRef: trendDomRef, updateOptions: updateTrendOptions } = useEcharts(() => ({
  tooltip: {
    trigger: 'axis',
    axisPointer: {
      type: 'cross',
      label: {
        backgroundColor: '#6a7985'
      }
    }
  },
  legend: {
    data: [$t('page.openPlatform.statistics.totalCalls'), $t('page.openPlatform.statistics.successCalls')]
  },
  grid: {
    left: '3%',
    right: '4%',
    bottom: '3%',
    containLabel: true
  },
  xAxis: {
    type: 'category',
    boundaryGap: false,
    data: [] as string[]
  },
  yAxis: {
    type: 'value'
  },
  series: [
    {
      color: '#8e9dff',
      name: $t('page.openPlatform.statistics.totalCalls'),
      type: 'line',
      smooth: true,
      areaStyle: {
        color: {
          type: 'linear',
          x: 0,
          y: 0,
          x2: 0,
          y2: 1,
          colorStops: [
            { offset: 0.25, color: '#8e9dff' },
            { offset: 1, color: '#fff' }
          ]
        }
      },
      emphasis: { focus: 'series' },
      data: [] as number[]
    },
    {
      color: '#26deca',
      name: $t('page.openPlatform.statistics.successCalls'),
      type: 'line',
      smooth: true,
      areaStyle: {
        color: {
          type: 'linear',
          x: 0,
          y: 0,
          x2: 0,
          y2: 1,
          colorStops: [
            { offset: 0.25, color: '#26deca' },
            { offset: 1, color: '#fff' }
          ]
        }
      },
      emphasis: { focus: 'series' },
      data: [] as number[]
    }
  ]
}));

const { domRef: topAppsDomRef, updateOptions: updateTopAppsOptions } = useEcharts(() => ({
  tooltip: {
    trigger: 'axis',
    axisPointer: { type: 'shadow' }
  },
  grid: {
    left: '3%',
    right: '4%',
    bottom: '3%',
    containLabel: true
  },
  xAxis: {
    type: 'value'
  },
  yAxis: {
    type: 'category',
    data: [] as string[]
  },
  series: [
    {
      type: 'bar',
      color: '#8e9dff',
      data: [] as number[],
      label: {
        show: true,
        position: 'right'
      }
    }
  ]
}));

const { domRef: topApisDomRef, updateOptions: updateTopApisOptions } = useEcharts(() => ({
  tooltip: {
    trigger: 'axis',
    axisPointer: { type: 'shadow' }
  },
  grid: {
    left: '3%',
    right: '4%',
    bottom: '3%',
    containLabel: true
  },
  xAxis: {
    type: 'value'
  },
  yAxis: {
    type: 'category',
    data: [] as string[]
  },
  series: [
    {
      type: 'bar',
      color: '#5da8ff',
      data: [] as number[],
      label: {
        show: true,
        position: 'right'
      }
    }
  ]
}));

const { domRef: statusDistDomRef, updateOptions: updateStatusDistOptions } = useEcharts(() => ({
  tooltip: {
    trigger: 'item'
  },
  legend: {
    bottom: '1%',
    left: 'center',
    itemStyle: { borderWidth: 0 }
  },
  series: [
    {
      color: ['#5da8ff', '#8e9dff', '#fedc69', '#26deca', '#f68057', '#b955a4'],
      name: $t('page.openPlatform.statistics.statusDistribution'),
      type: 'pie',
      radius: ['45%', '75%'],
      avoidLabelOverlap: false,
      itemStyle: {
        borderRadius: 10,
        borderColor: '#fff',
        borderWidth: 1
      },
      label: {
        show: false,
        position: 'center'
      },
      emphasis: {
        label: {
          show: true,
          fontSize: '12'
        }
      },
      labelLine: {
        show: false
      },
      data: [] as { name: string; value: number }[]
    }
  ]
}));

function getFormattedTimeRange() {
  const [start, end] = dateRange.value;
  return {
    startTime: dayjs(start).format('YYYY-MM-DD'),
    endTime: dayjs(end).format('YYYY-MM-DD')
  };
}

async function loadAppOptions() {
  const { data, error } = await fetchAppList({ current: 1, size: 200 });
  if (!error && data) {
    appOptions.value = (data.records || []).map(app => ({
      label: `${app.name} (${app.id})`,
      value: app.id
    }));
  }
}

async function fetchData() {
  loading.value = true;
  const { startTime, endTime } = getFormattedTimeRange();
  const baseParams = { startTime, endTime, granularity: granularity.value, appId: appId.value || undefined };

  try {
    const [overviewRes, trendRes, topAppsRes, topApisRes, statusDistRes, latencyRes] = await Promise.all([
      fetchGetOpenLogStatistics<Log.OverviewStats>({ ...baseParams, type: 'overview' }),
      fetchGetOpenLogStatistics<Log.TrendItem[]>({ ...baseParams, type: 'trend' }),
      fetchGetOpenLogStatistics<Log.AppStatItem[]>({ ...baseParams, type: 'top_apps' }),
      fetchGetOpenLogStatistics<Log.ApiStatItem[]>({ ...baseParams, type: 'top_apis' }),
      fetchGetOpenLogStatistics<Log.StatusDistItem[]>({ ...baseParams, type: 'status_distribution' }),
      fetchGetOpenLogStatistics<Log.LatencyStats>({ ...baseParams, type: 'latency_stats' })
    ]);

    overview.value = overviewRes.data || null;
    trend.value = trendRes.data || [];
    topApps.value = topAppsRes.data || [];
    topApis.value = topApisRes.data || [];
    statusDist.value = statusDistRes.data || [];
    latencyStats.value = latencyRes.data || null;

    updateCharts();
  } catch (error) {
    console.error('Failed to fetch statistics:', error);
  } finally {
    loading.value = false;
  }
}

function updateCharts() {
  updateTrendChart();
  updateTopAppsChart();
  updateTopApisChart();
  updateStatusDistChart();
}

function updateTrendChart() {
  const dates = trend.value.map(item => item.date);
  const totalCalls = trend.value.map(item => item.totalCalls);
  const successCalls = trend.value.map(item => item.successCalls);

  updateTrendOptions(opts => {
    opts.xAxis.data = dates;
    opts.series[0].data = totalCalls;
    opts.series[1].data = successCalls;
    return opts;
  });
}

function updateTopAppsChart() {
  const names = topApps.value.map(item => item.appName || item.appId);
  const calls = topApps.value.map(item => item.calls);

  updateTopAppsOptions(opts => {
    opts.yAxis.data = names;
    opts.series[0].data = calls;
    return opts;
  });
}

function updateTopApisChart() {
  const paths = topApis.value.map(item => `${item.apiMethod} ${item.apiPath}`);
  const calls = topApis.value.map(item => item.calls);

  updateTopApisOptions(opts => {
    opts.yAxis.data = paths;
    opts.series[0].data = calls;
    return opts;
  });
}

function updateStatusDistChart() {
  const data = statusDist.value.map(item => ({
    name: `${item.statusCode}`,
    value: item.calls
  }));

  updateStatusDistOptions(opts => {
    opts.series[0].data = data;
    return opts;
  });
}

function updateLocale() {
  updateTrendOptions((opts, factory) => {
    const originOpts = factory();
    opts.legend.data = originOpts.legend.data;
    opts.series[0].name = originOpts.series[0].name;
    opts.series[1].name = originOpts.series[1].name;
    return opts;
  });

  updateStatusDistOptions((opts, factory) => {
    const originOpts = factory();
    opts.series[0].name = originOpts.series[0].name;
    return opts;
  });
}

function formatLatency(ms: number): string {
  return `${ms.toFixed(2)}ms`;
}

function formatNumber(num: number): string {
  if (num >= 10000) {
    return `${(num / 10000).toFixed(1)}万`;
  }
  return num.toLocaleString();
}

function handleRefresh() {
  fetchData();
}

watch([dateRange, granularity, appId], () => {
  fetchData();
});

watch(
  () => appStore.locale,
  () => {
    updateLocale();
  }
);

onMounted(() => {
  loadAppOptions();
  fetchData();
});
</script>

<template>
  <div class="min-h-500px flex-col-stretch gap-16px overflow-hidden lt-sm:overflow-auto">
    <NSpace vertical :size="16">
      <!-- 筛选栏 -->
      <NCard :bordered="false" class="card-wrapper" size="small">
        <NSpace align="center" wrap>
          <NDatePicker
            v-model:value="dateRange"
            type="daterange"
            clearable
            :style="{ width: '240px' }"
            :disabled="loading"
          />
          <NSelect
            v-model:value="appId"
            :options="appOptions"
            :placeholder="$t('page.openPlatform.statistics.appFilter')"
            clearable
            filterable
            :style="{ width: '200px' }"
            :disabled="loading"
          />
          <NRadioGroup v-model:value="granularity" :disabled="loading">
            <NRadioButton value="day">{{ $t('page.openPlatform.statistics.day') }}</NRadioButton>
            <NRadioButton value="week">{{ $t('page.openPlatform.statistics.week') }}</NRadioButton>
            <NRadioButton value="month">{{ $t('page.openPlatform.statistics.month') }}</NRadioButton>
          </NRadioGroup>
          <NButton type="primary" :loading="loading" @click="handleRefresh">
            {{ $t('common.refresh') }}
          </NButton>
        </NSpace>
      </NCard>

      <!-- 总览卡片 -->
      <NGrid :x-gap="16" :y-gap="16" cols="s:1 m:2 l:4" responsive="screen">
        <NGi>
          <NCard :bordered="false" class="card-wrapper" size="small">
            <div class="flex items-center gap-12px">
              <div class="flex-center rd-8px bg-[#5da8ff] p-10px text-white">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <path d="M3 13H8V21H3V13Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                  <path d="M10 9H14V21H10V9Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                  <path d="M17 3H21V21H17V3Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </div>
              <div class="flex flex-col">
                <span class="text-12px text-[#999]">{{ $t('page.openPlatform.statistics.totalCalls') }}</span>
                <span class="text-20px font-bold">{{ overview ? formatNumber(overview.totalCalls) : '-' }}</span>
              </div>
            </div>
          </NCard>
        </NGi>
        <NGi>
          <NCard :bordered="false" class="card-wrapper" size="small">
            <div class="flex items-center gap-12px">
              <div class="flex-center rd-8px bg-[#26deca] p-10px text-white">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <path d="M22 11.08V12C21.9988 14.1564 21.3005 16.2547 20.0093 17.9818C18.7182 19.709 16.9033 20.9725 14.8354 21.5839C12.7674 22.1953 10.5573 22.1219 8.53447 21.3746C6.51168 20.6273 4.78465 19.2461 3.61096 17.4371C2.43727 15.628 1.87979 13.4881 2.02168 11.3363C2.16356 9.18455 2.99721 7.13631 4.39828 5.49706C5.79935 3.85781 7.69279 2.71537 9.79619 2.24013C11.8996 1.7649 14.1003 1.98232 16.07 2.85999" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                  <path d="M22 4L12 14.01L9 11.01" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </div>
              <div class="flex flex-col">
                <span class="text-12px text-[#999]">{{ $t('page.openPlatform.statistics.successCalls') }}</span>
                <span class="text-20px font-bold">{{ overview ? formatNumber(overview.successCalls) : '-' }}</span>
              </div>
            </div>
          </NCard>
        </NGi>
        <NGi>
          <NCard :bordered="false" class="card-wrapper" size="small">
            <div class="flex items-center gap-12px">
              <div class="flex-center rd-8px bg-[#f68057] p-10px text-white">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <path d="M12 22C17.5228 22 22 17.5228 22 12C22 6.47715 17.5228 2 12 2C6.47715 2 2 6.47715 2 12C2 17.5228 6.47715 22 12 22Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                  <path d="M15 9L9 15" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                  <path d="M9 9L15 15" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </div>
              <div class="flex flex-col">
                <span class="text-12px text-[#999]">{{ $t('page.openPlatform.statistics.failCalls') }}</span>
                <span class="text-20px font-bold">{{ overview ? formatNumber(overview.failCalls) : '-' }}</span>
              </div>
            </div>
          </NCard>
        </NGi>
        <NGi>
          <NCard :bordered="false" class="card-wrapper" size="small">
            <div class="flex items-center gap-12px">
              <div class="flex-center rd-8px bg-[#fedc69] p-10px text-white">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <path d="M12 22C17.5228 22 22 17.5228 22 12C22 6.47715 17.5228 2 12 2C6.47715 2 2 6.47715 2 12C2 17.5228 6.47715 22 12 22Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                  <path d="M12 6V12L16 14" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </div>
              <div class="flex flex-col">
                <span class="text-12px text-[#999]">{{ $t('page.openPlatform.statistics.avgLatency') }}</span>
                <span class="text-20px font-bold">{{ overview ? formatLatency(overview.avgLatency) : '-' }}</span>
              </div>
            </div>
          </NCard>
        </NGi>
      </NGrid>

      <!-- 调用量趋势图 -->
      <NCard :bordered="false" class="card-wrapper" :title="$t('page.openPlatform.statistics.trend')" size="small">
        <div ref="trendDomRef" class="h-360px overflow-hidden"></div>
      </NCard>

      <!-- 应用调用排行 & API 调用排行 -->
      <NGrid :x-gap="16" :y-gap="16" responsive="screen" item-responsive>
        <NGi span="24 s:24 m:12">
          <NCard :bordered="false" class="card-wrapper" :title="$t('page.openPlatform.statistics.appRanking')" size="small">
            <div ref="topAppsDomRef" class="h-360px overflow-hidden"></div>
          </NCard>
        </NGi>
        <NGi span="24 s:24 m:12">
          <NCard :bordered="false" class="card-wrapper" :title="$t('page.openPlatform.statistics.apiRanking')" size="small">
            <div ref="topApisDomRef" class="h-360px overflow-hidden"></div>
          </NCard>
        </NGi>
      </NGrid>

      <!-- 状态码分布 & 延迟统计 -->
      <NGrid :x-gap="16" :y-gap="16" responsive="screen" item-responsive>
        <NGi span="24 s:24 m:12">
          <NCard :bordered="false" class="card-wrapper" :title="$t('page.openPlatform.statistics.statusDistribution')" size="small">
            <div ref="statusDistDomRef" class="h-360px overflow-hidden"></div>
          </NCard>
        </NGi>
        <NGi span="24 s:24 m:12">
          <NCard :bordered="false" class="card-wrapper" :title="$t('page.openPlatform.statistics.latencyStats')" size="small">
            <NDescriptions :column="1" bordered size="small" class="pt-16px">
              <NDescriptionsItem :label="$t('page.openPlatform.statistics.avgLatency')">
                {{ latencyStats ? formatLatency(latencyStats.avgLatency) : '-' }}
              </NDescriptionsItem>
              <NDescriptionsItem :label="$t('page.openPlatform.statistics.p50')">
                {{ latencyStats ? formatLatency(latencyStats.p50) : '-' }}
              </NDescriptionsItem>
              <NDescriptionsItem :label="$t('page.openPlatform.statistics.p95')">
                {{ latencyStats ? formatLatency(latencyStats.p95) : '-' }}
              </NDescriptionsItem>
              <NDescriptionsItem :label="$t('page.openPlatform.statistics.p99')">
                {{ latencyStats ? formatLatency(latencyStats.p99) : '-' }}
              </NDescriptionsItem>
              <NDescriptionsItem :label="$t('page.openPlatform.statistics.maxLatency')">
                {{ latencyStats ? formatLatency(latencyStats.maxLatency) : '-' }}
              </NDescriptionsItem>
            </NDescriptions>
          </NCard>
        </NGi>
      </NGrid>
    </NSpace>
  </div>
</template>

<style scoped></style>