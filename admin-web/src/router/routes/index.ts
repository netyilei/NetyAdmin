import type { RouteRecordRaw } from 'vue-router';

/**
 * Constant routes
 * These routes are built-in and do not require permission verification.
 */
export const constantRoutes: RouteRecordRaw[] = [
  {
    name: 'login',
    path: '/login/:module(pwd-login|code-login|register|reset-pwd|bind-wechat)?',
    component: () => import('@/views/_builtin/login/index.vue'),
    meta: {
      title: 'login',
      i18nKey: 'route.login',
      constant: true,
      hideInMenu: true
    }
  },
  {
    name: '403',
    path: '/403',
    component: () => import('@/views/_builtin/403/index.vue'),
    meta: {
      title: '403',
      i18nKey: 'route.403',
      constant: true,
      hideInMenu: true
    }
  },
  {
    name: '404',
    path: '/404',
    component: () => import('@/views/_builtin/404/index.vue'),
    meta: {
      title: '404',
      i18nKey: 'route.404',
      constant: true,
      hideInMenu: true
    }
  },
  {
    name: '500',
    path: '/500',
    component: () => import('@/views/_builtin/500/index.vue'),
    meta: {
      title: '500',
      i18nKey: 'route.500',
      constant: true,
      hideInMenu: true
    }
  },
  {
    name: 'iframe-page',
    path: '/iframe-page/:url',
    component: () => import('@/views/_builtin/iframe-page/[url].vue'),
    meta: {
      title: 'iframe-page',
      i18nKey: 'route.iframe-page',
      constant: true,
      hideInMenu: true
    }
  }
];

/** create routes when the auth route mode is static */
export function createStaticRoutes() {
  // We completely decoupled from generated static routes.
  // We now strictly serve only the necessary constant routes for pre-login.
  // Actual auth routes are supplied entirely by the backend payload.
  return {
    constantRoutes,
    authRoutes: [] as RouteRecordRaw[]
  };
}

/**
 * Get auth vue routes
 */
export function getAuthVueRoutes(routes: RouteRecordRaw[]) {
  return routes;
}
