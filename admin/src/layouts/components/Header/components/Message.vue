<template>
  <div class="message">
    <el-popover placement="bottom" :width="310" trigger="click">
      <template #reference>
        <i :class="'iconfont icon-xiaoxi'" class="toolBar-icon"></i>
      </template>
      <!-- 原为硬编码假 badge(5)+五条假消息——消息中心后端面未交付，
           简化为空态占位（保留入口保持头部布局不变），接入真消息源时再恢复列表 -->
      <div class="message-empty">
        <el-empty :image-size="80" description="暂无消息" />
      </div>
    </el-popover>
  </div>
</template>

<script setup lang="ts"></script>

<style scoped lang="scss">
.message-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
}
</style>
