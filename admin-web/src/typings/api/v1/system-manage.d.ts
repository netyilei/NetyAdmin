/**
 * namespace SystemManage
 *
 * backend api module: "systemManage"
 */
export namespace SystemManage {
  type CommonSearchParams = Pick<import('@/typings/api/v1/common').Common.PaginatingCommonParams, 'current' | 'size'>;

  /** role base */
  type RoleBase = {
    /** role name */
    name: string;
    /** role code */
    code: string;
    /** role description */
    desc: string;
    /** role homeMenuId */
    homeMenuId?: number;
  };
  /** role */
  type Role = import('@/typings/api/v1/common').Common.CommonRecord<RoleBase>;
  /** role add params */
  type AddRole = RoleBase & Omit<RoleBase, 'status'>;
  /** role update params */
  type UpdateRole = RoleBase & { id: number };
  /** role search params */
  type RoleSearchParams = CommonType.RecordNullable<
    Pick<import('@/typings/api/v1/system-manage').SystemManage.Role, 'name' | 'code' | 'status'> & CommonSearchParams
  >;

  /** role list */
  type RoleList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<Role>;

  /** all role */
  type AllRole = Pick<Role, 'id' | 'name' | 'code'>;

  /** menu id list by role */
  type MenuIdListByRole = {
    /** role id */
    homeRouteName: string;
    /** menu id list */
    menuIds: number[];
  };

  /** admin gender
   *
   * - "1": "male"
   * - "2": "female"
   */
  type AdminGender = '1' | '2' | null;

  /** @deprecated Use AdminGender instead - backward compatibility */
  type UserGender = AdminGender;

  /** admin */
  type Admin = import('@/typings/api/v1/common').Common.CommonRecord<{
    /** admin user name */
    userName: string;
    /** admin gender */
    userGender: AdminGender | null;
    /** admin nick name */
    nickName: string;
    /** admin phone */
    userPhone: string;
    /** admin email */
    userEmail: string;
    /** admin role code collection */
    userRoles: string[];
  }>;
  /** add or update admin */
  type EditAdmin = {
    id?: number;
    username: string;
    nickname: string;
    phone: string;
    email: string;
    gender: AdminGender | null;
    roles: string[];
    status: import('@/typings/api/v1/common').Common.EnableStatus | null;
  };

  /** admin search params */
  type AdminSearchParams = CommonType.RecordNullable<
    Pick<
      import('@/typings/api/v1/system-manage').SystemManage.Admin,
      'userName' | 'userGender' | 'nickName' | 'userPhone' | 'userEmail' | 'status'
    > &
      CommonSearchParams
  >;

  /** admin list */
  type AdminList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<Admin>;

  /** @deprecated Use Admin instead - backward compatibility */
  type User = Admin;
  /** @deprecated Use EditAdmin instead - backward compatibility */
  type EditUser = EditAdmin;
  /** @deprecated Use AdminSearchParams instead - backward compatibility */
  type UserSearchParams = AdminSearchParams;
  /** @deprecated Use AdminList instead - backward compatibility */
  type UserList = AdminList;

  /**
   * menu type
   *
   * - "1": directory
   * - "2": menu
   * - "3": button
   */
  type MenuType = '1' | '2' | '3';

  type MenuButton = {
    /**
     * button code
     *
     * it can be used to control the button permission
     */
    code: string;
    /** button description */
    desc: string;
  };

  /**
   * icon type
   *
   * - "1": iconify icon
   * - "2": local icon
   */
  type IconType = '1' | '2';

  type MenuPropsOfRoute = Pick<
    import('vue-router').RouteMeta,
    | 'i18nKey'
    | 'keepAlive'
    | 'constant'
    | 'order'
    | 'href'
    | 'hideInMenu'
    | 'activeMenu'
    | 'multiTab'
    | 'fixedIndexInTab'
    | 'query'
  >;

  type Menu = import('@/typings/api/v1/common').Common.CommonRecord<{
    /** parent menu id */
    parentId: number;
    /** menu type */
    type: MenuType;
    /** menu name */
    name: string;
    /** route name */
    routeName: string;
    /** route path */
    routePath: string;
    /** component */
    component?: string;
    /** iconify icon name or local icon name */
    icon: string;
    /** icon type */
    iconType: IconType;
    /** buttons */
    buttons?: MenuButton[] | null;
    /** children menu */
    children?: Menu[] | null;
  }> &
    MenuPropsOfRoute;

  /** menu search params */
  type MenuSearchParams = CommonType.RecordNullable<
    Pick<import('@/typings/api/v1/system-manage').SystemManage.Menu, 'name' | 'status'> & CommonSearchParams
  >;

  /** menu list */
  type MenuList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<Menu>;

  type MenuTree = {
    id: number;
    label: string;
    pId: number;
    children?: MenuTree[];
  };

  /** button config */
  type ButtonConfig = {
    id: number;
    label: string;
    code: string;
  };

  type ApiItem = {
    id: number;
    createdAt: number;
    updatedAt: number;
    menuId: number;
    name: string;
    desc: string;
    method: import('@/typings/api/v1/common').Common.HttpMethod;
    path: string;
    auth: string;
  };

  type MenuButtonTree = {
    id: string;
    label: string;
    type: 'menu' | 'button';
    children?: MenuButtonTree[];
  };

  type MenuApiTree = {
    id: string;
    label: string;
    type: 'menu' | 'api';
    method?: import('@/typings/api/v1/common').Common.HttpMethod;
    path?: string;
    children?: MenuApiTree[];
  };

  /** System Configuration */
  interface SysConfig {
    groupName: string;
    configKey: string;
    configValue: string;
    valueType: string;
    description: string;
    isSystem: boolean;
  }

  /** Task Meta */
  interface TaskInfo {
    name: string;
    displayName: string;
    type: 'cron' | 'interval' | 'once';
    spec: string;
    weight: number;
    enabled: boolean;
    isRunning: boolean;
    lastRunTime: string;
    lastDuration: number;
    lastStatus: 'success' | 'error' | '';
    lastMessage: string;
    executionCount: number;
  }
}
