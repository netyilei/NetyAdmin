export namespace SystemDict {
  /** 字典类型 */
  interface DictType {
    id: number;
    name: string;
    code: string;
    status: string;
    description: string;
    createdAt?: string;
    updatedAt?: string;
  }

  /** 字典数据 */
  interface DictData {
    id: number;
    dictCode: string;
    label: string;
    value: string;
    tagType: NaiveUI.ThemeColor;
    orderBy: number;
    status: string;
    remark: string;
    createdAt?: string;
  }

  /** 字典类型搜索参数 */
  type DictTypeSearchParams = import('@/typings/api/v1/common').Common.CommonSearchParams & {
    name?: string | null;
    code?: string | null;
    status?: string | null;
  };

  /** 字典数据搜索参数 */
  type DictDataSearchParams = import('@/typings/api/v1/common').Common.CommonSearchParams & {
    dictCode?: string | null;
    label?: string | null;
    status?: string | null;
  };
}
