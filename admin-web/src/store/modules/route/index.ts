import { computed, nextTick, ref, shallowRef } from 'vue';
import type { RouteRecordRaw } from 'vue-router';
import { defineStore } from 'pinia';
import { useBoolean } from '@na/hooks';
import { router } from '@/router';
import { fetchGetUserRoutes, fetchIsRouteExist } from '@/service/api/v1/route';
import { SetupStoreId } from '@/enum';
import { createStaticRoutes, getAuthVueRoutes } from '@/router/routes';
import { ROOT_ROUTE } from '@/router/routes/builtin';
import { useAuthStore } from '../auth';
import { useTabStore } from '../tab';
import {
  getBreadcrumbsByRoute,
  getCacheRouteNames,
  getGlobalMenusByAuthRoutes,
  getSelectedMenuKeyPathByKey,
  sortRoutesByOrder,
  transformMenuToSearchMenus,
  updateLocaleOfGlobalMenus
} from './shared';

const views = import.meta.glob('@/views/**/*.vue');

/**
 * Map component string to actual Vue component
 * Supports:
 * - layout.base, layout.blank, layout -> Layout components
 * - view.xxx -> Page views
 * - xxx -> Direct views
 */
function mapComponent(componentStr?: string | unknown) {
  if (!componentStr || typeof componentStr !== 'string') return undefined;

  const normalized = componentStr.trim().toLowerCase();

  // Layout Mapping
  if (normalized === 'layout' || normalized === 'layout.base') {
    return () => import('@/layouts/base-layout/index.vue');
  }
  if (normalized === 'layout.blank') {
    return () => import('@/layouts/blank-layout/index.vue');
  }

  // Handle standard 'view.{name}' syntax
  let viewName = componentStr;
  if (viewName.startsWith('view.')) {
    viewName = viewName.replace('view.', '');
  }

  // Try directly replacing _ with / to search the file system
  const pathPart = viewName.replace(/_/g, '/');

  const possiblePaths = [
    `/src/views/${pathPart}/index.vue`,
    `/src/views/${pathPart}.vue`,
    `/src/views/${viewName}/index.vue`,
    `/src/views/${viewName}.vue`
  ];

  for (const path of possiblePaths) {
    if (views[path]) {
      return views[path];
    }
  }

  // If it's still missing, it might be a special layout keyword like 'layout' handled above
  // but if we are here, it means we didn't find a view file.
  return undefined;
}

export const useRouteStore = defineStore(SetupStoreId.Route, () => {
  const authStore = useAuthStore();
  const tabStore = useTabStore();
  const { bool: isInitConstantRoute, setBool: setIsInitConstantRoute } = useBoolean();
  const { bool: isInitAuthRoute, setBool: setIsInitAuthRoute } = useBoolean();

  const routeHome = ref<string>(import.meta.env.VITE_ROUTE_HOME || '/home');

  function setRouteHome(routeKey: string) {
    routeHome.value = routeKey;
  }

  const constantRoutes = shallowRef<RouteRecordRaw[]>([]);
  const authRoutes = shallowRef<RouteRecordRaw[]>([]);
  const removeRouteFns: (() => void)[] = [];
  const menus = ref<App.Global.Menu[]>([]);
  const searchMenus = computed(() => transformMenuToSearchMenus(menus.value));
  const cacheRoutes = ref<string[]>([]);
  const excludeCacheRoutes = ref<string[]>([]);

  function addConstantRoutes(routes: RouteRecordRaw[]) {
    constantRoutes.value = routes;
  }

  function addAuthRoutes(routes: RouteRecordRaw[]) {
    authRoutes.value = routes;
  }

  function getGlobalMenus(routes: RouteRecordRaw[]) {
    menus.value = getGlobalMenusByAuthRoutes(routes as any);
  }

  function updateGlobalMenusByLocale() {
    menus.value = updateLocaleOfGlobalMenus(menus.value);
  }

  function getCacheRoutes(routes: RouteRecordRaw[]) {
    cacheRoutes.value = getCacheRouteNames(routes as any);
  }

  async function resetRouteCache(routeKey?: string) {
    const routeName = routeKey || (router.currentRoute.value.name as string);
    excludeCacheRoutes.value.push(routeName);
    await nextTick();
    excludeCacheRoutes.value = [];
  }

  const breadcrumbs = computed(() => getBreadcrumbsByRoute(router.currentRoute.value, menus.value));

  async function resetStore() {
    const routeStore = useRouteStore();
    routeStore.$reset();
    resetVueRoutes();
    await initConstantRoute();
  }

  function resetVueRoutes() {
    removeRouteFns.forEach(fn => fn());
    removeRouteFns.length = 0;
  }

  async function initConstantRoute() {
    if (isInitConstantRoute.value) return;

    const staticRoute = createStaticRoutes();
    addConstantRoutes(staticRoute.constantRoutes);
    handleConstantAndAuthRoutes();

    setIsInitConstantRoute(true);
    tabStore.initHomeTab();
  }

  async function initAuthRoute() {
    if (!authStore.userInfo.userId) {
      await authStore.initUserInfo();
    }
    await initDynamicAuthRoute();
    tabStore.initHomeTab();
  }

  /**
   * Transform dynamic routes to Vue routes
   * Handles:
   * - Composite components (L$V pattern)
   * - Automatic layout wrapping for top-level pages
   * - Default layouts for directories
   */
  function transformDynamicRoutes(routes: any[], isRoot = true) {
    return routes.map(item => {
      const dynamicRoute: any = { ...item };
      const componentStr = item.component || '';

      // 1. Handle Composite Pattern: layout.base$view.home
      if (componentStr.includes('$')) {
        const [layoutPart, viewPart] = componentStr.split('$');
        dynamicRoute.component = mapComponent(layoutPart) || mapComponent('layout.base');

        // Hide children in menu for single-page layout wrappers
        if (!dynamicRoute.meta) dynamicRoute.meta = {};
        dynamicRoute.meta.hideChildrenInMenu = true;

        // Ensure navigation to the parent redirects to the child
        dynamicRoute.redirect = { name: `${item.name}_index` };

        // 4. Create child route for the actual view
        dynamicRoute.children = [
          {
            name: `${item.name}_index`,
            path: '',
            component: mapComponent(viewPart) || (() => import('@/views/_builtin/404/index.vue')),
            meta: {
              ...item.meta,
              hideInMenu: true // Hide the child to prevent sidebar nesting
            }
          }
        ];
        return dynamicRoute;
      }

      // 2. Handle Directory or Missing Component
      if (!componentStr || componentStr === 'layout' || componentStr === 'layout.base') {
        dynamicRoute.component = isRoot ? mapComponent('layout.base') : undefined;
      } else if (componentStr === 'layout.blank') {
        dynamicRoute.component = mapComponent('layout.blank');
      } else {
        // Simple View Mode
        const mappedView = mapComponent(componentStr);

        // If it's a root view (e.g. Home), it needs a layout wrapper in NetyAdmin
        if (isRoot) {
          dynamicRoute.component = mapComponent('layout.base');

          if (!dynamicRoute.meta) dynamicRoute.meta = {};
          dynamicRoute.meta.hideChildrenInMenu = true;

          // Ensure navigation to the parent redirects to the child
          dynamicRoute.redirect = { name: `${item.name}_index` };

          dynamicRoute.children = [
            {
              name: `${item.name}_index`,
              path: '',
              component: mappedView || (() => import('@/views/_builtin/404/index.vue')),
              meta: {
                ...item.meta,
                hideInMenu: true
              }
            }
          ];
          return dynamicRoute;
        }

        dynamicRoute.component = mappedView || (() => import('@/views/_builtin/404/index.vue'));
      }

      // Recursively transform children
      if (item.children && item.children.length > 0) {
        dynamicRoute.children = transformDynamicRoutes(item.children, false);
      }

      return dynamicRoute;
    });
  }

  async function initDynamicAuthRoute() {
    const { data, error } = await fetchGetUserRoutes();

    if (!error && data) {
      const { routes, home } = data;

      const mappedRoutes = transformDynamicRoutes(routes);

      addAuthRoutes(mappedRoutes);
      handleConstantAndAuthRoutes();
      setRouteHome(home);
      handleUpdateRootRouteRedirect(home);
      setIsInitAuthRoute(true);
    } else {
      authStore.resetStore();
    }
  }

  function handleConstantAndAuthRoutes() {
    const allRoutes = [...constantRoutes.value, ...authRoutes.value];
    const sortRoutes = sortRoutesByOrder(allRoutes as any);
    const vueRoutes = getAuthVueRoutes(sortRoutes as any);

    resetVueRoutes();
    addRoutesToVueRouter(vueRoutes);
    getGlobalMenus(sortRoutes as any);
    getCacheRoutes(vueRoutes);
  }

  function addRoutesToVueRouter(routes: RouteRecordRaw[]) {
    routes.forEach(route => {
      const removeFn = router.addRoute(route);
      addRemoveRouteFns(removeFn);
    });
  }

  function addRemoveRouteFns(fn: () => void) {
    removeRouteFns.push(fn);
  }

  function handleUpdateRootRouteRedirect(redirectPath: string) {
    if (redirectPath) {
      const rootRoute = { ...ROOT_ROUTE, redirect: redirectPath } as any as RouteRecordRaw;
      router.removeRoute(rootRoute.name as string);
      router.addRoute(rootRoute);
    }
  }

  async function getIsAuthRouteExist(routePath: string) {
    const routeName = routePath.split('/').filter(Boolean).join('_');
    if (!routeName) return false;
    const { data } = await fetchIsRouteExist(routeName);
    return data;
  }

  function getSelectedMenuKeyPath(selectedKey: string) {
    return getSelectedMenuKeyPathByKey(selectedKey, menus.value);
  }

  async function onRouteSwitchWhenLoggedIn() {
    await authStore.initUserInfo();
  }

  async function onRouteSwitchWhenNotLoggedIn() {}

  return {
    resetStore,
    routeHome,
    menus,
    searchMenus,
    updateGlobalMenusByLocale,
    cacheRoutes,
    excludeCacheRoutes,
    resetRouteCache,
    breadcrumbs,
    initConstantRoute,
    isInitConstantRoute,
    initAuthRoute,
    isInitAuthRoute,
    setIsInitAuthRoute,
    getIsAuthRouteExist,
    getSelectedMenuKeyPath,
    onRouteSwitchWhenLoggedIn,
    onRouteSwitchWhenNotLoggedIn
  };
});
