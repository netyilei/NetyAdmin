import { request } from '../../request';
import type { SystemIPAC } from '@/typings/api/v1/system-ipac';

/** 获取 IP 规则列表 */
export function fetchIPACList(params: SystemIPAC.IPACQueryParams) {
  return request<SystemIPAC.IPACPageResult>({
    url: '/admin/v1/ops/ip-access',
    method: 'get',
    params
  });
}

/** 新增 IP 规则 */
export function addIPAC(data: SystemIPAC.CreateIPACReq) {
  return request({
    url: '/admin/v1/ops/ip-access',
    method: 'post',
    data
  });
}

/** 修改 IP 规则 */
export function updateIPAC(data: SystemIPAC.UpdateIPACReq) {
  return request({
    url: '/admin/v1/ops/ip-access',
    method: 'put',
    data
  });
}

/** 删除单个 IP 规则 */
export function deleteIPAC(id: number) {
  return request({
    url: `/admin/v1/ops/ip-access/${id}`,
    method: 'delete'
  });
}

/** 批量删除 IP 规则 */
export function batchDeleteIPAC(ids: number[]) {
  return request({
    url: '/admin/v1/ops/ip-access/batch',
    method: 'delete',
    data: { ids }
  });
}
