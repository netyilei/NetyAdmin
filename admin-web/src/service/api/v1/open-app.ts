import type { OpenApp } from '@/typings/api/v1/open-app';
import { request } from '../../request';

/** 获取应用列表 */
export function fetchAppList(params: OpenApp.AppQueryParams) {
  return request<OpenApp.AppPageResult>({
    url: '/admin/v1/open/apps',
    method: 'get',
    params
  });
}

/** 新增应用 */
export function addApp(data: OpenApp.CreateAppReq) {
  return request({
    url: '/admin/v1/open/apps',
    method: 'post',
    data
  });
}

/** 修改应用 */
export function updateApp(data: OpenApp.UpdateAppReq) {
  return request({
    url: '/admin/v1/open/apps',
    method: 'put',
    data
  });
}

/** 删除应用 */
export function deleteApp(id: string) {
  return request({
    url: `/admin/v1/open/apps/${id}`,
    method: 'delete'
  });
}

/** 重置 AppSecret */
export function resetAppSecret(id: string) {
  return request<OpenApp.ResetSecretResult>({
    url: '/admin/v1/open/apps/reset-secret',
    method: 'put',
    data: { id }
  });
}

/** 获取应用的权限范围 */
export function fetchAppScopes(id: string) {
  return request<string[]>({
    url: '/admin/v1/open/apps/scopes',
    method: 'get',
    params: { id }
  });
}

/** 获取所有可用的权限范围 */
export function fetchAvailableScopes() {
  return request<{ label: string; value: string; i18nKey: string }[]>({
    url: '/admin/v1/open/apps/available-scopes',
    method: 'get'
  });
}

/** 获取权限分组列表 */
export function fetchScopeGroupList() {
  return request<OpenApp.ScopeGroup[]>({
    url: '/admin/v1/open/scopes',
    method: 'get'
  });
}

/** 新增权限分组 */
export function addScopeGroup(data: any) {
  return request({
    url: '/admin/v1/open/scopes',
    method: 'post',
    data
  });
}

/** 修改权限分组 */
export function updateScopeGroup(data: any) {
  return request({
    url: '/admin/v1/open/scopes',
    method: 'put',
    data
  });
}

/** 删除权限分组 */
export function deleteScopeGroup(id: number) {
  return request({
    url: `/admin/v1/open/scopes/${id}`,
    method: 'delete'
  });
}
