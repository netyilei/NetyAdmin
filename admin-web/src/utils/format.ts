import dayjs from 'dayjs';

/**
 * 统一的时间/数值格式化工具（收敛各视图散落的内联实现）。
 *
 * 下游新增展示逻辑时优先引用本文件；仅当确实出现新形态时再扩展，
 * 避免同功能格式化函数在多个视图中碎片化重写。
 */

/** 日期时间 → "YYYY-MM-DD HH:mm:ss"；空值返回 fallback（默认 "-"） */
export function formatDateTime(value: dayjs.ConfigType | null | undefined, fallback = '-'): string {
  if (value === null || value === undefined || value === '') return fallback;
  const d = dayjs(value);
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : fallback;
}

/** 日期 → "YYYY-MM-DD"；空值返回 fallback */
export function formatDate(value: dayjs.ConfigType | null | undefined, fallback = '-'): string {
  if (value === null || value === undefined || value === '') return fallback;
  const d = dayjs(value);
  return d.isValid() ? d.format('YYYY-MM-DD') : fallback;
}

/** 纳秒延迟 → 毫秒字符串（保留 2 位小数）；空/非法返回 fallback */
export function formatNsToMs(ns: number | null | undefined, fallback = '-'): string {
  if (ns === null || ns === undefined || Number.isNaN(ns)) return fallback;
  return `${(ns / 1e6).toFixed(2)}ms`;
}

/** 毫秒延迟 → 字符串（保留 2 位小数）；空/非法返回 fallback */
export function formatMs(ms: number | null | undefined, fallback = '-'): string {
  if (ms === null || ms === undefined || Number.isNaN(ms)) return fallback;
  return `${ms.toFixed(2)}ms`;
}

/** 字节数 → 人类可读（B/KB/MB/GB，1 位小数）；空/非法返回 fallback */
export function formatBytes(bytes: number | null | undefined, fallback = '-'): string {
  if (bytes === null || bytes === undefined || Number.isNaN(bytes) || bytes < 0) return fallback;
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let val = bytes;
  let i = 0;
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024;
    i += 1;
  }
  return `${val.toFixed(1)}${units[i]}`;
}
