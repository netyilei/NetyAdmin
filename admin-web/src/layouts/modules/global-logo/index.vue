<script setup lang="ts">
import { useRouteStore } from '@/store/modules/route';
import { $t } from '@/locales';

defineOptions({
  name: 'GlobalLogo'
});

interface Props {
  /** Whether to show the title */
  showTitle?: boolean;
}

withDefaults(defineProps<Props>(), {
  showTitle: true
});

const routeStore = useRouteStore();
</script>

<template>
  <RouterLink
    :to="routeStore.routeHome.startsWith('/') ? routeStore.routeHome : { name: routeStore.routeHome }"
    class="w-full flex-center nowrap-hidden"
  >
    <SystemLogo class="text-32px text-primary" />
    <h2 v-show="showTitle" class="pl-8px text-16px text-primary font-bold transition duration-300 ease-in-out">
      {{ $t('system.title') }}
    </h2>
  </RouterLink>
</template>

<style scoped></style>
