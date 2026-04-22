import type { Log } from '@/typings/api/v1/log';
import { request } from '../../request';

export function fetchGetOperationLogList(params?: Log.OperationLogSearchParams) {
  return request<Log.OperationLogList>({
    url: '/admin/v1/operation-logs',
    method: 'get',
    params
  });
}

export function fetchDeleteOperationLog(id: number) {
  return request({
    url: `/admin/v1/operation-logs/${id}`,
    method: 'delete'
  });
}

export function fetchBatchDeleteOperationLog(ids: number[]) {
  return request({
    url: '/admin/v1/operation-logs/batch-delete',
    method: 'post',
    data: { ids }
  });
}

export function fetchGetErrorLogList(params?: Log.ErrorLogSearchParams) {
  return request<Log.ErrorLogList>({
    url: '/admin/v1/error-logs',
    method: 'get',
    params
  });
}

export function fetchResolveErrorLog(id: number) {
  return request({
    url: `/admin/v1/error-logs/${id}/resolve`,
    method: 'put'
  });
}

export function fetchDeleteErrorLog(id: number) {
  return request({
    url: `/admin/v1/error-logs/${id}`,
    method: 'delete'
  });
}

export function fetchBatchDeleteErrorLog(ids: number[]) {
  return request({
    url: '/admin/v1/error-logs/batch-delete',
    method: 'post',
    data: { ids }
  });
}

export function fetchGetOpenLogList(params?: Log.OpenLogSearchParams) {
  return request<Log.OpenLogList>({
    url: '/admin/v1/ops/open-platform-log',
    method: 'get',
    params
  });
}

export function fetchGetTaskLogs(params: Log.TaskLogSearchParams) {
  return request<Log.TaskLogList>({
    url: '/admin/v1/system/tasks/logs',
    method: 'get',
    params
  });
}
