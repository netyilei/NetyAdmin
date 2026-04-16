import { request } from '../../request';

/** 获取开放平台调用日志列表 */
export function fetchOpenLogList(params: any) {
  return request<any>({
    url: '/admin/v1/ops/open-platform-log',
    method: 'get',
    params
  });
}
