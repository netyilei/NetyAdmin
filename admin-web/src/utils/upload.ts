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
