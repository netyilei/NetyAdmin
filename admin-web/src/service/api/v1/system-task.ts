import type { SystemManage } from '@/typings/api/v1/system-manage';
import { request } from '../../request';

export function fetchGetTaskList() {
  return request<SystemManage.TaskInfo[]>({
    url: '/admin/v1/system/tasks',
    method: 'get'
  });
}

export function fetchRunTask(name: string) {
  return request({
    url: `/admin/v1/system/tasks/${name}/run`,
    method: 'post'
  });
}

export function fetchStartTask(name: string) {
  return request({
    url: `/admin/v1/system/tasks/${name}/start`,
    method: 'post'
  });
}

export function fetchStopTask(name: string) {
  return request({
    url: `/admin/v1/system/tasks/${name}/stop`,
    method: 'post'
  });
}

export function fetchReloadTask(name: string) {
  return request({
    url: `/admin/v1/system/tasks/${name}/reload`,
    method: 'post'
  });
}

export function fetchUpdateTask(data: { name: string; enabled: boolean; spec: string }) {
  return request({
    url: '/admin/v1/system/tasks/update',
    method: 'put',
    data
  });
}
