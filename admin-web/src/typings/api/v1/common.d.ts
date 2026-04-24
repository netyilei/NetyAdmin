export namespace Common {
  interface PaginatingCommonParams {
    current: number;
    size: number;
    total: number;
  }

  interface PaginatingQueryRecord<T = any> extends PaginatingCommonParams {
    records: T[];
  }

  type CommonSearchParams = Pick<Common.PaginatingCommonParams, 'current' | 'size'>;

  type EnableStatus = '1' | '0';

  type HttpMethod = 'GET' | 'POST' | 'PUT' | 'DELETE';

  type CommonRecord<T = any> = {
    id: number;
    createBy: string;
    createTime: string;
    updateBy: string;
    updateTime: string;
    status: EnableStatus | null;
  } & T;
}
