export namespace SystemManage {
  type RoleBase = {
    name: string;
    code: string;
    desc: string;
    homeMenuId?: number;
  };
  type Role = import('@/typings/api/v1/common').Common.CommonRecord<RoleBase>;
  type AddRole = RoleBase & Omit<RoleBase, 'status'>;
  type UpdateRole = RoleBase & { id: number };
  type RoleSearchParams = CommonType.RecordNullable<
    Pick<import('@/typings/api/v1/system-manage').SystemManage.Role, 'name' | 'code' | 'status'> &
      import('@/typings/api/v1/common').Common.CommonSearchParams
  >;

  type RoleList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<Role>;

  type AllRole = Pick<Role, 'id' | 'name' | 'code'>;

  type MenuIdListByRole = {
    homeRouteName: string;
    menuIds: number[];
  };

  type AdminGender = '1' | '2' | null;

  type Admin = import('@/typings/api/v1/common').Common.CommonRecord<{
    userName: string;
    userGender: AdminGender | null;
    nickName: string;
    userPhone: string;
    userEmail: string;
    userRoles: string[];
  }>;
  type EditAdmin = {
    id?: number;
    username: string;
    password?: string;
    nickname: string;
    phone: string;
    email: string;
    gender: AdminGender | null;
    roles: string[];
    status: import('@/typings/api/v1/common').Common.EnableStatus | null;
  };

  type AdminSearchParams = CommonType.RecordNullable<
    Pick<
      import('@/typings/api/v1/system-manage').SystemManage.EditAdmin,
      'username' | 'gender' | 'nickname' | 'phone' | 'email' | 'status'
    > &
      import('@/typings/api/v1/common').Common.CommonSearchParams
  >;

  type AdminList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<Admin>;

  type MenuType = '1' | '2' | '3';

  type MenuButton = {
    code: string;
    desc: string;
  };

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
    parentId: number;
    type: MenuType;
    name: string;
    routeName: string;
    routePath: string;
    component?: string;
    icon: string;
    iconType: IconType;
    buttons?: MenuButton[] | null;
    children?: Menu[] | null;
  }> &
    MenuPropsOfRoute;

  type MenuSearchParams = CommonType.RecordNullable<
    Pick<import('@/typings/api/v1/system-manage').SystemManage.Menu, 'name' | 'status'> &
      import('@/typings/api/v1/common').Common.CommonSearchParams
  >;

  type MenuList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<Menu>;

  type MenuTree = {
    id: number;
    label: string;
    pId: number;
    children?: MenuTree[];
  };

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

  interface SysConfig {
    groupName: string;
    configKey: string;
    configValue: string;
    valueType: string;
    description: string;
    isSystem: boolean;
  }

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
