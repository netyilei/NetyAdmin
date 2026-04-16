export namespace OpenApp {
  interface App {
    id: string;
    appKey: string;
    name: string;
    type: 1 | 2; // 1: Internal, 2: External
    status: 0 | 1; // 0: Disabled, 1: Enabled
    ipStrategy: 1 | 2; // 1: Blacklist, 2: Whitelist
    quotaConfig: string;
    remark: string;
    createdAt: string;
    updatedAt: string;
    scopes?: string[];
  }

  interface AppQueryParams {
    current: number;
    size: number;
    name?: string;
    appKey?: string;
    type?: 1 | 2;
    status?: 0 | 1;
    total?: number;
  }

  type AppPageResult = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<App>;

  interface CreateAppReq {
    name: string;
    type: number;
    status: number;
    ipStrategy: number;
    remark?: string;
    scopes?: string[];
  }

  interface UpdateAppReq {
    id: string;
    name: string;
    type: number;
    status: number;
    ipStrategy: number;
    remark?: string;
    scopes?: string[];
  }

  interface ResetSecretReq {
    id: string;
  }

  interface ResetSecretResult {
    appSecret: string;
  }

  interface ScopeGroup {
    id: number;
    code: string;
    name: string;
    i18nKey: string;
    description: string;
    status: 0 | 1;
    createdAt: string;
  }
}
