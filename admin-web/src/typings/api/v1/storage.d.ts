export namespace Storage {
  type StorageProvider = 'aliyun' | 'tencent' | 'huawei' | 'qiniu' | 'minio' | 'aws' | 'cloudflare' | 'custom';

  type StorageConfig = {
    id: number;
    name: string;
    provider: StorageProvider;
    endpoint: string;
    region: string;
    bucket: string;
    accessKey: string;
    domain: string;
    pathPrefix: string;
    isDefault: boolean;
    status: import('@/typings/api/v1/common').Common.EnableStatus;
    maxFileSize: number;
    allowedTypes: string;
    stsExpireTime: number;
    remark: string;
    createdAt: string;
    updatedAt: string;
  };

  type StorageConfigSearchParams = import('@/typings/api/v1/common').Common.CommonSearchParams & {
    name?: string;
    provider?: StorageProvider;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
  };

  type StorageConfigList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<StorageConfig>;

  type CreateStorageConfigParams = {
    name: string;
    provider: StorageProvider;
    endpoint: string;
    region?: string;
    bucket: string;
    accessKey: string;
    secretKey: string;
    domain?: string;
    pathPrefix?: string;
    isDefault?: boolean;
    status?: import('@/typings/api/v1/common').Common.EnableStatus;
    maxFileSize?: number;
    allowedTypes?: string;
    stsExpireTime?: number;
    remark?: string;
  };

  type UpdateStorageConfigParams = CreateStorageConfigParams & { id: number };

  type TestUploadParams = {
    configId: number;
    fileName: string;
    content: string;
  };

  type TestUploadResult = {
    url: string;
    key: string;
  };

  type UploadSource = 'admin' | 'client' | 'api' | 'system';

  type UploadRecord = {
    id: number;
    storageConfigId: number;
    storageName: string;
    fileName: string;
    storedName: string;
    filePath: string;
    fileUrl: string;
    fileSize: number;
    mimeType: string;
    fileExt: string;
    md5: string;
    source: UploadSource;
    sourceId: string;
    sourceInfo: string;
    uploaderIp: string;
    businessType: string;
    businessId: string;
    appId: string;
    uploadedAt: string;
    createdAt: string;
  };

  type UploadRecordSearchParams = import('@/typings/api/v1/common').Common.CommonSearchParams & {
    fileName?: string;
    source?: UploadSource;
    sourceId?: string;
    businessType?: string;
    businessId?: string;
    mimeType?: string;
    storageConfigId?: number;
    appId?: string;
    startTime?: string;
    endTime?: string;
  };

  type UploadRecordList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<UploadRecord>;

  type UploadCredentialsParams = {
    configId?: number;
    fileName: string;
    fileSize?: number;
    contentType?: string;
    businessType?: string;
    businessId?: string;
  };

  type UploadCredentials = {
    url: string;
    method: string;
    headers: Record<string, string>;
    expiresAt: string;
    objectKey: string;
    domain: string;
    finalUrl: string;
    configId: number;
    region: string;
    bucket: string;
    endpoint: string;
    pathPrefix: string;
    maxFileSize: number;
  };

  type CreateUploadRecordParams = {
    configId: number;
    fileName: string;
    objectKey: string;
    fileSize: number;
    mimeType?: string;
    md5?: string;
    businessType?: string;
    businessId?: string;
    sourceInfo?: string;
  };
}
