import type { AxiosResponse } from 'axios';
import { BACKEND_ERROR_CODE, createFlatRequest } from '@na/axios';
import { useAuthStore } from '@/store/modules/auth';
import { getServiceBaseURL } from '@/utils/service';
import { getAuthorization, handleExpiredRequest, showErrorMsg } from './shared';
import type { RequestInstanceState } from './type';
import { BackendErrorCode, getBackendErrorMessage } from './backend-error';

const isHttpProxy = import.meta.env.DEV && import.meta.env.VITE_HTTP_PROXY === 'Y';
const { baseURL } = getServiceBaseURL(import.meta.env, isHttpProxy);

/**
 * 业务码分组：直接引用 BackendErrorCode 常量，避免 .env 占位符与实际错误码漂移。
 *
 * - logoutCodes：身份已失效，必须重新登录（含 token 版本号失效、账户禁用）
 * - expiredTokenCodes：token 过期/无效，但身份仍合法，尝试 refresh token 自动续期
 *
 * 注意：refreshToken 接口本身不可返回 expiredTokenCodes，否则会死循环；
 * 它必须返回 logoutCodes（如 unauthorized）让前端直接登出。
 */
const LOGOUT_CODES: readonly string[] = [
  BackendErrorCode.unauthorized, // 100002 — token 版本号失效、ParseToken 失败
  BackendErrorCode.userDisabled // 101002 — 账户被禁用
];

const EXPIRED_TOKEN_CODES: readonly string[] = [
  BackendErrorCode.tokenExpired, // 101005
  BackendErrorCode.tokenInvalid // 101006
];

export const request = createFlatRequest<App.Service.Response, RequestInstanceState>(
  {
    baseURL
  },
  {
    async onRequest(config) {
      const Authorization = getAuthorization();
      Object.assign(config.headers, { Authorization });

      return config;
    },
    isBackendSuccess(response) {
      // when the backend response code equals `VITE_SERVICE_SUCCESS_CODE`, it means the request is success
      return String(response.data.code) === import.meta.env.VITE_SERVICE_SUCCESS_CODE;
    },
    async onBackendFail(response, instance) {
      const authStore = useAuthStore();
      const responseCode = String(response.data.code);

      function handleLogout() {
        authStore.resetStore();
      }

      // 身份已失效：直接登出并跳转登录页（版本号失效、账户禁用）
      if (LOGOUT_CODES.includes(responseCode)) {
        handleLogout();
        return null;
      }

      // token 过期/无效：尝试 refresh token 续期并重试原请求
      if (EXPIRED_TOKEN_CODES.includes(responseCode)) {
        const success = await handleExpiredRequest(request.state);
        if (success) {
          const Authorization = getAuthorization();
          Object.assign(response.config.headers, { Authorization });

          return instance.request(response.config) as Promise<AxiosResponse>;
        }
      }

      return null;
    },
    transformBackendResponse(response) {
      return response.data.data;
    },
    onError(error) {
      let message = error.message;
      let backendErrorCode = '';

      // 非 2xx HTTP 响应（如网关 502/504）— 提取后端错误码（如有）
      if (error.code === BACKEND_ERROR_CODE) {
        backendErrorCode = String(error.response?.data?.code || '');
        message = getBackendErrorMessage(backendErrorCode) || message;
      }

      // 身份失效场景已由 onBackendFail 处理登出，onError 不再重复弹窗
      if (LOGOUT_CODES.includes(backendErrorCode)) {
        return;
      }

      // token 过期场景已由 onBackendFail 触发 refresh，onError 不再重复弹窗
      if (EXPIRED_TOKEN_CODES.includes(backendErrorCode)) {
        return;
      }

      showErrorMsg(request.state, message);
    }
  }
);
