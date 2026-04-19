import type { OpenApi } from '@/typings/api/v1/open-api';
import { request } from '../../request';

export function fetchApiList(params: OpenApi.ApiQueryParams) {
  return request<OpenApi.ApiPageResult>({
    url: '/admin/v1/open/apis',
    method: 'get',
    params
  });
}

export function addApi(data: OpenApi.CreateApiReq) {
  return request({
    url: '/admin/v1/open/apis',
    method: 'post',
    data
  });
}

export function updateApi(data: OpenApi.UpdateApiReq) {
  return request({
    url: '/admin/v1/open/apis',
    method: 'put',
    data
  });
}

export function deleteApi(id: number) {
  return request({
    url: `/admin/v1/open/apis/${id}`,
    method: 'delete'
  });
}

export function fetchAllApis() {
  return request<OpenApi.Api[]>({
    url: '/admin/v1/open/apis/all',
    method: 'get'
  });
}

export function fetchScopeApis(scopeId: number) {
  return request<OpenApi.Api[]>({
    url: '/admin/v1/open/apis/scope-apis',
    method: 'get',
    params: { scopeId }
  });
}

export function updateScopeApis(data: OpenApi.UpdateScopeApisReq) {
  return request({
    url: '/admin/v1/open/apis/scope-apis',
    method: 'put',
    data
  });
}
