<template>
  <div class="home card">
    <!-- 仪表盘统计卡片（有 admin.gw.stats.view 才显示；无权限保持纯欢迎图） -->
    <el-row v-if="canViewStats" v-loading="statsLoading" :gutter="12" class="stats-row">
      <el-col :span="6">
        <el-card shadow="never"><el-statistic title="调用总量" :value="stats.total" /></el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="never"><el-statistic title="错误数" :value="stats.errors" /></el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="never"><el-statistic title="错误率 (%)" :value="errorRatePercent" /></el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="never"><el-statistic title="平均延迟 (ms)" :value="stats.avgLatency" /></el-card>
      </el-col>
    </el-row>
    <div class="home-body">
      <img class="home-bg" src="@/assets/images/welcome.png" alt="welcome" />
    </div>
  </div>
</template>

<script setup lang="ts" name="home">
// 首页仪表盘：欢迎图 + 网关调用概览四指标（overview，24h 窗——
// 与应用平台页同源 /admin/v1/open/stats/overview）。
// 权限面=admin.gw.stats.view（与后端 stats 端点 RequirePerm 同源；权限集在
// 动态路由初始化时已就绪——首页进入前完成）；无权限=保持纯欢迎图。
// 数据最简形态：进页一次拉取，不做轮询（刷新/重新进页即更新）。
import { computed, onMounted, reactive, ref } from "vue";

import { gwApi } from "@/api/modules/gwAdmin";
import { useAuthStore } from "@/stores/modules/auth";

const PERM_STATS_VIEW = "admin.gw.stats.view";

const authStore = useAuthStore();
const canViewStats = computed(() => authStore.authButtonListGet.includes(PERM_STATS_VIEW));

const statsLoading = ref(false);
const stats = reactive({ total: 0, errors: 0, errorRate: 0, avgLatency: 0 });
const errorRatePercent = computed(() => Number((stats.errorRate * 100).toFixed(2)));

const loadStats = async () => {
  statsLoading.value = true;
  try {
    const resp: any = await gwApi.statsOverview(24);
    const d = resp.data ?? {};
    stats.total = d.total ?? 0;
    stats.errors = d.errors ?? 0;
    stats.errorRate = d.errorRate ?? 0;
    stats.avgLatency = d.avgLatency ?? 0;
  } finally {
    statsLoading.value = false;
  }
};

onMounted(() => {
  if (canViewStats.value) loadStats();
});
</script>

<style scoped lang="scss">
@use "./index";
</style>
