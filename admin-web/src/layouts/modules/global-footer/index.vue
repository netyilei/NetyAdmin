<script setup lang="ts">
import { APP_CONFIG } from '@/config';

defineOptions({
  name: 'GlobalFooter'
});
</script>

<template>
  <DarkModeContainer class="h-full flex-center">
    <a href="https://github.com/netyilei/NetyAdmin/blob/master/LICENSE" target="_blank" rel="noopener noreferrer">
      Copyright MIT © 2026 {{ APP_CONFIG.name }}
    </a>
  </DarkModeContainer>
</template>

<style scoped></style>
