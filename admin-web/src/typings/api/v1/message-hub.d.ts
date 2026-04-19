export namespace MessageHub {
  interface Template {
    id: number;
    code: string;
    name: string;
    channel: 'sms' | 'email' | 'internal' | 'push';
    title?: string;
    content: string;
    providerTplId?: string;
    status: 0 | 1;
    createdAt: string;
    updatedAt: string;
  }

  interface TemplateQueryParams {
    current: number;
    size: number;
    channel?: string;
    code?: string;
    name?: string;
    status?: 0 | 1;
    total?: number;
  }

  type TemplatePageResult = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<Template>;

  interface Record {
    id: number;
    userId?: number;
    channel: string;
    receiver: string;
    title?: string;
    content: string;
    status: 0 | 1 | 2; // 0: Pending, 1: Success, 2: Failed
    errorMsg?: string;
    nodeId?: string;
    priority: number;
    retryCount: number;
    createdAt: string;
    updatedAt: string;
  }

  interface RecordQueryParams {
    current: number;
    size: number;
    channel?: string;
    receiver?: string;
    status?: number;
    total?: number;
  }

  type RecordPageResult = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<Record>;

  interface SendDirectReq {
    channel: string;
    receiver: string;
    title?: string;
    content: string;
  }
}
