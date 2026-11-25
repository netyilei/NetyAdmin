// 邮件设置 API（cmd/netyadmin/smtpconfig.go：configcenter 存储 + 热更生效；
// 密码 env-only 不回显。信封由 axios 拦截器解包，调用方取 data）。
import http from "@/api";

export interface SmtpConfig {
  host: string;
  port: number;
  username: string;
  from: string;
  password_set: boolean;
  source: string; // config（启动值）| configcenter（有覆盖）
  active_driver: string;
}

export interface SmtpInput {
  host: string;
  port: number;
  username: string;
  from: string;
}

export const smtpApi = {
  get: () => http.get<SmtpConfig>("/admin/v1/system/smtp"),
  save: (payload: SmtpInput) => http.put<SmtpConfig>("/admin/v1/system/smtp", payload),
  reset: () => http.delete<SmtpConfig>("/admin/v1/system/smtp"),
  test: (to: string) => http.post<{ sent: boolean; to: string }>("/admin/v1/system/smtp/test", { to })
};
