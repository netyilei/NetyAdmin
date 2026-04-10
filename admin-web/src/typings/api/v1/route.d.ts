/**
 * namespace Route
 *
 * backend api module: "route"
 */
export namespace Route {
  type MenuRoute = import('vue-router').RouteRecordRaw & {
    id: string;
  };

  interface UserRoute {
    routes: MenuRoute[];
    home: string;
  }
}
