import type { Router } from 'vue-router';
import { $t } from '@/locales';

/**
 * Get all tabs
 */
export function getAllTabs(tabs: App.Global.Tab[], homeTab?: App.Global.Tab) {
  if (!homeTab) {
    return [];
  }

  const filterHomeTabs = tabs.filter(tab => tab.id !== homeTab.id);
  const fixedTabs = filterHomeTabs.filter(isFixedTab).sort((a, b) => a.fixedIndex! - b.fixedIndex!);
  const remainTabs = filterHomeTabs.filter(tab => !isFixedTab(tab));

  const allTabs = [homeTab, ...fixedTabs, ...remainTabs];

  return updateTabsLabel(allTabs);
}

/**
 * Is fixed tab
 */
function isFixedTab(tab: App.Global.Tab) {
  return tab.fixedIndex !== undefined && tab.fixedIndex !== null;
}

/**
 * Get tab id by route
 */
export function getTabIdByRoute(route: App.Global.TabRoute) {
  const { path, query = {}, meta } = route;

  let id = path;

  if (meta.multiTab) {
    const queryKeys = Object.keys(query).sort();
    const qs = queryKeys.map(key => `${key}=${query[key]}`).join('&');

    id = `${path}?${qs}`;
  }

  return id;
}

/**
 * Get tab by route
 */
export function getTabByRoute(route: App.Global.TabRoute) {
  const { name, path, fullPath = path, meta } = route;

  const { title, i18nKey, fixedIndexInTab } = meta;

  // Get icon and localIcon from getRouteIcons function
  const { icon, localIcon } = getRouteIcons(route);

  const label = i18nKey ? $t(i18nKey as any) : title;

  const tab: App.Global.Tab = {
    id: getTabIdByRoute(route),
    label: (label as string) || (name as string),
    routeKey: name as string,
    routePath: path as string,
    fullPath,
    fixedIndex: fixedIndexInTab as number,
    icon: icon as string,
    localIcon: localIcon as string,
    i18nKey: i18nKey as App.I18n.FlexibleI18nKey
  };

  return tab;
}

/**
 * getRouteIcons
 */
export function getRouteIcons(route: App.Global.TabRoute) {
  let icon: string = (route?.meta?.icon as string) || import.meta.env.VITE_MENU_ICON;
  let localIcon: string | undefined = route?.meta?.localIcon as string;

  if (route.matched) {
    const currentRoute = route.matched.find(r => r.name === route.name);
    icon = (currentRoute?.meta?.icon as string) || icon;
    localIcon = currentRoute?.meta?.localIcon as string;
  }

  return { icon, localIcon };
}

/**
 * Get default home tab
 */
export function getDefaultHomeTab(router: Router, homeRouteName: string) {
  const routes = router.getRoutes();
  const homeRoute = routes.find(route => route.name === homeRouteName);
  const homeRoutePath = homeRoute?.path || '/home';
  const i18nLabel = $t(`route.${homeRouteName}` as any);

  let homeTab: App.Global.Tab = {
    id: homeRoutePath,
    label: (i18nLabel as string) || homeRouteName,
    routeKey: homeRouteName,
    routePath: homeRoutePath,
    fullPath: homeRoutePath
  };

  if (homeRoute) {
    homeTab = getTabByRoute(homeRoute as unknown as App.Global.TabRoute);
  }

  return homeTab;
}

/**
 * Is tab in tabs
 */
export function isTabInTabs(tabId: string, tabs: App.Global.Tab[]) {
  return tabs.some(tab => tab.id === tabId);
}

/**
 * Filter tabs by id
 */
export function filterTabsById(tabId: string, tabs: App.Global.Tab[]) {
  return tabs.filter(tab => tab.id !== tabId);
}

/**
 * Filter tabs by ids
 */
export function filterTabsByIds(tabIds: string[], tabs: App.Global.Tab[]) {
  return tabs.filter(tab => !tabIds.includes(tab.id));
}

/**
 * extract tabs by all routes
 */
export function extractTabsByAllRoutes(router: Router, tabs: App.Global.Tab[]) {
  const routes = router.getRoutes();
  const routeNames = routes.map(route => route.name);
  return tabs.filter(tab => routeNames.includes(tab.routeKey));
}

/**
 * Get fixed tabs
 */
export function getFixedTabs(tabs: App.Global.Tab[]) {
  return tabs.filter(isFixedTab);
}

/**
 * Get fixed tab ids
 */
export function getFixedTabIds(tabs: App.Global.Tab[]) {
  const fixedTabs = getFixedTabs(tabs);
  return fixedTabs.map(tab => tab.id);
}

/**
 * Update tabs label
 */
function updateTabsLabel(tabs: App.Global.Tab[]) {
  const updated = tabs.map(tab => ({
    ...tab,
    label: tab.newLabel || tab.oldLabel || tab.label
  }));

  return updated;
}

/**
 * Update tab by i18n key
 */
export function updateTabByI18nKey(tab: App.Global.Tab) {
  const { i18nKey, label } = tab;

  return {
    ...tab,
    label: i18nKey ? ($t(i18nKey as any) as string) : label
  };
}

/**
 * Update tabs by i18n key
 */
export function updateTabsByI18nKey(tabs: App.Global.Tab[]) {
  return tabs.map(tab => updateTabByI18nKey(tab));
}

/**
 * find tab by route name
 */
export function findTabByRouteName(name: string, tabs: App.Global.Tab[]) {
  const tab = tabs.find(t => t.routeKey === name);
  if (tab) return tab;

  return undefined;
}
