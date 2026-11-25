// 消息管理 API（V2 message——端点契约 contracts/openapi/admin.v1.yaml
// admin.message.*；字段名=后端 json tag（snake_case，对照 spec schema 精确声明）。
// 信封由 axios 拦截器统一解包（调用方拿信封对象取 .data）；分页 Page{list,total,page,page_size}。
import http from "@/api";

// 模板通道枚举（domain.Channel）
export type MessageChannel = "email" | "sms";

/** 消息模板行（msgdomain.Template）。 */
export interface MessageTemplateRow {
  id: string;
  name: string;
  channel: MessageChannel;
  subject: string; // sms 通道为空
  body: string; // {{key}} 占位符
  status: number; // 1=enabled 2=disabled
  created_at: string;
  updated_at: string;
}

/** 模板写请求（adapters.templateRequest——status 必填枚举 1|2，update 缺省落 0=静默禁用防呆）。 */
export interface MessageTemplatePayload {
  name: string;
  channel: MessageChannel;
  subject?: string;
  body: string;
  status: number;
}

/** 发送记录行（msgdomain.Record）。 */
export interface MessageRecordRow {
  id: string;
  template_id: string;
  channel: MessageChannel;
  target: string;
  subject: string;
  body: string;
  driver: string;
  status: number; // 1=pending 2=sent 3=failed
  retries: number;
  error: string;
  created_at: string;
  sent_at: string | null;
}

/** 站内信发送请求（adapters.inboxSendRequest——三项必填）。 */
export interface InboxSendPayload {
  user_id: string;
  title: string;
  body: string;
}

export const messageApi = {
  /** 模板列表（name=名称包含过滤；分页） */
  listTemplates: (page: number, pageSize: number, query?: { name?: string }) =>
    http.page<MessageTemplateRow>("/admin/v1/message/templates", page, pageSize, query),
  /** 创建模板（响应=模板行） */
  createTemplate: (data: MessageTemplatePayload) => http.post<MessageTemplateRow>("/admin/v1/message/templates", data),
  /** 更新模板（status 必填——防静默禁用） */
  updateTemplate: (id: string, data: MessageTemplatePayload) =>
    http.put<MessageTemplateRow>(`/admin/v1/message/templates/${id}`, data),
  /** 删除模板 */
  deleteTemplate: (id: string) => http.delete(`/admin/v1/message/templates/${id}`),
  /** 按模板发送消息（响应=发送记录行） */
  send: (data: { template_id: string; target: string; vars?: Record<string, string> }) =>
    http.post<MessageRecordRow>("/admin/v1/message/send", data),
  /** 发送记录列表（channel/status 过滤；status: 1=pending 2=sent 3=failed） */
  listRecords: (page: number, pageSize: number, query?: { channel?: string; status?: string }) =>
    http.page<MessageRecordRow>("/admin/v1/message/records", page, pageSize, query),
  /** 重发失败/待发记录（响应=新记录行） */
  resend: (id: string) => http.post<MessageRecordRow>(`/admin/v1/message/records/${id}/resend`),
  /** 发送站内信 */
  sendInbox: (data: InboxSendPayload) => http.post("/admin/v1/message/inbox", data)
};
