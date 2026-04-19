import type {
  LocationQueryRaw,
  NavigationGuardNext,
  RouteLocationNormalized,
  RouteLocationRaw,
  Router
} from 'vue-router';
import { useRouteStore } from '@/store/modules/route';
import { localStg } from '@/utils/storage';

/**
 * create route guard
 *
 * @param router router instance
 */
export function createRouteGuard(router: Router) {
  router.beforeEach(async (to, from, next) => {
    const location = await initRoute(to);

    if (location) {
      next(location);
      return;
    }

    const rootRoute = 'root';
    const loginRoute = 'login';

    const isLogin = Boolean(localStg.get('token'));
    const needLogin = !to.meta.constant;

    // if it is login route when logged in, then switch to the root page
    if (to.name === loginRoute && isLogin) {
      next({ name: rootRoute });
      return;
    }

    // if the route does not need login, then it is allowed to access directly
    if (!needLogin) {
      handleRouteSwitch(to, from, next);
      return;
    }

    // the route need login but the user is not logged in, then switch to the login page
    if (!isLogin) {
      next({ name: loginRoute, query: { redirect: to.fullPath } });
      return;
    }

    // switch route normally - roles are checked in backend
    handleRouteSwitch(to, from, next);
  });
}

/**
 * initialize route
 *
 * @param to to route
 */
async function initRoute(to: RouteLocationNormalized): Promise<RouteLocationRaw | null> {
  const routeStore = useRouteStore();

  const notFoundRoute = 'not-found';
  const isNotFoundRoute = to.name === notFoundRoute;

  // if the constant route is not initialized, then initialize the constant route
  if (!routeStore.isInitConstantRoute) {
    await routeStore.initConstantRoute();

    const path = to.fullPath;
    const location: RouteLocationRaw = {
      path,
      replace: true,
      query: to.query,
      hash: to.hash
    };

    return location;
  }

  const isLogin = Boolean(localStg.get('token'));

  if (!isLogin) {
    if (to.meta.constant && !isNotFoundRoute) {
      routeStore.onRouteSwitchWhenNotLoggedIn();
      return null;
    }

    const loginRoute = 'login';
    const query = getRouteQueryOfLoginRoute(to, routeStore.routeHome);

    const location: RouteLocationRaw = {
      name: loginRoute,
      query
    };

    return location;
  }

  if (!routeStore.isInitAuthRoute) {
    await routeStore.initAuthRoute();

    if (isNotFoundRoute) {
      const rootRoute = 'root';
      const path = to.redirectedFrom?.name === rootRoute ? '/' : to.fullPath;

      const location: RouteLocationRaw = {
        path,
        replace: true,
        query: to.query,
        hash: to.hash
      };

      return location;
    }
  }

  routeStore.onRouteSwitchWhenLoggedIn();

  if (!isNotFoundRoute) {
    return null;
  }

  const exist = await routeStore.getIsAuthRouteExist(to.path);
  const noPermissionRoute = '403';

  if (exist) {
    const location: RouteLocationRaw = {
      name: noPermissionRoute
    };

    return location;
  }

  return null;
}

function handleRouteSwitch(to: RouteLocationNormalized, from: RouteLocationNormalized, next: NavigationGuardNext) {
  if (to.meta.href) {
    window.open(to.meta.href, '_blank');
    next({ path: from.fullPath, replace: true, query: from.query, hash: to.hash });
    return;
  }
  next();
}

function getRouteQueryOfLoginRoute(to: RouteLocationNormalized, routeHome: string) {
  const loginRoute = 'login';
  const redirect = to.fullPath;
  const [redirectPath, redirectQuery] = redirect.split('?');

  // Clean basic implementation since we dropped getRouteName
  const redirectName = redirectPath.split('/').filter(Boolean).join('_');

  const isRedirectHome = routeHome === redirectName;

  const query: LocationQueryRaw = to.name !== loginRoute && !isRedirectHome ? { redirect } : {};

  if (isRedirectHome && redirectQuery) {
    query.redirect = `/?${redirectQuery}`;
  }

  return query;
}
