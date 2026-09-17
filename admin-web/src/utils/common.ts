import { $t } from '@/locales';

/**
 * Transform record to option
 *
 * @example
 *   ```ts
 *   const record = {
 *     key1: 'label1',
 *     key2: 'label2'
 *   };
 *   const options = transformRecordToOption(record);
 *   // [
 *   //   { value: 'key1', label: 'label1' },
 *   //   { value: 'key2', label: 'label2' }
 *   // ]
 *   ```;
 *
 * @param record
 */
export function transformRecordToOption<T extends Record<string, string>>(record: T) {
  return Object.entries(record).map(([value, label]) => ({
    value,
    label
  })) as CommonType.Option<keyof T, T[keyof T]>[];
}

/**
 * Translate options
 *
 * @param options
 */
export function translateOptions<T extends CommonType.Option<string, App.I18n.I18nKey>>(options: T[]) {
  return options.map(option => ({
    ...option,
    label: $t(option.label)
  }));
}

/**
 * Toggle html class
 *
 * @param className
 */
export function toggleHtmlClass(className: string) {
  function add() {
    document.documentElement.classList.add(className);
  }

  function remove() {
    document.documentElement.classList.remove(className);
  }

  return {
    add,
    remove
  };
}

// 判断是否超管角色（支持单个 code 或角色数组——数组语义：包含任一超管 code）
// 全项目超管判定的唯一实现（原先 hooks/business/auth 与 store/modules/auth 各自内联比较）
export function isSuperByCode(code: string | string[]): boolean {
  const superRole = import.meta.env.VITE_STATIC_SUPER_ROLE;
  return Array.isArray(code) ? code.includes(superRole) : code === superRole;
}
