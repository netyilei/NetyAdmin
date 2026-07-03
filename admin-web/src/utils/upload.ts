import { fetchCompleteUpload, fetchGetUploadCredentials } from '@/service/api/v1/storage';
import { $t } from '@/locales';

/**
 * S3 兼容存储前端直传工具
 */

export interface UploadCredentials {
  url: string;
  method: string;
  headers: Record<string, string>;
  objectKey: string;
  domain: string;
  finalUrl: string;
  configId: number;
  recordId: number;
  secret: string;
}

export interface UploadProgress {
  loaded: number;
  total: number;
  percent: number;
}

export type OnProgressCallback = (progress: UploadProgress) => void;

/**
 * 使用预签名 URL 直传文件到对象存储
 */
export async function uploadWithPresignedUrl(
  credentials: UploadCredentials,
  file: File,
  onProgress?: OnProgressCallback
): Promise<string> {
  const { url, method, headers } = credentials;

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();

    xhr.upload.addEventListener('progress', event => {
      if (event.lengthComputable && onProgress) {
        onProgress({
          loaded: event.loaded,
          total: event.total,
          percent: Math.round((event.loaded / event.total) * 100)
        });
      }
    });

    xhr.addEventListener('load', () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(credentials.finalUrl);
      } else {
        reject(new Error(`${$t('common.uploadFailed')} (HTTP ${xhr.status})`));
      }
    });

    xhr.addEventListener('error', () => {
      reject(new Error($t('common.networkError')));
    });

    xhr.addEventListener('abort', () => {
      reject(new Error($t('common.uploadCancelled')));
    });

    xhr.open(method, url);

    // Set headers
    const uploadHeaders = { ...headers };
    if (file.type && !Object.keys(uploadHeaders).some(k => k.toLowerCase() === 'content-type')) {
      uploadHeaders['Content-Type'] = file.type;
    }

    Object.entries(uploadHeaders).forEach(([key, value]) => {
      xhr.setRequestHeader(key, value);
    });

    xhr.send(file);
  });
}

/**
 * uploadFileWithCredentials 入参
 *
 * 字段与后端 UploadCredentialsParams 对齐，fileSize/contentType 从传入的 file 自动兜底，
 * businessId 仅在需要归集到某个业务实体时传入（如 article_cover 关联到 articleId）。
 */
export interface UploadFileOptions {
  /** 存储配置 ID，不传则使用默认配置 */
  configId?: number;
  /** 业务类型标识，例如 article_cover / banner_image / user_avatar / storage_test */
  businessType: string;
  /** 业务实体 ID（可选），用于上传记录归集 */
  businessId?: string;
  /** 待上传文件 */
  file: File;
  /** 上传进度回调 */
  onProgress?: OnProgressCallback;
}

/**
 * uploadFileWithCredentials 完整上传三步流程结果
 */
export interface UploadFileResult {
  /** 直传成功后可访问的最终 URL（即 credentials.finalUrl） */
  fileUrl: string;
  /** 凭证签发返回的完整信息，供调用方按需使用（recordId / configId 等） */
  credentials: UploadCredentials;
}

/**
 * 上传文件完整流程：获取凭证 → 预签名直传 → 完成上传通知
 *
 * 封装跨模块复制粘贴的上传三步逻辑，统一错误抛出（由调用方 catch）。
 * 文件名 / 大小 / MIME 全部从 file 自动派生，调用方只需关注 businessType 与赋值目标。
 */
export async function uploadFileWithCredentials(options: UploadFileOptions): Promise<UploadFileResult> {
  const { configId, businessType, businessId, file, onProgress } = options;

  // 第一步：向后端申请预签名上传凭证
  const { data: credentials, error } = await fetchGetUploadCredentials({
    configId,
    fileName: file.name,
    fileSize: file.size,
    contentType: file.type || 'application/octet-stream',
    businessType,
    businessId
  });

  if (error || !credentials) {
    throw new Error(error?.message || $t('common.uploadFailed'));
  }

  // 第二步：使用预签名 URL 直传文件到对象存储
  await uploadWithPresignedUrl(credentials, file, onProgress);

  // 第三步：回传 recordId + secret，由后端验签后置为 uploaded，完成上传闭环
  await fetchCompleteUpload({
    recordId: credentials.recordId,
    secret: credentials.secret,
    objectKey: credentials.objectKey,
    fileUrl: credentials.finalUrl,
    fileSize: file.size,
    mimeType: file.type || 'application/octet-stream'
  });

  return { fileUrl: credentials.finalUrl, credentials };
}
