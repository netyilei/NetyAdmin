export namespace OpenApi {
  interface Api {
    id: number;
    method: string;
    path: string;
    name: string;
    group: string;
    description: string;
    status: 0 | 1;
    createdAt: string;
    updatedAt: string;
  }

  interface ApiQueryParams {
    current: number;
    size: number;
    method?: string;
    path?: string;
    name?: string;
    group?: string;
    status?: 0 | 1;
  }

  type ApiPageResult = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<Api>;

  interface CreateApiReq {
    method: string;
    path: string;
    name: string;
    group?: string;
    description?: string;
    status: 0 | 1;
  }

  interface UpdateApiReq {
    id: number;
    method: string;
    path: string;
    name: string;
    group?: string;
    description?: string;
    status: 0 | 1;
  }

  interface UpdateScopeApisReq {
    scopeId: number;
    apiIds: number[];
  }

  interface GroupedApi {
    group: string;
    apis: ApiItem[];
  }

  interface ApiItem {
    id: number;
    name: string;
    method: string;
    path: string;
  }
}
