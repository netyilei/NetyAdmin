import { h } from 'vue';
import { NTag } from 'naive-ui';
import { boolToDictValue, dictValueToBool, isDisabledStatus, isEnabledStatus } from '@/constants/business';
import { useDictStore } from '@/store/modules/dict';
import { $t } from '@/locales';

/**
 * 字典 label 的 i18n 启发式翻译：label 含 '.' 视为 i18n key 走 $t，否则原样。
 * 全项目唯一实现（原先 hook 内部与 app-dict-select / app-dict-radio-group 各自内联）。
 */
export function translateDictLabel(label: string): string {
  return label.includes('.') ? $t(label) : label;
}

export function useDict() {
  const dictStore = useDictStore();

  async function loadDicts(codes: string[]) {
    await dictStore.loadDicts(codes);
  }

  function getDictLabel(dictCode: string, value: string | number) {
    const data = dictStore.dictMap.get(dictCode);
    const item = data?.find(i => String(i.value) === String(value));
    return translateDictLabel(item?.label || String(value));
  }

  function renderDictTag(dictCode: string, value: string | number) {
    const data = dictStore.dictMap.get(dictCode);
    const item = data?.find(i => String(i.value) === String(value));
    if (!item) return h('span', {}, value);
    const label = translateDictLabel(item.label);
    return h(NTag, { type: item.tagType as any }, { default: () => label });
  }

  function getDictOptions(dictCode: string) {
    return dictStore.dictMap.get(dictCode)?.map(i => ({ label: i.label, value: i.value })) || [];
  }

  function renderBoolDictTag(dictCode: string, boolVal: boolean | null | undefined) {
    return renderDictTag(dictCode, boolToDictValue(boolVal));
  }

  function getDictBoolLabel(dictCode: string, boolVal: boolean | null | undefined) {
    return getDictLabel(dictCode, boolToDictValue(boolVal));
  }

  return {
    loadDicts,
    getDictLabel,
    renderDictTag,
    getDictOptions,
    renderBoolDictTag,
    getDictBoolLabel,
    isEnabledStatus,
    isDisabledStatus,
    boolToDictValue,
    dictValueToBool
  };
}
