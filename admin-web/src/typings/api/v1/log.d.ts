export namespace Log {
  type OperationLog = {
    id: number;
    adminId: number;
    username: string;
    action: string;
    resource: string;
    detail: string;
    ip: string;
    userAgent: string;
    createdAt: string;
  };

  type OperationLogSearchParams = import('@/typings/api/v1/common').Common.CommonSearchParams & {
    adminId?: number;
    action?: string;
    startDate?: string;
    endDate?: string;
  };

  type OperationLogList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<OperationLog>;

  type ErrorLogLevel = 'error' | 'panic' | 'warn';

  type ErrorLog = {
    id: number;
    level: ErrorLogLevel;
    message: string;
    stack: string;
    requestId: string;
    path: string;
    method: string;
    adminId: number;
    ip: string;
    userAgent: string;
    resolved: boolean;
    resolvedBy: number;
    occurCount: number;
    lastOccurredAt: string;
    createdAt: string;
  };

  type ErrorLogSearchParams = import('@/typings/api/v1/common').Common.CommonSearchParams & {
    level?: ErrorLogLevel;
    resolved?: boolean;
    startTime?: string;
    endTime?: string;
  };

  type ErrorLogList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<ErrorLog>;

  type OpenLog = {
    id: number;
    appId: string;
    appKey: string;
    apiPath: string;
    apiMethod: string;
    statusCode: number;
    latency: number;
    clientIp: string;
    requestHeader: string;
    requestBody: string;
    responseBody: string;
    errorMsg: string;
    createdAt: string;
  };

  type OpenLogSearchParams = import('@/typings/api/v1/common').Common.CommonSearchParams & {
    appId?: string;
    appKey?: string;
    apiPath?: string;
    statusCode?: number;
    startTime?: string;
    endTime?: string;
  };

  type OpenLogList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<OpenLog>;

  type TaskLog = {
    id: number;
    name: string;
    startTime: string;
    endTime: string;
    duration: number;
    status: 'success' | 'error';
    message: string;
  };

  type TaskLogSearchParams = {
    name: string;
    page: number;
    size: number;
  };

  type TaskLogList = {
    list: TaskLog[];
    total: number;
  };
}
