import { request } from '../../request';
import type { MessageHub } from '@/typings/api/v1/message-hub';

/** 获取消息模板列表 */
export function fetchTemplateList(params: MessageHub.TemplateQueryParams) {
  return request<MessageHub.TemplatePageResult>({
    url: '/admin/v1/message/templates',
    method: 'get',
    params
  });
}

/** 新增消息模板 */
export function addTemplate(data: any) {
  return request({
    url: '/admin/v1/message/templates',
    method: 'post',
    data
  });
}

/** 修改消息模板 */
export function updateTemplate(data: any) {
  return request({
    url: '/admin/v1/message/templates',
    method: 'put',
    data
  });
}

/** 删除消息模板 */
export function deleteTemplate(id: number) {
  return request({
    url: `/admin/v1/message/templates/${id}`,
    method: 'delete'
  });
}

/** 获取消息记录列表 */
export function fetchRecordList(params: MessageHub.RecordQueryParams) {
  return request<MessageHub.RecordPageResult>({
    url: '/admin/v1/message/records',
    method: 'get',
    params
  });
}

/** 直接发送消息 */
export function sendDirect(data: MessageHub.SendDirectReq) {
  return request({
    url: '/admin/v1/message/send',
    method: 'post',
    data
  });
}
