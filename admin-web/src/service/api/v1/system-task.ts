import type { SystemManage } from '@/typings/api/v1/system-manage';
import { request } from '../../request';

/** get task list */
export function fetchGetTaskList() {
  return request<SystemManage.TaskInfo[]>({
    url: '/admin/v1/system/tasks',
    method: 'get'
  });
}

/** run task manually */
export function fetchRunTask(name: string) {
  return request({
    url: `/admin/v1/system/tasks/${name}/run`,
    method: 'post'
  });
}

/** start task */
export function fetchStartTask(name: string) {
  return request({
    url: `/admin/v1/system/tasks/${name}/start`,
    method: 'post'
  });
}

/** stop task */
export function fetchStopTask(name: string) {
  return request({
    url: `/admin/v1/system/tasks/${name}/stop`,
    method: 'post'
  });
}

/** reload task */
export function fetchReloadTask(name: string) {
  return request({
    url: `/admin/v1/system/tasks/${name}/reload`,
    method: 'post'
  });
}

/** update task config */
export function fetchUpdateTask(name: string, data: { name: string; enabled: boolean; spec: string }) {
  return request({
    url: `/admin/v1/system/tasks/${name}`,
    method: 'put',
    data
  });
}

/** get task logs */
export function fetchGetTaskLogs(params: SystemManage.TaskLogSearchParams) {
  return request<SystemManage.TaskLogList>({
    url: '/admin/v1/system/tasks/logs',
    method: 'get',
    params
  });
}
