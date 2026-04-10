import type { Route } from '@/typings/api/v1/route';
import { request } from '../../request';

/** get user routes */
export function fetchGetUserRoutes() {
  return request<Route.UserRoute>({ url: '/admin/v1/route/getUserRoutes' });
}

/**
 * whether the route is exist
 *
 * @param routeName route name
 */
export function fetchIsRouteExist(routeName: string) {
  return request<boolean>({ url: '/admin/v1/route/isRouteExist', params: { routeName } });
}
