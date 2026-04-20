import type { RouteLocationNormalizedLoaded, RouteRecordRaw } from 'vue-router';
import { useSvgIcon } from '@/hooks/common/icon';
import { $t } from '@/locales';

/**
 * sort route by order
 */
function sortRouteByOrder(route: RouteRecordRaw) {
  if (route.children?.length) {
    route.children.sort((next, prev) => (Number(next.meta?.order) || 0) - (Number(prev.meta?.order) || 0));
    route.children.forEach(sortRouteByOrder);
  }

  return route;
}

/**
 * sort routes by order
 */
export function sortRoutesByOrder(routes: RouteRecordRaw[]) {
  routes.sort((next, prev) => (Number(next.meta?.order) || 0) - (Number(prev.meta?.order) || 0));
  routes.forEach(sortRouteByOrder);

  return routes;
}

/**
 * Get global menus by auth routes
 */
export function getGlobalMenusByAuthRoutes(routes: RouteRecordRaw[]) {
  const menus: App.Global.Menu[] = [];

  routes.forEach(route => {
    if (!route.meta?.hideInMenu) {
      const menu = getGlobalMenuByBaseRoute(route);

      if (route.children?.some(child => !child.meta?.hideInMenu)) {
        menu.children = getGlobalMenusByAuthRoutes(route.children);
      }

      menus.push(menu);
    }
  });

  return menus;
}

/**
 * Update locale of global menus
 */
export function updateLocaleOfGlobalMenus(menus: App.Global.Menu[]) {
  const result: App.Global.Menu[] = [];

  menus.forEach(menu => {
    const { i18nKey, label, children } = menu;

    let newLabel = label;
    // Fallback logic: if i18nKey translation exists and is not equal to the key itself, use it. Otherwise use original label.
    if (i18nKey) {
      const translation = $t(i18nKey as any);
      if (translation !== i18nKey) {
        newLabel = translation;
      }
    }

    const newMenu: App.Global.Menu = {
      ...menu,
      label: newLabel
    };

    if (children?.length) {
      newMenu.children = updateLocaleOfGlobalMenus(children);
    }

    result.push(newMenu);
  });

  return result;
}

/**
 * Get global menu by route
 */
function getGlobalMenuByBaseRoute(route: RouteLocationNormalizedLoaded | RouteRecordRaw) {
  const { SvgIconVNode } = useSvgIcon();

  const { name, path } = route;
  const { title, i18nKey, icon = import.meta.env.VITE_MENU_ICON, localIcon, iconFontSize } = route.meta ?? {};

  let label = title as string;
  if (i18nKey) {
    const translation = $t(i18nKey as any);
    if (translation !== i18nKey) {
      label = translation;
    }
  }

  const menu: App.Global.Menu = {
    key: name as string,
    label,
    i18nKey: i18nKey as App.I18n.FlexibleI18nKey,
    routeKey: name as string,
    routePath: path as string,
    icon: SvgIconVNode({
      icon: icon as string,
      localIcon: localIcon as string,
      fontSize: (iconFontSize as number) || 20
    })
  };

  return menu;
}

/**
 * Get cache route names
 */
export function getCacheRouteNames(routes: RouteRecordRaw[]) {
  const cacheNames: string[] = [];

  routes.forEach(route => {
    route.children?.forEach(child => {
      if (child.component && child.meta?.keepAlive) {
        cacheNames.push(child.name as string);
      }
    });
  });

  return cacheNames;
}

/**
 * Get selected menu key path
 */
export function getSelectedMenuKeyPathByKey(selectedKey: string, menus: App.Global.Menu[]) {
  const keyPath: string[] = [];

  menus.some(menu => {
    const path = findMenuPath(selectedKey, menu);

    const find = Boolean(path?.length);

    if (find) {
      keyPath.push(...path!);
    }

    return find;
  });

  return keyPath;
}

/**
 * Find menu path
 */
function findMenuPath(targetKey: string, menu: App.Global.Menu): string[] | null {
  const path: string[] = [];

  function dfs(item: App.Global.Menu): boolean {
    path.push(item.key);

    if (item.key === targetKey) {
      return true;
    }

    if (item.children) {
      for (const child of item.children) {
        if (dfs(child)) {
          return true;
        }
      }
    }

    path.pop();

    return false;
  }

  if (dfs(menu)) {
    return path;
  }

  return null;
}

/**
 * Transform menu to breadcrumb
 */
function transformMenuToBreadcrumb(menu: App.Global.Menu) {
  const { children, ...rest } = menu;

  const breadcrumb: App.Global.Breadcrumb = {
    ...rest
  };

  if (children?.length) {
    breadcrumb.options = children.map(transformMenuToBreadcrumb);
  }

  return breadcrumb;
}

/**
 * Get breadcrumbs by route
 */
export function getBreadcrumbsByRoute(
  route: RouteLocationNormalizedLoaded,
  menus: App.Global.Menu[]
): App.Global.Breadcrumb[] {
  const key = route.name as string;
  const activeKey = route.meta?.activeMenu as string;

  for (const menu of menus) {
    if (menu.key === key) {
      return [transformMenuToBreadcrumb(menu)];
    }

    if (menu.key === activeKey) {
      const ROUTE_DEGREE_SPLITTER = '_';

      const parentKey = key.split(ROUTE_DEGREE_SPLITTER).slice(0, -1).join(ROUTE_DEGREE_SPLITTER);

      const breadcrumbMenu = getGlobalMenuByBaseRoute(route);
      if (parentKey !== activeKey) {
        return [transformMenuToBreadcrumb(breadcrumbMenu)];
      }

      return [transformMenuToBreadcrumb(menu), transformMenuToBreadcrumb(breadcrumbMenu)];
    }

    if (menu.children?.length) {
      const result = getBreadcrumbsByRoute(route, menu.children);
      if (result.length > 0) {
        return [transformMenuToBreadcrumb(menu), ...result];
      }
    }
  }

  return [];
}

/**
 * Transform menu to searchMenus
 */
export function transformMenuToSearchMenus(menus: App.Global.Menu[]): App.Global.Menu[] {
  const result: App.Global.Menu[] = [];

  for (const menu of menus) {
    if (menu.children && menu.children.length > 0) {
      result.push(...transformMenuToSearchMenus(menu.children));
    } else {
      result.push(menu);
    }
  }

  return result;
}
