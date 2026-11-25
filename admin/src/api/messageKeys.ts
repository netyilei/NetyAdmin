// V2 message_key → 文案映射（api-style §4 message_key=i18n 键唯一源；
// 收敛点：axios 拦截器与业务 catch 共用本表—— 后并入 i18n languages）。
// 键集=kernel/errorx codes_gen.go codeMessageKey 的 admin 面子集（
// 漂移修正：common.validation→validation_failed、identity.*→auth.* 系；gateway.*
// 与 ws.* 系不进 admin 信封不收录；与 codes_gen 同步义务——改键双侧同批）。
export const MESSAGE_KEYS: Record<string, string> = {
  // —— common（10001-10012）——
  "common.invalid_params": "请求参数错误",
  "common.validation_failed": "请求参数校验失败",
  "common.not_found": "资源不存在",
  "common.conflict": "数据冲突",
  "common.unauthorized": "登录状态已失效，请重新登录",
  "common.forbidden": "无权限执行该操作",
  "common.too_many_requests": "请求过于频繁，请稍后重试",
  "common.timeout": "请求超时，请稍后重试",
  "common.internal": "系统内部错误，请稍后重试",
  "common.unavailable": "服务暂不可用",
  "common.idempotency_conflict": "请求正在处理中，请勿重复提交",
  "common.unsupported": "不支持的请求",
  // —— auth（10101-10106）——
  "auth.token_missing": "登录状态已失效，请重新登录",
  "auth.token_invalid": "登录状态已失效，请重新登录",
  "auth.token_expired": "登录已过期，请重新登录",
  "auth.token_revoked": "登录状态已失效（凭证已作废），请重新登录",
  "auth.account_disabled": "账号已禁用",
  "auth.permission_denied": "无权限执行该操作",
  // —— data（10201-10204）——
  "data.db_error": "数据访问异常，请稍后重试",
  "data.cache_error": "缓存访问异常，请稍后重试",
  "data.optimistic_lock": "数据已被他人修改，请刷新后重试",
  "data.unique_violation": "数据已存在（唯一性冲突）",
  // —— 业务面高频 ——
  "config.invalid": "配置项不合法",
  "storage.error": "存储操作失败",
  "message.send_failed": "消息发送失败"
};

export const messageKeyText = (key?: string): string => MESSAGE_KEYS[key ?? ""] ?? key ?? "操作失败";

/** 业务 catch 通用错误提示（e=信封形状——拦截器 reject 的 {code,message_key,details}）。 */
export function elError(e: any): void {
  // 动态 import ElMessage 以避免循环依赖（axios 拦截器同层引用）
  import("element-plus").then(({ ElMessage }) => ElMessage.error(messageKeyText(e?.message_key ?? e?.message)));
}
