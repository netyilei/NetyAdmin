export namespace OpenApp {
  interface App {
    id: string;
    appKey: string;
    name: string;
    status: 0 | 1;
    ipFilterEnabled: boolean;
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
    status?: 0 | 1;
    total?: number;
  }

  type AppPageResult = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<App>;

  interface CreateAppReq {
    name: string;
    status: number;
    ipFilterEnabled: boolean;
    remark?: string;
    scopes?: string[];
  }

  interface UpdateAppReq {
    id: string;
    name: string;
    status: number;
    ipFilterEnabled: boolean;
    remark?: string;
    scopes?: string[];
  }

  interface ResetSecretReq {
    id: string;
  }

  interface ResetSecretResult {
    appSecret: string;
  }

  interface LinkIPRulesReq {
    id: string;
    ruleIds: number[];
  }

  interface ScopeGroup {
    id: number;
    code: string;
    name: string;
    description: string;
    status: 0 | 1;
    createdAt: string;
  }
}
