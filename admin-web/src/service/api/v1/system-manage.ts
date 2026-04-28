import type { ClientUser } from '@/typings/api/v1/client-user';
import type { SystemManage } from '@/typings/api/v1/system-manage';
import { request } from '../../request';

/** get role list */
export function fetchGetRoleList(params?: SystemManage.RoleSearchParams) {
  return request<SystemManage.RoleList>({
    url: '/admin/v1/systemManage/getRoleList',
    method: 'get',
    params
  });
}

/**
 * get all roles
 *
 * these roles are all enabled
 */
export function fetchGetAllRoles() {
  return request<SystemManage.AllRole[]>({
    url: '/admin/v1/systemManage/getAllRoles',
    method: 'get'
  });
}

/** get all api */
export function fetchGetAllApi() {
  return request<SystemManage.ApiItem[]>({
    url: '/admin/v1/systemManage/getAllApi',
    method: 'get',
    params: {
      auth: '1'
    }
  });
}

/** get all api by role id */
export function fetchGetApiIdsByRole(roleId: number) {
  return request<number[]>({
    url: `/admin/v1/systemManage/role/${roleId}/apis`,
    method: 'get',
    params: {
      auth: '1'
    }
  });
}

/** update api id list by role id */
export function fetchUpdateApiIdsByRole(roleId: number, apiIds: number[]) {
  return request({
    url: `/admin/v1/systemManage/role/${roleId}/apis`,
    method: 'put',
    data: apiIds
  });
}

/** add role */
export function fetchAddRole(data: SystemManage.AddRole) {
  return request({
    url: '/admin/v1/systemManage/addRole',
    method: 'post',
    data
  });
}

/** update role */
export function fetchUpdateRole(data: SystemManage.UpdateRole) {
  return request({
    url: '/admin/v1/systemManage/updateRole',
    method: 'put',
    data
  });
}

/** delete role */
export function fetchDeleteRole(id: number) {
  return request({
    url: '/admin/v1/systemManage/deleteRole',
    method: 'delete',
    params: { roleId: id }
  });
}

/** delete roles */
export function fetchBatchDeleteRole(ids: number[]) {
  return request({
    url: '/admin/v1/systemManage/deleteRoles',
    method: 'delete',
    params: { roleIds: ids }
  });
}

/** get admin list */
export function fetchGetAdminList(params?: SystemManage.AdminSearchParams) {
  return request<SystemManage.AdminList>({
    url: '/admin/v1/admins',
    method: 'get',
    params
  });
}

/** add admin */
export function fetchAddAdmin(data: SystemManage.EditAdmin) {
  return request({
    url: '/admin/v1/admins',
    method: 'post',
    data
  });
}

/** update admin */
export function fetchUpdateAdmin(id: number, data: Omit<SystemManage.EditAdmin, 'id'>) {
  return request({
    url: `/admin/v1/admins/${id}`,
    method: 'put',
    data
  });
}

/** delete admin */
export function fetchDeleteAdmin(id: number) {
  return request({
    url: `/admin/v1/admins/${id}`,
    method: 'delete'
  });
}

/** delete admins */
export function fetchBatchDeleteAdmin(ids: number[]) {
  return request({
    url: '/admin/v1/admins/batch',
    method: 'delete',
    data: { ids }
  });
}

/** get menu list */
export function fetchGetMenuList(params?: SystemManage.MenuSearchParams) {
  return request<SystemManage.MenuList>({
    url: '/admin/v1/systemManage/getMenuList',
    method: 'get',
    params
  });
}

/** get all pages */
export function fetchGetAllPages() {
  return request<string[]>({
    url: '/admin/v1/systemManage/getAllPages',
    method: 'get'
  });
}

/** get all button */
export function fetchGetAllButton() {
  return request<SystemManage.ButtonConfig[]>({
    url: '/admin/v1/systemManage/getAllButton',
    method: 'get'
  });
}

/** get button id list by role id */
export function fetchGetButtonIdsByRole(roleId: number) {
  return request<number[]>({
    url: `/admin/v1/systemManage/role/${roleId}/buttons`,
    method: 'get'
  });
}

/** update button id list by role id */
export function fetchUpdateButtonIdsByRole(roleId: number, buttonIds: number[]) {
  return request({
    url: `/admin/v1/systemManage/role/${roleId}/buttons`,
    method: 'put',
    data: buttonIds
  });
}

/** get menu tree */
export function fetchGetMenuTree() {
  return request<SystemManage.MenuTree[]>({
    url: '/admin/v1/systemManage/getMenuTree',
    method: 'get'
  });
}

/** get button tree */
export function fetchGetButtonTree() {
  return request<SystemManage.MenuButtonTree[]>({
    url: '/admin/v1/systemManage/getButtonTree',
    method: 'get'
  });
}

/** get api tree */
export function fetchGetApiTree() {
  return request<SystemManage.MenuApiTree[]>({
    url: '/admin/v1/systemManage/getApiTree',
    method: 'get'
  });
}

/** GetMenuIdsByRole */
export function fetchGetMenuIdsByRole(roleId: number) {
  return request<SystemManage.MenuIdListByRole>({
    url: `/admin/v1/systemManage/role/${roleId}/menus`,
    method: 'get'
  });
}

/** fetchUpdateMenuIdsByRole */
export function fetchUpdateMenuIdsByRole(roleId: number, data: SystemManage.MenuIdListByRole) {
  return request({
    url: `/admin/v1/systemManage/role/${roleId}/menus`,
    method: 'put',
    data
  });
}

/** add menu */
export function fetchAddMenu(data: SystemManage.Menu) {
  return request<{ id: number }>({
    url: '/admin/v1/systemManage/addMenu',
    method: 'post',
    data
  });
}

/** get menu by id */
export function fetchGetMenu(id: number) {
  return request<SystemManage.Menu>({
    url: `/admin/v1/systemManage/getMenu/${id}`,
    method: 'get'
  });
}

/** update menu */
export function fetchUpdateMenu(data: SystemManage.Menu) {
  return request({
    url: '/admin/v1/systemManage/updateMenu',
    method: 'put',
    data
  });
}

/** delete menu */
export function fetchDeleteMenu(id: number) {
  return request({
    url: '/admin/v1/systemManage/deleteMenu',
    method: 'delete',
    params: { menuId: id }
  });
}

/** delete menus */
export function fetchBatchDeleteMenu(ids: number[]) {
  return request({
    url: '/admin/v1/systemManage/deleteMenus',
    method: 'delete',
    params: { menuIds: ids }
  });
}

/** 获取终端用户列表 */
export function fetchGetUserList(params?: ClientUser.ClientUserSearchParams) {
  return request<ClientUser.ClientUserList>({
    url: '/admin/v1/systemManage/users',
    method: 'get',
    params
  });
}

/** 用户自动补全（用于消息中心收件人等场景） */
export function fetchUserAutocomplete(keyword: string) {
  return request<ClientUser.UserInfo[]>({
    url: '/admin/v1/systemManage/users/autocomplete',
    method: 'get',
    params: { keyword }
  });
}

/** 新增终端用户 */
export function fetchAddUser(data: any) {
  return request({
    url: '/admin/v1/systemManage/users',
    method: 'post',
    data
  });
}

/** 更新终端用户 */
export function fetchUpdateUser(id: string, data: any) {
  return request({
    url: `/admin/v1/systemManage/users/${id}`,
    method: 'put',
    data
  });
}

/** 更新终端用户状态 */
export function fetchUpdateUserStatus(id: string, status: string) {
  return request({
    url: `/admin/v1/systemManage/users/${id}/status`,
    method: 'patch',
    data: { status }
  });
}

/** 删除终端用户 */
export function fetchDeleteUser(id: string) {
  return request({
    url: `/admin/v1/systemManage/users/${id}`,
    method: 'delete'
  });
}

/** 解锁终端用户 */
export function fetchUnlockUser(id: string) {
  return request({
    url: `/admin/v1/systemManage/users/${id}/unlock`,
    method: 'post'
  });
}

/** 获取系统配置（缓存开关等） */
export function fetchGetSysConfigs(groupName?: string) {
  return request<SystemManage.SysConfig[]>({
    url: '/admin/v1/system/configs',
    method: 'get',
    params: { groupName }
  });
}

/** 修改系统配置（修改后自动触发 Redis 广播热重载） */
export function fetchUpdateSysConfig(data: SystemManage.SysConfig) {
  return request({
    url: '/admin/v1/system/configs',
    method: 'put',
    data
  });
}

/** 测试邮件发送 */
export function fetchTestEmail(data: { receiver: string }) {
  return request({
    url: '/admin/v1/system/test-email',
    method: 'post',
    data
  });
}
