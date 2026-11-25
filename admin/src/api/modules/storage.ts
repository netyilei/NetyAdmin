// 存储配置 API（V2 storage——端点契约 contracts/openapi/admin.v1.yaml
// admin.storage.*；字段名=后端 json tag（snake_case，对照 spec schema 精确声明）。
// 信封由 axios 拦截器统一解包（调用方拿信封对象取 .data）；分页 Page{list,total,page,page_size}。
import http from "@/api";

// 寻址风格枚举（domain.StorageBucketLookup——空=默认 path）
export type BucketLookup = "" | "path" | "virtual-host";
// 提供商枚举（domain.StorageProvider）
export type StorageProvider = "minio" | "s3";

/** 存储配置行（domain.StorageConfig——响应 secret_key 掩码 ****xxxx，不回显原文）。 */
export interface StorageConfigRow {
  id: string;
  name: string;
  provider: StorageProvider;
  endpoint: string;
  region: string;
  bucket_lookup: BucketLookup;
  bucket: string;
  access_key: string;
  secret_key: string;
  domain: string;
  path_prefix: string;
  is_default: boolean;
  status: number; // 1=enabled 2=disabled
  max_file_size: number; // 字节；0=不限
  allowed_types: string; // 逗号分隔 MIME；空=不限
  created_at: string;
  updated_at: string;
}

/** 配置写请求（adapters.configRequest——required: name/provider/endpoint/bucket/access_key/secret_key）。 */
export interface StorageConfigPayload {
  name: string;
  provider: StorageProvider;
  endpoint: string;
  region?: string;
  bucket_lookup?: BucketLookup;
  bucket: string;
  access_key: string;
  secret_key: string;
  domain?: string;
  path_prefix?: string;
  is_default?: boolean;
  status?: number;
  max_file_size?: number;
  allowed_types?: string;
}

/** 上传记录行（domain.UploadRecord——HMAC secret 不出域，响应面无此字段）。 */
export interface StorageRecordRow {
  id: string;
  config_id: string;
  object_key: string;
  file_name: string;
  file_size: number;
  mime_type: string;
  business_type: string;
  business_id: string;
  status: number; // 1=pending 2=completed 3=expired
  file_url: string;
  uploader_id: string;
  uploader_ip: string;
  created_at: string;
  completed_at: string | null;
}

export const storageApi = {
  /** 配置列表（分页；契约无过滤参数） */
  listConfigs: (page: number, pageSize: number) => http.page<StorageConfigRow>("/admin/v1/storage/configs", page, pageSize),
  /** 创建配置（响应=掩码后配置行） */
  createConfig: (data: StorageConfigPayload) => http.post<StorageConfigRow>("/admin/v1/storage/configs", data),
  /** 更新配置（secret_key 必填重填——响应掩码不可回传） */
  updateConfig: (id: string, data: StorageConfigPayload) => http.put<StorageConfigRow>(`/admin/v1/storage/configs/${id}`, data),
  /** 删除配置 */
  deleteConfig: (id: string) => http.delete(`/admin/v1/storage/configs/${id}`),
  /** 设默认配置 */
  setDefault: (id: string) => http.put(`/admin/v1/storage/configs/${id}/default`),
  /** 上传记录列表（分页） */
  listRecords: (page: number, pageSize: number) => http.page<StorageRecordRow>("/admin/v1/storage/records", page, pageSize),
  /** 手动清理过期上传记录（响应 data.expired=清理条数） */
  cleanup: () => http.post<{ expired: number }>("/admin/v1/storage/cleanup")
};
