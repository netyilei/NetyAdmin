// 操作日志 API（——/admin/v1/operation-logs 查询面；security 子面
// =type=3 过滤；无删除面——logging.md §3 追加写不可篡改）。
// 字段对照 internal/modules/audit/domain/record.go（camelCase json tag）。
import http from "@/api";

/** RecordType：1=operation 2=access 3=security 4=system。 */
export const LOG_TYPE_OPERATION = 1;
export const LOG_TYPE_ACCESS = 2;
export const LOG_TYPE_SECURITY = 3;
export const LOG_TYPE_SYSTEM = 4;

export const LOG_TYPE_LABELS: Record<number, string> = {
  [LOG_TYPE_OPERATION]: "操作",
  [LOG_TYPE_ACCESS]: "访问",
  [LOG_TYPE_SECURITY]: "安全",
  [LOG_TYPE_SYSTEM]: "系统"
};

export interface AuditRecord {
  id: string;
  type: number;
  actorType?: string;
  actorId?: string;
  action?: string;
  resource?: string;
  result?: string;
  bizCode?: string;
  ip?: string;
  userAgent?: string;
  requestId?: string;
  traceId?: string;
  nodeId?: string;
  detail?: Record<string, any>;
  createdAt?: string;
}

/** 列表过滤（request_id 仅列表支持——导出端点无该参数）。 */
export interface AuditLogQuery {
  type?: number;
  action?: string;
  actor_id?: string;
  result?: string;
  request_id?: string;
  time_from?: string;
  time_to?: string;
}

export const auditApi = {
  logs: (page = 1, pageSize = 10, query?: AuditLogQuery) =>
    http.page<AuditRecord>("/admin/v1/operation-logs", page, pageSize, query as Record<string, string | undefined>),
  get: (id: string) => http.get<AuditRecord>(`/admin/v1/operation-logs/${id}`),
  /** CSV 导出（GET + responseType blob——拦截器对无 code 的 Blob 原样放行）。 */
  export: (query?: AuditLogQuery) =>
    http.get("/admin/v1/operation-logs/export", query as Record<string, string | undefined>, { responseType: "blob" })
};
