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

  // 统计相关类型
  type TrendItem = {
    date: string;
    totalCalls: number;
    successCalls: number;
    failCalls: number;
    avgLatency: number;
  };

  type AppStatItem = {
    appId: string;
    appName: string;
    calls: number;
    percent: number;
  };

  type ApiStatItem = {
    apiPath: string;
    apiMethod: string;
    calls: number;
    percent: number;
  };

  type StatusDistItem = {
    statusCode: number;
    calls: number;
    percent: number;
  };

  type LatencyStats = {
    avgLatency: number;
    p50: number;
    p95: number;
    p99: number;
    maxLatency: number;
  };

  type OverviewStats = {
    totalCalls: number;
    successCalls: number;
    failCalls: number;
    avgLatency: number;
    appCount: number;
    apiCount: number;
  };

  type StatisticsParams = {
    type: 'trend' | 'top_apps' | 'top_apis' | 'status_distribution' | 'latency_stats' | 'overview';
    startTime?: string;
    endTime?: string;
    appId?: string;
    granularity?: 'day' | 'week' | 'month';
  };
}
