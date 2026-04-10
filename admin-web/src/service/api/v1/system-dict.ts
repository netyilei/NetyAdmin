import type { SystemDict } from '@/typings/api/v1/system-dict';
import type { Common } from '@/typings/api/v1/common';
import { request } from '../../request';

// ===== 字典类型 =====

/** 获取字典类型列表 */
export function fetchGetDictTypeList(params?: SystemDict.DictTypeSearchParams) {
  return request<Common.PaginatingQueryRecord<SystemDict.DictType>>({
    url: '/admin/v1/system/dict/types',
    method: 'get',
    params
  });
}

/** 创建字典类型 */
export function fetchCreateDictType(data: Pick<SystemDict.DictType, 'name' | 'code' | 'status' | 'description'>) {
  return request<null>({
    url: '/admin/v1/system/dict/types',
    method: 'post',
    data
  });
}

/** 更新字典类型 */
export function fetchUpdateDictType(data: SystemDict.DictType) {
  return request<null>({
    url: '/admin/v1/system/dict/types',
    method: 'put',
    data
  });
}

/** 删除字典类型 */
export function fetchDeleteDictType(id: number) {
  return request<null>({
    url: `/admin/v1/system/dict/types/${id}`,
    method: 'delete'
  });
}

// ===== 字典数据 =====

/** 根据 dictCode 获取启用的字典数据（带缓存） */
export function fetchGetDictData(code: string) {
  return request<SystemDict.DictData[]>({
    url: `/admin/v1/system/dict/data/${code}`,
    method: 'get'
  });
}

/** 获取字典数据管理列表 */
export function fetchGetDictDataList(params?: SystemDict.DictDataSearchParams) {
  return request<Common.PaginatingQueryRecord<SystemDict.DictData>>({
    url: '/admin/v1/system/dict/data',
    method: 'get',
    params
  });
}

/** 创建字典数据 */
export function fetchCreateDictData(
  data: Pick<SystemDict.DictData, 'dictCode' | 'label' | 'value' | 'tagType' | 'orderBy' | 'status' | 'remark'>
) {
  return request<null>({
    url: '/admin/v1/system/dict/data',
    method: 'post',
    data
  });
}

/** 更新字典数据 */
export function fetchUpdateDictData(data: SystemDict.DictData) {
  return request<null>({
    url: '/admin/v1/system/dict/data',
    method: 'put',
    data
  });
}

/** 删除字典数据 */
export function fetchDeleteDictData(id: number) {
  return request<null>({
    url: `/admin/v1/system/dict/data/${id}`,
    method: 'delete'
  });
}
