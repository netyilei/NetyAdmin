import type { RouteRecordRaw } from 'vue-router';

export const ROOT_ROUTE: RouteRecordRaw = {
  name: 'root',
  path: '/',
  redirect: import.meta.env.VITE_ROUTE_HOME || '/home',
  meta: {
    title: 'root',
    constant: true
  }
};

const NOT_FOUND_ROUTE: RouteRecordRaw = {
  name: 'not-found',
  path: '/:pathMatch(.*)*',
  component: () => import('@/views/_builtin/404/index.vue'),
  meta: {
    title: 'not-found',
    constant: true
  }
};

/** create builtin vue routes */
export function createBuiltinVueRoutes() {
  return [ROOT_ROUTE, NOT_FOUND_ROUTE];
}
