export namespace Log {
  type CommonSearchParams = Pick<import('@/typings/api/v1/common').Common.PaginatingCommonParams, 'current' | 'size'>;

  type OperationLog = {
    id: number;
    userId: number;
    username: string;
    action: string;
    resource: string;
    detail: string;
    ip: string;
    userAgent: string;
    createdAt: string;
  };

  type OperationLogSearchParams = CommonSearchParams & {
    userId?: number;
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
    userId: number;
    ip: string;
    userAgent: string;
    resolved: boolean;
    resolvedBy: number;
    occurCount: number;
    lastOccurredAt: string;
    createdAt: string;
  };

  type ErrorLogSearchParams = CommonSearchParams & {
    level?: ErrorLogLevel;
    resolved?: boolean;
    startTime?: string;
    endTime?: string;
  };

  type ErrorLogList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<ErrorLog>;
}
