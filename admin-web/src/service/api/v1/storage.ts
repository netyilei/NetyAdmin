import type { Storage } from '@/typings/api/v1/storage';
import { request } from '../../request';

export function fetchGetStorageConfigList(params?: Storage.StorageConfigSearchParams) {
  return request<Storage.StorageConfigList>({
    url: '/admin/v1/storage-configs',
    method: 'get',
    params
  });
}

export function fetchGetStorageConfig(id: number) {
  return request<Storage.StorageConfig>({
    url: `/admin/v1/storage-configs/${id}`,
    method: 'get'
  });
}

export function fetchCreateStorageConfig(data: Storage.CreateStorageConfigParams) {
  return request({
    url: '/admin/v1/storage-configs',
    method: 'post',
    data
  });
}

export function fetchUpdateStorageConfig(data: Storage.UpdateStorageConfigParams) {
  return request({
    url: '/admin/v1/storage-configs',
    method: 'put',
    data
  });
}

export function fetchDeleteStorageConfig(id: number) {
  return request({
    url: `/admin/v1/storage-configs/${id}`,
    method: 'delete'
  });
}

export function fetchSetDefaultStorageConfig(id: number) {
  return request({
    url: `/admin/v1/storage-configs/${id}/default`,
    method: 'put'
  });
}

export function fetchTestUpload(data: Storage.TestUploadParams) {
  return request<Storage.TestUploadResult>({
    url: '/admin/v1/storage-configs/test-upload',
    method: 'post',
    data
  });
}

export function fetchGetUploadRecordList(params?: Storage.UploadRecordSearchParams) {
  return request<Storage.UploadRecordList>({
    url: '/admin/v1/upload-records',
    method: 'get',
    params
  });
}

export function fetchGetUploadRecord(id: number) {
  return request<Storage.UploadRecord>({
    url: `/admin/v1/upload-records/${id}`,
    method: 'get'
  });
}

export function fetchDeleteUploadRecord(id: number) {
  return request({
    url: `/admin/v1/upload-records/${id}`,
    method: 'delete'
  });
}

export function fetchBatchDeleteUploadRecord(ids: number[]) {
  return request({
    url: '/admin/v1/upload-records/batch-delete',
    method: 'post',
    data: { ids }
  });
}

export function fetchGetUploadCredentials(data: Storage.UploadCredentialsParams) {
  return request<Storage.UploadCredentials>({
    url: '/admin/v1/storage/upload-credentials',
    method: 'post',
    data
  });
}

export function fetchCreateUploadRecord(data: Storage.CreateUploadRecordParams) {
  return request<Storage.UploadRecordVO>({
    url: '/admin/v1/storage/upload-record',
    method: 'post',
    data
  });
}
