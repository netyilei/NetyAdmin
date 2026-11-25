import axios, { AxiosError, AxiosInstance, AxiosRequestConfig, AxiosResponse, InternalAxiosRequestConfig } from "axios";
import { ElMessage } from "element-plus";

import { ResultData, Page } from "@/api/interface";
import { showFullScreenLoading, tryHideFullScreenLoading } from "@/components/Loading/fullScreen";
import { messageKeyText } from "@/api/messageKeys";
import { LOGIN_URL } from "@/config";
import { StatusCode } from "@/constants/status-codes";
import { ResultEnum } from "@/enums/httpEnum";
import router from "@/routers";
import { useUserStore } from "@/stores/modules/user";

import { AxiosCanceler } from "./helper/axiosCancel";
import { checkStatus } from "./helper/checkStatus";

export interface CustomAxiosRequestConfig extends InternalAxiosRequestConfig {
  _isRetry?: boolean;
  loading?: boolean;
  cancel?: boolean;
}

const config = {
  // 默认地址请求地址，可在 .env.** 文件中修改（prod/test 空串=同源挂载）
  baseURL: import.meta.env.VITE_API_URL as string,
  // 设置超时时间
  timeout: ResultEnum.TIMEOUT as number,
  // 跨域凭证：同源挂载下无需携带（拆分部署启用 CORS 时再开——：
  // 此前恒 true 曾向 mock 域携带凭证）
  withCredentials: false
};

const axiosCanceler = new AxiosCanceler();

/** 并发 401 登出单飞（2：N 个请求同时 401 只登出+提示一次）。 */
let logoutInFlight = false;
function forceLogout(message: string) {
  if (logoutInFlight) return;
  logoutInFlight = true;
  const userStore = useUserStore();
  userStore.setToken("");
  userStore.setRefreshToken("");
  router.replace(LOGIN_URL);
  ElMessage.error(message);
  setTimeout(() => (logoutInFlight = false), 500);
}

/** refresh_token 换新 access（单飞防并发风暴；读写经 user store persist——
 * 单一存储路径。绕拦截器原始 axios：refresh 自身 401 不递归）。 */
// refresh 走原生 axios 不经实例拦截器——必须自带 baseURL（拆分部署
// VITE_API_URL 非空时相对路径会打到 SPA 域 404）与超时（悬挂会让 401 排队
// 永不释放）
const REFRESH_TIMEOUT_MS = 10_000;
let refreshInFlight: Promise<string> | null = null;
async function refreshAccessToken(): Promise<string> {
  if (!refreshInFlight) {
    refreshInFlight = (async () => {
      const userStore = useUserStore();
      const rt = userStore.refreshToken;
      if (!rt) throw new Error("no refresh_token");
      const r = await axios.post(
        "/admin/v1/auth/refresh-token",
        { refresh_token: rt },
        {
          baseURL: (import.meta.env.VITE_API_URL as string) || undefined, // 空=同源相对
          timeout: REFRESH_TIMEOUT_MS
        }
      );
      const env = r.data;
      if (env.code !== StatusCode.CodeSuccess) throw new Error(env.message_key ?? "refresh failed");
      userStore.setToken(env.data.access_token);
      userStore.setRefreshToken(env.data.refresh_token);
      return env.data.access_token;
    })();
    refreshInFlight.finally(() => (refreshInFlight = null));
  }
  return refreshInFlight;
}

/** 可否尝试 refresh（有 refresh_token、非 refresh 端点、非失败重放）。 */
function refreshable(config?: InternalAxiosRequestConfig): boolean {
  if (!config || (config as CustomAxiosRequestConfig)._isRetry) return false;
  if (config.url?.includes("/auth/refresh-token")) return false;
  return !!useUserStore().refreshToken;
}

class RequestHttp {
  service: AxiosInstance;
  public constructor(config: AxiosRequestConfig) {
    // instantiation
    this.service = axios.create(config);

    /**
     * @description 请求拦截器
     * 客户端发送请求 -> [请求拦截器] -> 服务器
     */
    this.service.interceptors.request.use(
      (config: CustomAxiosRequestConfig) => {
        const userStore = useUserStore();
        // 重复请求不需要取消，在 api 服务中通过指定的第三个参数：{ cancel: false } 来控制
        config.cancel ??= true;
        if (config.cancel) axiosCanceler.addPending(config);
        // 当前请求不需要显示 loading，在 api 服务中通过指定的第三个参数：{ loading: false } 来控制
        config.loading ??= true;
        if (config.loading) showFullScreenLoading();
        if (config.headers && typeof config.headers.set === "function") {
          // V2 adminapi 鉴权头（identity AccessVerifier——Bearer JWT）
          config.headers.set("Authorization", `Bearer ${userStore.token}`);
        }
        return config;
      },
      (error: AxiosError) => {
        return Promise.reject(error);
      }
    );

    /**
     * @description 响应拦截器——成功分支（HTTP 200）
     * 服务器返回信息 -> [拦截统一处理] -> 客户端JS获取到信息
     * V2 契约：HTTP 200 ⇔ code=10000（errorx HTTPStatusOf 强制配对）——
     * 401/refresh 全在 error 分支接管（此前 refresh 挂在本分支，
     * 后端 token 错误走真 HTTP 401 永不触发=死代码）
     */
    this.service.interceptors.response.use(
      (response: AxiosResponse & { config: CustomAxiosRequestConfig }) => {
        const { data, config } = response;
        axiosCanceler.removePending(config);
        if (config.loading) tryHideFullScreenLoading();
        // 全局错误信息拦截（防御面：正常契约下 HTTP 200 必带 code=10000；
        // 下载文件返回数据流无 code 直接放行）
        if (data.code && data.code !== ResultEnum.SUCCESS) {
          ElMessage.error(messageKeyText(data.message_key));
          return Promise.reject(data);
        }
        // 成功请求（在页面上除非特殊情况，否则不用处理失败逻辑）
        return data;
      },
      /**
       * @description 响应拦截器——错误分支（HTTP 4xx/5xx 与网络层）
       * V2 业务错误全部以真实 HTTP 状态码返回（errorx HTTPStatusOf），body 携带
       * 统一信封 {code,message_key,message,details}（根修：此前本
       * 分支只弹 checkStatus 通用文案，message_key/details 全部丢失——409 占用
       * 守卫的「请先解绑」引导永不出现）
       */
      async (error: AxiosError) => {
        const { response, config } = error;
        tryHideFullScreenLoading();

        // —— 网络层（无 response：超时/断网/DNS 失败） ——
        if (!response) {
          if (error.message.indexOf("timeout") !== -1) ElMessage.error("请求超时！请您稍后重试");
          else if (error.message.indexOf("Network Error") !== -1) ElMessage.error("网络错误！请您稍后重试");
          if (!window.navigator.onLine) router.replace("/500");
          return Promise.reject(error);
        }

        const body: any = response.data; // V2 信封

        // —— 401 面：refresh 单飞+重放 / 单飞登出 ——
        if (response.status === 401) {
          // 10104 吊销/TokenVersion 失效：refresh 无意义（服务端已全作废）——直登出
          if (body?.code !== StatusCode.CodeTokenRevoked && config && refreshable(config)) {
            try {
              const newToken = await refreshAccessToken();
              (config as CustomAxiosRequestConfig)._isRetry = true;
              config.headers.set("Authorization", `Bearer ${newToken}`);
              return this.service.request(config);
            } catch {
              // refresh 失败（refresh_token 也过期/被吊销）——落入单飞登出
            }
          }
          forceLogout(messageKeyText(body?.message_key));
          return Promise.reject(body ?? error);
        }

        // —— 非 401 业务错误：信封优先（reject 信封本体——业务 catch 可读
        //    code/message_key/details 结构化原因），无信封回退 HTTP 状态码文案 ——
        if (body?.code) {
          ElMessage.error(messageKeyText(body.message_key));
          return Promise.reject(body);
        }
        checkStatus(response.status);
        return Promise.reject(error);
      }
    );
  }

  /**
   * @description 常用请求方法封装
   */
  get<T>(url: string, params?: object, _object = {}): Promise<ResultData<T>> {
    return this.service.get(url, { params, ..._object });
  }
  post<T>(url: string, params?: object | string, _object = {}): Promise<ResultData<T>> {
    return this.service.post(url, params, _object);
  }
  put<T>(url: string, params?: object, _object = {}): Promise<ResultData<T>> {
    return this.service.put(url, params, _object);
  }
  delete<T>(url: string, params?: any, _object = {}): Promise<ResultData<T>> {
    return this.service.delete(url, { params, ..._object });
  }
  /** V2 PATCH（identity 启停等——transport Register 支持 PATCH） */
  patch<T>(url: string, params?: any, _object = {}): Promise<ResultData<T>> {
    return this.service.patch(url, params, _object);
  }
  /** V2 分页（Page{list,total,page,page_size}——api-style §3 契约；T=行类型，
   * 返回信封 data 为 Page<T>—— 组根修：此前写死 ResultData<any> 泛型被吞） */
  page<T>(url: string, page: number, pageSize: number, query?: Record<string, string | undefined>): Promise<ResultData<Page<T>>> {
    const qs = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
    // undefined/null/空串不拼入 query（避免 "undefined" 字面量被后端当过滤值——
    // 联调实证：username=undefined 过滤恒空集）
    for (const [k, v] of Object.entries(query ?? {})) {
      if (v !== undefined && v !== null && v !== "") qs.set(k, String(v));
    }
    return this.service.get(`${url}?${qs.toString()}`);
  }
  download(url: string, params?: object, _object = {}): Promise<BlobPart> {
    return this.service.post(url, params, { ..._object, responseType: "blob" });
  }
}

export default new RequestHttp(config);
