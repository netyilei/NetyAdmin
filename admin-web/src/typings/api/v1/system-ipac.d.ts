export namespace SystemIPAC {
  interface IPAC {
    id: number;
    appId: number | null;
    ipAddr: string;
    type: 1 | 2; // 1: Allow, 2: Deny
    reason: string;
    expiredAt: string | null;
    status: 0 | 1;
    createdAt: string;
    updatedAt: string;
  }

  interface IPACQueryParams {
    current: number;
    size: number;
    appId?: number;
    ipAddr?: string;
    type?: 1 | 2;
    status?: 0 | 1;
    total?: number;
  }

  type IPACPageResult = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<IPAC>;

  interface CreateIPACReq {
    appId?: number;
    ipAddr: string;
    type: number;
    reason?: string;
    expiredAt?: string;
    status: number;
  }

  interface UpdateIPACReq {
    id: number;
    type: number;
    reason?: string;
    expiredAt?: string;
    status: number;
  }
}
