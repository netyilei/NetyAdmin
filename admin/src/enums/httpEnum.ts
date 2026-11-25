/**
 * @description：请求配置
 */
import { StatusCode } from "@/constants/status-codes";

// V2 信封契约（api-style §4）：code=5 位字符串、成功 10000、message_key=i18n 键。
// const 对象替代 enum（TS 数字/字符串混合 enum 限制——消费面均为只读比较）。
export const ResultEnum = {
  SUCCESS: StatusCode.CodeSuccess,
  OVERDUE: StatusCode.CodeUnauthorized, // 401 面（10101-10104 同走登出）
  TIMEOUT: 30000,
  TYPE: "success"
} as const;
